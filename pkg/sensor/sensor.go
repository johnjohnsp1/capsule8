// Copyright 2017 Capsule8 Inc. All rights reserved.

package sensor

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/container"
	"github.com/capsule8/capsule8/pkg/stream"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"
)

// Number of random bytes to generate for Sensor Id
const sensorIdLengthBytes = 32

// Sensor represents the state of the singleton Sensor instance
type Sensor struct {
	// Unique Id for this sensor. Sensor Ids are ephemeral.
	Id string

	// Sensor-unique event sequence number. Each event sent from this
	// sensor to any subscription has a unique sequence number for the
	// indicated sensor Id.
	sequenceNumber uint64

	// Record the value of CLOCK_MONOTONIC_RAW when the sensor starts.
	// All event monotimes are relative to this value.
	bootMonotimeNanos int64

	// Metrics counters for this sensor
	Metrics MetricsCounters

	// Repeater used for container event subscriptions
	containerEventRepeater *ContainerEventRepeater

	// If temporary fs mounts are made at startup, they're stored here.
	perfEventMountPoint string
	traceFSMountPoint   string

	// A sensor-global event monitor that is used for events to aid in
	// caching process information
	monitor *perf.EventMonitor

	// Per-sensor process cache.
	processCache ProcessInfoCache

	// Mapping of event ids to data streams (subscriptions)
	eventMap *safeSubscriptionMap
}

func NewSensor() (*Sensor, error) {
	randomBytes := make([]byte, sensorIdLengthBytes)
	rand.Read(randomBytes)
	sensorId := hex.EncodeToString(randomBytes[:])

	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)
	bootMonotimeNanos := ts.Nsec + (ts.Sec * int64(time.Second))

	s := &Sensor{
		Id:                sensorId,
		bootMonotimeNanos: bootMonotimeNanos,
		eventMap:          newSafeSubscriptionMap(),
	}

	cer, err := NewContainerEventRepeater(s)
	if err != nil {
		return nil, err
	}
	s.containerEventRepeater = cer

	return s, nil
}

func (s *Sensor) Start() error {
	var err error

	// We require that our run dir (usually /var/run/capsule8) exists.
	// Ensure that now before proceeding any further.
	err = os.MkdirAll(config.Global.RunDir, 0700)
	if err != nil {
		glog.Warningf("Couldn't mkdir %s: %s",
			config.Global.RunDir, err)
		return err
	}

	// If there is no mounted tracefs, the Sensor really can't do anything.
	// Try mounting our own private mount of it.
	if !config.Sensor.DontMountTracing && len(sys.TracingDir()) == 0 {
		// If we couldn't find one, try mounting our own private one
		glog.V(2).Info("Can't find mounted tracefs, mounting one")
		err = s.mountTraceFS()
		if err != nil {
			glog.V(1).Info(err)
			return err
		}
	}

	// If there is no mounted cgroupfs for the perf_event cgroup, we can't
	// efficiently separate processes in monitored containers from host
	// processes. We can run without it, but it's better performance when
	// available.
	if !config.Sensor.DontMountPerfEvent && len(sys.PerfEventDir()) == 0 {
		glog.V(2).Info("Can't find mounted perf_event cgroupfs, mounting one")
		err = s.mountPerfEventCgroupFS()
		if err != nil {
			glog.V(1).Info(err)
			// This is not a fatal error condition, proceed on
		}
	}

	// Create the sensor-global event monitor. This EventMonitor instance
	// will be used for all perf_event events
	err = s.createEventMonitor()
	if err != nil {
		s.Stop()
		return err
	}

	s.processCache = NewProcessInfoCache(s)

	// Make sure that all events registered with the sensor's event monitor
	// are active
	s.monitor.EnableAll()

	return nil
}

func (s *Sensor) Stop() {
	if s.monitor != nil {
		glog.V(2).Info("Stopping sensor-global EventMonitor")
		s.monitor.Close(true)
		s.monitor = nil
		glog.V(2).Info("Sensor-global EventMonitor stopped successfully")
	}

	if len(s.traceFSMountPoint) > 0 {
		s.unmountTraceFS()
	}

	if len(s.perfEventMountPoint) > 0 {
		s.unmountPerfEventCgroupFS()
	}
}

func (s *Sensor) dispatchSample(eventID uint64, sample interface{}, err error) {
	if err != nil {
		glog.Warning(err)
	}

	if event, ok := sample.(*api.Event); ok && event != nil {
		eventMap := s.eventMap.getMap()
		data := eventMap[eventID]
		if data != nil {
			data <- event
		}
	}
}

func (s *Sensor) mountTraceFS() error {
	dir := filepath.Join(config.Global.RunDir, "tracing")
	err := sys.MountTempFS("tracefs", dir, "tracefs", 0, "")
	if err == nil {
		s.traceFSMountPoint = dir
	}
	return err
}

func (s *Sensor) unmountTraceFS() {
	err := sys.UnmountTempFS(s.traceFSMountPoint, "tracefs")
	if err == nil {
		s.traceFSMountPoint = ""
	} else {
		glog.V(2).Infof("Could not unmount %s: %s",
			s.traceFSMountPoint, err)
	}
}

func (s *Sensor) mountPerfEventCgroupFS() error {
	dir := filepath.Join(config.Global.RunDir, "perf_event")
	err := sys.MountTempFS("cgroup", dir, "cgroup", 0, "perf_event")
	if err == nil {
		s.perfEventMountPoint = dir
	}
	return err
}

func (s *Sensor) unmountPerfEventCgroupFS() {
	err := sys.UnmountTempFS(s.perfEventMountPoint, "cgroup")
	if err == nil {
		s.perfEventMountPoint = ""
	} else {
		glog.V(2).Infof("Could not unmount %s: %s",
			s.perfEventMountPoint, err)
	}
}

func (s *Sensor) currentMonotimeNanos() int64 {
	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)
	d := ts.Nsec + (ts.Sec * int64(time.Second))
	return d - s.bootMonotimeNanos
}

func (s *Sensor) nextSequenceNumber() uint64 {
	// The first sequence number is intentionally 1 to disambiguate
	// from no sequence number being included in the protobuf message.
	s.sequenceNumber++
	return s.sequenceNumber
}

func (s *Sensor) NewEvent() *api.Event {
	monotime := s.currentMonotimeNanos()
	sequenceNumber := s.nextSequenceNumber()

	var b []byte
	buf := bytes.NewBuffer(b)

	binary.Write(buf, binary.LittleEndian, s.Id)
	binary.Write(buf, binary.LittleEndian, sequenceNumber)
	binary.Write(buf, binary.LittleEndian, monotime)

	h := sha256.Sum256(buf.Bytes())
	eventId := hex.EncodeToString(h[:])

	s.Metrics.Events++

	return &api.Event{
		Id:                   eventId,
		SensorId:             s.Id,
		SensorMonotimeNanos:  monotime,
		SensorSequenceNumber: sequenceNumber,
	}
}

func (s *Sensor) NewEventFromContainer(containerId string) *api.Event {
	e := s.NewEvent()
	e.ContainerId = containerId
	return e
}

func (s *Sensor) NewEventFromSample(sample *perf.SampleRecord,
	data perf.TraceEventSampleData) *api.Event {

	e := s.NewEvent()
	e.SensorMonotimeNanos = int64(sample.Time) - s.bootMonotimeNanos

	// When both the sensor and the process generating the sample are in
	// containers, the sample.Pid and sample.Tid fields will be zero.
	// Use "common_pid" from the trace event data instead.
	e.ProcessPid = data["common_pid"].(int32)
	e.Cpu = int32(sample.CPU)

	processId, ok := s.processCache.ProcessId(int(e.ProcessPid))
	if ok {
		e.ProcessId = processId
	}

	// Add an associated containerId
	containerId, ok := s.processCache.ProcessContainerId(int(e.ProcessPid))
	if ok {
		// Add the container Id if we have it and then try
		// using it to look up additional container info
		e.ContainerId = containerId

		containerInfo := container.GetInfo(containerId)
		if containerInfo != nil {
			e.ContainerName = containerInfo.Name
			e.ImageId = containerInfo.ImageID
			e.ImageName = containerInfo.ImageName
		}
	}

	return e
}

func (s *Sensor) buildMonitorGroups() ([]string, []int, error) {
	var (
		cgroupList []string
		pidList    []int
		system     bool
	)

	cgroups := make(map[string]bool)
	for _, cgroup := range config.Sensor.CgroupName {
		if len(cgroup) == 0 || cgroup == "/" {
			system = true
			continue
		}
		if cgroups[cgroup] {
			continue
		}
		cgroups[cgroup] = true
		cgroupList = append(cgroupList, cgroup)
	}

	// Try a system-wide perf event monitor if requested or as
	// a fallback if no cgroups were requested
	if system || len(sys.PerfEventDir()) == 0 || len(cgroupList) == 0 {
		glog.V(1).Info("Creating new system-wide event monitor")
		pidList = append(pidList, -1)
	}

	return cgroupList, pidList, nil
}

func (s *Sensor) createEventMonitor() error {
	eventMonitorOptions := []perf.EventMonitorOption{}

	cgroups, pids, err := s.buildMonitorGroups()
	if err != nil {
		return err
	}

	if len(cgroups) == 0 && len(pids) == 0 {
		glog.Fatal("Can't create event monitor with no cgroups or pids")
	}

	if len(cgroups) > 0 {
		var perfEventDir string
		if len(s.perfEventMountPoint) > 0 {
			perfEventDir = s.perfEventMountPoint
		} else {
			perfEventDir = sys.PerfEventDir()
		}
		if len(perfEventDir) > 0 {
			glog.V(1).Infof("Creating new perf event monitor on cgroups %s",
				strings.Join(cgroups, ","))

			eventMonitorOptions = append(eventMonitorOptions,
				perf.WithPerfEventDir(perfEventDir),
				perf.WithCgroups(cgroups))
		}
	}

	if len(pids) > 0 {
		glog.V(1).Info("Creating new system-wide event monitor")
		eventMonitorOptions = append(eventMonitorOptions,
			perf.WithPids(pids))
	}

	s.monitor, err = perf.NewEventMonitor(eventMonitorOptions...)
	if err != nil {
		// If a cgroup-specific event monitor could not be created,
		// fall back to a system-wide event monitor.
		if len(cgroups) > 0 &&
			(len(pids) == 0 || (len(pids) == 1 && pids[0] == -1)) {

			glog.Warningf("Couldn't create perf event monitor on cgroups %s: %s",
				strings.Join(cgroups, ","), err)

			glog.V(1).Info("Creating new system-wide event monitor")
			s.monitor, err = perf.NewEventMonitor()
		}
		if err != nil {
			glog.V(1).Infof("Couldn't create event monitor: %s", err)
			return err
		}
	}

	go func() {
		err := s.monitor.Run(s.dispatchSample)
		if err != nil {
			glog.Fatal(err)
		}
		glog.V(2).Info("EventMonitor.Run() returned; exiting goroutine")
	}()

	return nil
}

func (s *Sensor) createPerfEventStream(sub *api.Subscription) (*stream.Stream, error) {
	eventMap := make(map[uint64]chan interface{})

	ctrl := make(chan interface{})
	data := make(chan interface{}, config.Sensor.ChannelBufferLength)

	if len(sub.EventFilter.FileEvents) > 0 {
		ids := registerFileEvents(s.monitor, s, sub.EventFilter.FileEvents)
		for _, id := range ids {
			eventMap[id] = data
		}
	}
	if len(sub.EventFilter.KernelEvents) > 0 {
		ids := registerKernelEvents(s.monitor, s, sub.EventFilter.KernelEvents)
		for _, id := range ids {
			eventMap[id] = data
		}
	}
	if len(sub.EventFilter.NetworkEvents) > 0 {
		ids := registerNetworkEvents(s.monitor, s, sub.EventFilter.NetworkEvents)
		for _, id := range ids {
			eventMap[id] = data
		}
	}
	if len(sub.EventFilter.ProcessEvents) > 0 {
		ids := registerProcessEvents(s.monitor, s, sub.EventFilter.ProcessEvents)
		for _, id := range ids {
			eventMap[id] = data
		}
	}
	if len(sub.EventFilter.SyscallEvents) > 0 {
		ids := registerSyscallEvents(s.monitor, s, sub.EventFilter.SyscallEvents)
		for _, id := range ids {
			eventMap[id] = data
		}
	}

	if len(eventMap) == 0 {
		close(ctrl)
		close(data)
		return nil, nil
	}

	go func() {
		defer close(data)

		for {
			_, ok := <-ctrl
			if !ok {
				glog.V(2).Info("Control channel closed")

				// Remove from .eventMap first so that any
				// pending events being processed get discarded
				// as quickly as possible. Then remove the
				// events from the EventMonitor
				s.eventMap.remove(eventMap)
				for eventID := range eventMap {
					s.monitor.UnregisterEvent(eventID)
				}
				return
			}
		}
	}()

	s.eventMap.update(eventMap)
	for eventID := range eventMap {
		s.monitor.Enable(eventID)
	}

	return &stream.Stream{
		Ctrl: ctrl,
		Data: data,
	}, nil
}

func (s *Sensor) applyModifiers(eventStream *stream.Stream, modifier api.Modifier) *stream.Stream {
	if modifier.Throttle != nil {
		eventStream = stream.Throttle(eventStream, *modifier.Throttle)
	}

	if modifier.Limit != nil {
		eventStream = stream.Limit(eventStream, *modifier.Limit)
	}

	return eventStream
}

// NewSubscription creates a new telemetry subscription from the given
// api.Subscription descriptor. NewSubscription returns a stream.Stream of
// api.Events matching the specified filters. Closing the Stream cancels the
// subscription.
func (s *Sensor) NewSubscription(sub *api.Subscription) (*stream.Stream, error) {
	glog.V(1).Infof("Subscribing to %+v", sub)

	eventStream, joiner := stream.NewJoiner()
	joiner.Off()

	if len(sub.EventFilter.FileEvents) > 0 ||
		len(sub.EventFilter.KernelEvents) > 0 ||
		len(sub.EventFilter.NetworkEvents) > 0 ||
		len(sub.EventFilter.ProcessEvents) > 0 ||
		len(sub.EventFilter.SyscallEvents) > 0 {

		pes, err := s.createPerfEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, err
		}
		if pes != nil {
			joiner.Add(pes)
		}
	}

	if len(sub.EventFilter.ContainerEvents) > 0 {
		ces, err := s.containerEventRepeater.NewEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, err
		}
		joiner.Add(ces)
	}

	for _, cf := range sub.EventFilter.ChargenEvents {
		cs, err := newChargenSource(s, cf)
		if err != nil {
			joiner.Close()
			return nil, err
		}
		joiner.Add(cs)
	}

	for _, tf := range sub.EventFilter.TickerEvents {
		ts, err := newTickerSource(s, tf)
		if err != nil {
			joiner.Close()
			return nil, err
		}
		joiner.Add(ts)
	}

	if sub.ContainerFilter != nil {
		// Filter stream as requested by subscriber in the
		// specified ContainerFilter to restrict the events to
		// those matching the specified container ids, names,
		// images, etc.
		cef := newContainerFilter(sub.ContainerFilter)
		eventStream = stream.Filter(eventStream, cef.FilterFunc)
		eventStream = stream.Do(eventStream, cef.DoFunc)
	}

	if sub.Modifier != nil {
		eventStream = s.applyModifiers(eventStream, *sub.Modifier)
	}

	s.Metrics.Subscriptions++
	joiner.On()

	return eventStream, nil
}

func filterNils(e interface{}) bool {
	if e != nil {
		ev := e.(*api.Event)
		return ev != nil
	}
	return false
}

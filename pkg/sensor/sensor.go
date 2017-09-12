package sensor

import (
	"sync"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/filter"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/golang/glog"
)

func (s *Sensor) onSampleEvent(sample interface{}, err error) {
	if sample != nil {
		event := sample.(*api.Event)

		for _, c := range s.eventStreams {
			c <- event
		}
	}
}

//
// FilterSet represents the union of all requested events for a
// subscription. It consists of sub-event sets for each supported type of
// event, which similarly represent the union of all requested events of
// a given type.
//
// The perf event backend translates a FilterSet into a set of tracepoint,
// kprobe, and uprobe events with ftrace filters. The eBPF backend translates
// a FilterSet into an eBPF program on tracepoints, kprobes, and uprobes.
//
type filterSet struct {
	syscalls  *syscallFilterSet
	processes *processFilterSet
	files     *fileFilterSet
}

func (fs *filterSet) addSyscallEventFilter(sef *api.SyscallEventFilter) {
	if fs.syscalls == nil {
		fs.syscalls = new(syscallFilterSet)
	}

	fs.syscalls.add(sef)
}

func (fs *filterSet) addProcessEventFilter(pef *api.ProcessEventFilter) {
	if fs.processes == nil {
		fs.processes = new(processFilterSet)
	}

	fs.processes.add(pef)
}

func (fs *filterSet) addFileEventFilter(fef *api.FileEventFilter) {
	if fs.files == nil {
		fs.files = new(fileFilterSet)
	}

	fs.files.add(fef)
}

func (fs *filterSet) len() int {
	length := 0

	if fs.syscalls != nil {
		length += fs.syscalls.len()
	}

	if fs.processes != nil {
		length += fs.processes.len()
	}

	if fs.files != nil {
		length += fs.files.len()
	}

	return length
}

func (fs *filterSet) registerEvents(monitor *perf.EventMonitor) {
	if fs.syscalls != nil {
		fs.syscalls.registerEvents(monitor)
	}
	if fs.processes != nil {
		fs.processes.registerEvents(monitor)
	}
	if fs.files != nil {
		fs.files.registerEvents(monitor)
	}
}

// ---------------------------------------------------------------------

type Sensor struct {
	mu                     sync.Mutex
	containerEventRepeater *stream.Repeater
	monitor                *perf.EventMonitor
	eventStreams           map[*api.Subscription]chan interface{}
}

//
// Sensor is a singleton
//

var (
	sensor     *Sensor
	sensorOnce sync.Once
)

func getSensor() *Sensor {
	sensorOnce.Do(func() {
		sensor = &Sensor{}
	})

	return sensor
}

func (s *Sensor) update() error {
	fs := &filterSet{}

	for s := range s.eventStreams {
		if s.EventFilter != nil {
			ef := s.EventFilter
			for _, sef := range ef.SyscallEvents {
				fs.addSyscallEventFilter(sef)
			}

			for _, pef := range ef.ProcessEvents {
				fs.addProcessEventFilter(pef)
			}

			for _, fef := range ef.FileEvents {
				fs.addFileEventFilter(fef)
			}
		}
	}

	//
	// Update perf
	//

	// Stop old perf session
	if s.monitor != nil {
		glog.Info("Disabling existing perf session")
		s.monitor.Close(false)
		s.monitor = nil
	}

	// Create a new perf session only if we have perf event config info
	if fs.len() > 0 {
		var (
			monitor *perf.EventMonitor
			err     error
		)

		//
		// If a cgroup name is configured (can be "/"), then monitor
		// that cgroup within the perf_event hierarchy. Otherwise,
		// monitor all processes on the system.
		//
		if len(config.Sensor.CgroupName) > 0 {
			glog.Infof("Creating new event monitor on cgroup %s",
				config.Sensor.CgroupName)
			monitor, err = perf.NewEventMonitorWithCgroup(config.Sensor.CgroupName, 0, 0, nil)
		} else {
			glog.Info("Createing new system-wide event monitor")
			monitor, err = perf.NewEventMonitor(-1, 0, 0, nil)
		}
		if err != nil {
			return err
		}

		fs.registerEvents(monitor)
		s.monitor = monitor

		go func() {
			s.monitor.Enable()

			s.monitor.Run(s.onSampleEvent)
			glog.Infof("perf.Run() returned, exiting goroutine")
		}()
	}

	return nil
}

func (s *Sensor) createPerfEventStream(sub *api.Subscription) (*stream.Stream, error) {
	ctrl := make(chan interface{})
	data := make(chan interface{})

	go func() {
		defer close(data)

		for {
			select {
			case _, ok := <-ctrl:
				if !ok {
					return
				}
			}
		}
	}()

	//
	// We only need to save the data channel
	//
	if s.eventStreams == nil {
		s.eventStreams = make(map[*api.Subscription]chan interface{})
	}

	s.eventStreams[sub] = data

	err := s.update()
	if err != nil {
		return nil, err
	}

	return &stream.Stream{
		Ctrl: ctrl,
		Data: data,
	}, nil
}

func (s *Sensor) getContainerEventStream() (*stream.Stream, error) {
	if s.containerEventRepeater == nil {
		ces, err := container.NewEventStream()
		if err != nil {
			return nil, err
		}

		// Translate container events to protobuf versions
		ces = stream.Map(ces, translateContainerEvents)
		ces = stream.Filter(ces, filterNils)

		s.containerEventRepeater = stream.NewRepeater(ces)
	}

	return s.containerEventRepeater.NewStream(), nil
}

func applyModifiers(strm *stream.Stream, modifier api.Modifier) *stream.Stream {
	if modifier.Throttle != nil {
		strm = stream.Throttle(strm, *modifier.Throttle)
	}

	if modifier.Limit != nil {
		strm = stream.Limit(strm, *modifier.Limit)
	}

	return strm
}

func filterNils(e interface{}) bool {
	if e != nil {
		ev := e.(*api.Event)
		return ev != nil
	}

	return e != nil
}

// Add returns a stream
func (s *Sensor) Add(sub *api.Subscription) (*stream.Stream, error) {
	glog.Infof("Enter Add(%v)", sub)
	defer glog.Infof("Exit Add(%v)", sub)

	s.mu.Lock()
	defer s.mu.Unlock()

	eventStream, joiner := stream.NewJoiner()
	joiner.Off()

	if len(sub.EventFilter.SyscallEvents) > 0 ||
		len(sub.EventFilter.ProcessEvents) > 0 ||
		len(sub.EventFilter.FileEvents) > 0 {

		//
		// Create a perf event stream
		//
		pes, err := s.createPerfEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		glog.Info("Adding perf EventStream to joiner")
		joiner.Add(pes)
		glog.Info("Added perf EventStream to joiner")
	}

	if len(sub.EventFilter.ContainerEvents) > 0 {
		ces, err := s.getContainerEventStream()
		if err != nil {
			joiner.Close()
			return nil, err
		}

		glog.Info("Adding container EventStream to joiner")
		joiner.Add(ces)
		glog.Info("Added container EventStream to joiner")
	}

	for _, cf := range sub.EventFilter.ChargenEvents {
		cs, err := NewChargenSensor(cf)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		glog.Info("Adding chargen EventStream to joiner")
		joiner.Add(cs)
		glog.Info("Added chargen EventStream to joiner")
	}

	for _, tf := range sub.EventFilter.TickerEvents {
		ts, err := NewTickerSensor(tf)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		glog.Info("Adding ticker EventStream to joiner")
		joiner.Add(ts)
		glog.Info("Added ticker EventStream to joiner")
	}

	//
	// Filter event stream by event type first
	//
	ef := filter.NewEventFilter(sub.EventFilter)
	eventStream = stream.Filter(eventStream, ef.FilterFunc)

	if sub.ContainerFilter != nil {
		//
		// Attach a ContainerFilter to filter events
		//
		cef := filter.NewContainerFilter(sub.ContainerFilter)
		// Filter eventStream by container
		eventStream = stream.Filter(eventStream, cef.FilterFunc)
		eventStream = stream.Do(eventStream, cef.DoFunc)
	}

	if sub.Modifier != nil {
		eventStream = applyModifiers(eventStream, *sub.Modifier)
	}

	joiner.On()
	return eventStream, nil
}

func Remove(subscription *api.Subscription) bool {
	glog.Infof("Removing subscription %v", subscription)
	defer glog.Infof("Removed subscription %v", subscription)

	s := getSensor()
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.eventStreams[subscription]
	if ok {
		delete(s.eventStreams, subscription)
		s.update()
	}

	return ok
}

// Receives a Subscription
// If Node filter doesn't match self, bail
// Configure Process Monitor based on the Process Filter
// Configure Sensors based on the Selectors

func NewSensor(sub *api.Subscription) (*stream.Stream, error) {
	s := getSensor()

	eventStream, err := s.Add(sub)
	if err != nil {
		glog.Errorf("Couldn't add subscription %v: %v", sub, err)
		return nil, err
	}

	return eventStream, nil
}

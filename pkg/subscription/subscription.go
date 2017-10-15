package subscription

import (
	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/filter"
	"github.com/capsule8/capsule8/pkg/stream"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"
)

var (
	// SensorID is a unique ID of the running instance of the Sensor.
	SensorID string
)

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
	fileFilters    *fileFilterSet
	kernelFilters  *kprobeFilterSet
	networkFilters *networkFilterSet
	processFilters *processFilterSet
	syscallFilters *syscallFilterSet
}

func (fs *filterSet) addFileEventFilter(fef *api.FileEventFilter) {
	if fs.fileFilters == nil {
		fs.fileFilters = new(fileFilterSet)
	}

	fs.fileFilters.add(fef)
}

func (fs *filterSet) addKprobeEventFilter(kef *api.KernelFunctionCallFilter) {
	if fs.kernelFilters == nil {
		fs.kernelFilters = new(kprobeFilterSet)
	}

	fs.kernelFilters.add(kef)
}

func (fs *filterSet) addNetworkEventFilter(nef *api.NetworkEventFilter) {
	if fs.networkFilters == nil {
		fs.networkFilters = new(networkFilterSet)
	}

	fs.networkFilters.add(nef)
}

func (fs *filterSet) addProcessEventFilter(pef *api.ProcessEventFilter) {
	if fs.processFilters == nil {
		fs.processFilters = new(processFilterSet)
	}

	fs.processFilters.add(pef)
}

func (fs *filterSet) addSyscallEventFilter(sef *api.SyscallEventFilter) {
	if fs.syscallFilters == nil {
		fs.syscallFilters = new(syscallFilterSet)
	}

	fs.syscallFilters.add(sef)
}

func (fs *filterSet) len() int {
	length := 0

	if fs.fileFilters != nil {
		length += fs.fileFilters.len()
	}
	if fs.kernelFilters != nil {
		length += fs.kernelFilters.len()
	}
	if fs.networkFilters != nil {
		length += fs.networkFilters.len()
	}
	if fs.processFilters != nil {
		length += fs.processFilters.len()
	}
	if fs.syscallFilters != nil {
		length += fs.syscallFilters.len()
	}

	return length
}

func (fs *filterSet) registerEvents(monitor *perf.EventMonitor) {
	if fs.fileFilters != nil {
		fs.fileFilters.registerEvents(monitor)
	}
	if fs.kernelFilters != nil {
		fs.kernelFilters.registerEvents(monitor)
	}
	if fs.networkFilters != nil {
		fs.networkFilters.registerEvents(monitor)
	}
	if fs.processFilters != nil {
		fs.processFilters.registerEvents(monitor)
	}
	if fs.syscallFilters != nil {
		fs.syscallFilters.registerEvents(monitor)
	}
}

// ---------------------------------------------------------------------

func createMonitor(s *api.Subscription) (monitor *perf.EventMonitor, err error) {
	fs := &filterSet{}

	if s.EventFilter != nil {
		ef := s.EventFilter
		for _, fef := range ef.FileEvents {
			fs.addFileEventFilter(fef)
		}
		for _, kef := range ef.KernelEvents {
			fs.addKprobeEventFilter(kef)
		}
		for _, nef := range ef.NetworkEvents {
			fs.addNetworkEventFilter(nef)
		}
		for _, pef := range ef.ProcessEvents {
			fs.addProcessEventFilter(pef)
		}
		for _, sef := range ef.SyscallEvents {
			fs.addSyscallEventFilter(sef)
		}
	}

	if fs.len() > 0 {
		// If a perf_event cgroupfs is mounted and a cgroup
		// name is configured (can be "/"), then monitor that
		// cgroup within the perf_event hierarchy. Otherwise,
		// monitor all processes on the system.
		if len(config.Sensor.CgroupName) > 0 {
			glog.V(1).Infof("Creating new perf event monitor on "+
				"cgroup %s", config.Sensor.CgroupName)

			monitor, err = perf.NewEventMonitorWithCgroup(
				config.Sensor.CgroupName, 0, 0, nil)

			if err != nil {
				glog.Warningf("Couldn't create perf event "+
					"monitor on cgroup %s (%s), creating "+
					"new system-wide perf event monitor "+
					"instead.", config.Sensor.CgroupName,
					err)
			}
		} else if inContainer() {
			// Assume /docker if we are running within a container

			glog.V(1).Infof("Creating new perf event monitor on "+
				"cgroup %s", "/docker")

			monitor, err = perf.NewEventMonitorWithCgroup(
				"/docker", 0, 0, nil)

			if err != nil {
				glog.Warningf("Couldn't create perf event "+
					"monitor on cgroup %s (%s), creating "+
					"new system-wide perf event monitor "+
					"instead.", "/docker",
					err)
			}
		}

		// Try a system-wide perf event monitor as a fallback if either
		// of the above failed.
		if monitor == nil {
			glog.V(1).Info("Creating new system-wide event monitor")
			monitor, err = perf.NewEventMonitor(-1, 0, 0, nil)
		}

		if monitor == nil {
			return
		}

		fs.registerEvents(monitor)

	} else {
		glog.V(1).Infof("No filters, not creating a new EventMonitor")
	}

	return monitor, err
}

func inContainer() bool {
	procFS := sys.ProcFS()
	initCgroups, err := procFS.Cgroups(1)
	if err != nil {
		glog.Fatalf("Couldn't get cgroups for pid 1: %s", err)
	}

	for _, cg := range initCgroups {
		if cg.Path == "/" {
			// /proc is a host procfs, return it
			return false
		}
	}

	return true
}

func createPerfEventStream(sub *api.Subscription) (*stream.Stream, error) {
	//
	// Create the perf event monitor out of the subscription
	// first. If it fails, there is nothing else we can do.
	//
	monitor, err := createMonitor(sub)
	if err != nil {
		return nil, err
	}

	ctrl := make(chan interface{})
	data := make(chan interface{}, 128)

	go func() {
		defer close(data)

		for {
			_, ok := <-ctrl
			if !ok {
				glog.V(1).Infof("Control channel closed, closing monitor")

				// Wait until Close() fully terminates before
				// allowing data channel to close
				monitor.Close(true)
				return
			}
		}
	}()

	go func() {
		monitor.Run(func(sample interface{}, err error) {
			if event, ok := sample.(*api.Event); ok && event != nil {
				data <- event
			}
		})

		glog.V(1).Infof("EventMonitor.Run() returned, exiting goroutine")

	}()

	glog.V(1).Infof("Enabling EventMonitor")
	monitor.Enable()

	return &stream.Stream{
		Ctrl: ctrl,
		Data: data,
	}, nil
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

func createTelemetryStream(sub *api.Subscription) (*stream.Stream, error) {
	glog.V(1).Infof("Subscribing to %+v", sub)

	eventStream, joiner := stream.NewJoiner()
	joiner.Off()

	if len(sub.EventFilter.FileEvents) > 0 ||
		len(sub.EventFilter.KernelEvents) > 0 ||
		len(sub.EventFilter.NetworkEvents) > 0 ||
		len(sub.EventFilter.ProcessEvents) > 0 ||
		len(sub.EventFilter.SyscallEvents) > 0 {

		//
		// Create a perf event stream
		//
		pes, err := createPerfEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		joiner.Add(pes)
	}

	if len(sub.EventFilter.ContainerEvents) > 0 {
		ces, err := createContainerEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		joiner.Add(ces)
	}

	for _, cf := range sub.EventFilter.ChargenEvents {
		cs, err := NewChargenSensor(cf)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		joiner.Add(cs)
	}

	for _, tf := range sub.EventFilter.TickerEvents {
		ts, err := NewTickerSensor(tf)
		if err != nil {
			joiner.Close()
			return nil, err
		}

		joiner.Add(ts)
	}

	if sub.ContainerFilter != nil {
		//
		// Filter stream as requested by subscriber in the
		// specified ContainerFilter to restrict the events to
		// those matching the specified container ids, names,
		// images, etc.
		//
		cef := filter.NewContainerFilter(sub.ContainerFilter)
		eventStream = stream.Filter(eventStream, cef.FilterFunc)
		eventStream = stream.Do(eventStream, cef.DoFunc)
	}

	if sub.Modifier != nil {
		eventStream = applyModifiers(eventStream, *sub.Modifier)
	}

	joiner.On()

	return eventStream, nil
}

// NewSubscription creates a new telemetry subscription from the given
// api.Subscription descriptor. NewSubscription returns a stream.Stream of
// api.Events matching the specified filters. Closing the Stream cancels the
// subscription.
func NewSubscription(sub *api.Subscription, sensorID string) (*stream.Stream, error) {
	SensorID = sensorID

	return createTelemetryStream(sub)
}

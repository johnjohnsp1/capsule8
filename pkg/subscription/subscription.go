package subscription

import (
	"sync"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/filter"
	"github.com/capsule8/reactive8/pkg/process"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/capsule8/reactive8/pkg/sys"
	"github.com/capsule8/reactive8/pkg/sys/perf"
	"github.com/golang/glog"
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

//
// subscriptionBroker is a singleton
//

type subscriptionBroker struct {
	mu                     sync.Mutex
	containerEventRepeater *stream.Repeater
	monitor                *perf.EventMonitor
	perfEventStreams       map[*api.Subscription]chan interface{}
}

var (
	broker     *subscriptionBroker
	brokerOnce sync.Once
)

func getSubscriptionBroker() *subscriptionBroker {
	brokerOnce.Do(func() {
		broker = &subscriptionBroker{}
	})

	return broker
}

func (sb *subscriptionBroker) update() error {
	glog.V(1).Info("Updating perf events and filters")

	fs := &filterSet{}

	for s := range sb.perfEventStreams {
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
	}

	//
	// Update perf
	//

	// Stop old perf session
	if sb.monitor != nil {
		glog.V(1).Info("Stopping existing perf.EventMonitor")
		sb.monitor.Close(false)
		sb.monitor = nil
	}

	// Create a new perf session only if we have perf event config info
	if fs.len() > 0 {
		var (
			monitor *perf.EventMonitor
			err     error
		)

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

		if err != nil {
			return err
		}

		fs.registerEvents(monitor)

		go func() {
			s := make([]chan interface{}, len(sb.perfEventStreams))
			i := 0

			for _, v := range sb.perfEventStreams {
				s[i] = v
				i++
			}

			monitor.Run(func(sample interface{}, err error) {
				if event, ok := sample.(*api.Event); ok && event != nil {
					for _, c := range s {
						c <- event
					}
				}
			})

			glog.V(1).Infof("EventMonitor.Run() returned, exiting goroutine")
		}()

		glog.V(1).Infof("Enabling EventMonitor")
		monitor.Enable()

		sb.monitor = monitor
	} else {
		glog.V(1).Infof("No filters, not creating a new EventMonitor")
	}

	return nil
}

func inContainer() bool {
	procFS := sys.ProcFS()
	initCgroups := procFS.Cgroups(1)
	for _, cg := range initCgroups {
		if cg.Path == "/" {
			// /proc is a host procfs, return it
			return false
		}
	}

	return true
}

func (sb *subscriptionBroker) createPerfEventStream(sub *api.Subscription) (*stream.Stream, error) {
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
	if sb.perfEventStreams == nil {
		sb.perfEventStreams = make(map[*api.Subscription]chan interface{})
	}

	sb.perfEventStreams[sub] = data

	err := sb.update()
	if err != nil {
		return nil, err
	}

	return &stream.Stream{
		Ctrl: ctrl,
		Data: data,
	}, nil
}

func (sb *subscriptionBroker) getContainerEventStream() (*stream.Stream, error) {
	if sb.containerEventRepeater == nil {
		ces, err := container.NewEventStream()
		if err != nil {
			return nil, err
		}

		// Translate container events to protobuf versions
		ces = stream.Map(ces, translateContainerEvents)
		ces = stream.Filter(ces, filterNils)

		sb.containerEventRepeater = stream.NewRepeater(ces)
	}

	return sb.containerEventRepeater.NewStream(), nil
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

// Subscribe creates a new telemetry subscription
func (sb *subscriptionBroker) Subscribe(sub *api.Subscription) (*stream.Stream, *stream.Joiner, error) {
	glog.V(1).Infof("Subscribing to %+v", sub)

	sb.mu.Lock()
	defer sb.mu.Unlock()

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
		pes, err := sb.createPerfEventStream(sub)
		if err != nil {
			joiner.Close()
			return nil, nil, err
		}

		joiner.Add(pes)
	}

	if len(sub.EventFilter.ContainerEvents) > 0 {
		ces, err := sb.getContainerEventStream()
		if err != nil {
			joiner.Close()
			return nil, nil, err
		}

		joiner.Add(ces)
	}

	for _, cf := range sub.EventFilter.ChargenEvents {
		cs, err := NewChargenSensor(cf)
		if err != nil {
			joiner.Close()
			return nil, nil, err
		}

		joiner.Add(cs)
	}

	for _, tf := range sub.EventFilter.TickerEvents {
		ts, err := NewTickerSensor(tf)
		if err != nil {
			joiner.Close()
			return nil, nil, err
		}

		joiner.Add(ts)
	}

	//
	// Filter event stream by event type first
	//
	ef := filter.NewEventFilter(sub.EventFilter)
	eventStream = stream.Filter(eventStream, ef.FilterFunc)

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

	//
	// If adding process lineage to events, only do so after event
	// filtering.  (No point in adding lineage to an event that is
	// going to be filtered).
	//
	if sub.ProcessView == api.ProcessView_PROCESS_VIEW_FULL {
		eventStream = stream.Do(eventStream, addProcessLineage)
	}

	joiner.On()
	return eventStream, joiner, nil
}

func addProcessLineage(i interface{}) {
	e := i.(*api.Event)

	var lineage []*api.Process

	pis := process.GetLineage(e.ProcessPid)

	for _, pi := range pis {
		p := &api.Process{
			Pid:     pi.Pid,
			Command: pi.Command,
		}
		lineage = append(lineage, p)
	}

	e.ProcessLineage = lineage
}

// Cancel cancels an active telemetry subscription and returns a boolean
// indicating whether the specified subscription was found and
// canceled or not.
func (sb *subscriptionBroker) Cancel(subscription *api.Subscription) bool {
	glog.V(1).Infof("Canceling subscription %+v", subscription)

	sb.mu.Lock()
	defer sb.mu.Unlock()

	_, ok := sb.perfEventStreams[subscription]
	if ok {
		delete(sb.perfEventStreams, subscription)
		sb.update()
	}

	//
	// It's possible for a subscriber to subscribe to only
	// non-perf event streams (e.g. container events), so not
	// finding them in perfEventStreams is ok.
	//

	return ok
}

// NewSubscription creates a new telemetry subscription from the given
// api.Subscription descriptor. NewSubscription returns a stream.Stream of
// api.Events matching the specified filters. Closing the Stream cancels the
// subscription.
func NewSubscription(sub *api.Subscription) (*stream.Stream, error) {
	sb := getSubscriptionBroker()

	eventStream, _, err := sb.Subscribe(sub)
	if err != nil {
		glog.Errorf("Couldn't add subscription %v: %v", sub, err)
		return nil, err
	}

	ctrl := make(chan interface{})

	go func() {
		for {
			select {
			case _, ok := <-ctrl:
				if !ok {
					sb.Cancel(sub)
					return
				}
			}
		}
	}()

	return &stream.Stream{
		Ctrl: ctrl,
		Data: eventStream.Data,
	}, nil
}

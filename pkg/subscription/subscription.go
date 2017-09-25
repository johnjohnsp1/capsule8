package subscription

import (
	"sync"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/config"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/filter"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/process"
	"github.com/capsule8/reactive8/pkg/stream"
	"github.com/capsule8/reactive8/pkg/sys"
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
	syscalls  *syscallFilterSet
	processes *processFilterSet
	files     *fileFilterSet
	kprobes   *kprobeFilterSet
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

func (fs *filterSet) addKprobeEventFilter(kef *api.KernelFunctionCallFilter) {
	if fs.kprobes == nil {
		fs.kprobes = new(kprobeFilterSet)
	}

	fs.kprobes.add(kef)
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

	if fs.kprobes != nil {
		length += fs.kprobes.len()
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
	if fs.kprobes != nil {
		fs.kprobes.registerEvents(monitor)
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
	eventStreams           map[*api.Subscription]chan interface{}
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

func (sb *subscriptionBroker) onSampleEvent(sample interface{}, err error) {
	if sample != nil {
		event := sample.(*api.Event)

		sb.mu.Lock()
		defer sb.mu.Unlock()

		for _, c := range sb.eventStreams {
			c <- event
		}

	}
}

func (sb *subscriptionBroker) update() error {
	glog.V(2).Infof("Updating perf event filters...")

	fs := &filterSet{}

	for s := range sb.eventStreams {
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

			for _, kef := range ef.KernelEvents {
				fs.addKprobeEventFilter(kef)
			}
		}
	}

	//
	// Update perf
	//

	// Stop old perf session
	if sb.monitor != nil {
		glog.Info("Disabling existing perf session")
		sb.monitor.Close(false)
		sb.monitor = nil
	}

	// Create a new perf session only if we have perf event config info
	if fs.len() > 0 {
		var (
			monitor *perf.EventMonitor
			err     error
		)

		//
		// If a perf_event cgroupfs is mounted and a cgroup
		// name is configured (can be "/"), then monitor that
		// cgroup within the perf_event hierarchy. Otherwise,
		// monitor all processes on the system.
		//
		if len(sys.PerfEventDir()) > 0 && len(config.Sensor.CgroupName) > 0 {
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
		sb.monitor = monitor

		go func() {
			sb.monitor.Enable()

			sb.monitor.Run(sb.onSampleEvent)
			glog.Infof("perf.Run() returned, exiting goroutine")
		}()
	}

	return nil
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
	if sb.eventStreams == nil {
		sb.eventStreams = make(map[*api.Subscription]chan interface{})
	}

	sb.eventStreams[sub] = data

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
	glog.V(2).Infof("Subscribing to %+v", sub)

	sb.mu.Lock()
	defer sb.mu.Unlock()

	eventStream, joiner := stream.NewJoiner()
	joiner.Off()

	if len(sub.EventFilter.SyscallEvents) > 0 ||
		len(sub.EventFilter.ProcessEvents) > 0 ||
		len(sub.EventFilter.FileEvents) > 0 ||
		len(sub.EventFilter.KernelEvents) > 0 {

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
	// If adding process lineage to events, only do so after event filtering.
	// (No point in adding lineage to an event that is going to be filtered).
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
	sb.mu.Lock()
	defer sb.mu.Unlock()

	_, ok := sb.eventStreams[subscription]
	if ok {
		delete(sb.eventStreams, subscription)
		sb.update()
	}

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

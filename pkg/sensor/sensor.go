package sensor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/reactive8/pkg/container"
	"github.com/capsule8/reactive8/pkg/filter"
	"github.com/capsule8/reactive8/pkg/perf"
	"github.com/capsule8/reactive8/pkg/stream"
)

func (s *Sensor) decodeSample(sr *perf.SampleRecord) (interface{}, error) {
	var tracepointEventType uint16

	reader := bytes.NewReader(sr.RawData)
	binary.Read(reader, binary.LittleEndian, &tracepointEventType)

	val := s.decoders.Load()
	decoders := val.(map[uint16]traceEventDecoderFn)
	decoder := decoders[tracepointEventType]
	if decoder != nil {
		return decoder(sr.RawData)
	}

	return nil, nil
}

func (s *Sensor) onSampleEvent(perfEv *perf.Sample, err error) {
	switch perfEv.Record.(type) {
	case *perf.SampleRecord:
		sample := perfEv.Record.(*perf.SampleRecord)
		e, err := s.decodeSample(sample)
		if err != nil {
			log.Printf("Decoder error: %v", err)
			return
		}

		event := e.(*api.Event)

		// Use monotime based on perf event vs. Event construction
		event.SensorMonotimeNanos =
			HostMonotimeNanosToSensor(int64(sample.Time))

		for _, c := range s.eventStreams {
			c <- event
		}

	default:
		log.Printf("Unknown record type %T", perfEv.Record)

	}
}

func newTraceEventAttr(name string) *perf.EventAttr {
	eventID, err := perf.GetTraceEventID(name)

	if err != nil {
		return nil
	}

	sampleType :=
		perf.PERF_SAMPLE_TID | perf.PERF_SAMPLE_TIME |
			perf.PERF_SAMPLE_CPU | perf.PERF_SAMPLE_RAW

	return &perf.EventAttr{
		Disabled:        true,
		Type:            perf.PERF_TYPE_TRACEPOINT,
		Config:          uint64(eventID),
		SampleType:      sampleType,
		Inherit:         true,
		SampleIDAll:     true,
		SamplePeriod:    1,
		Watermark:       true,
		WakeupWatermark: 1,
	}
}

//
// Tracepoints and kprobes for system calls can be attached on syscall enter
// and/or syscall exit, so we track the filters for enter/exit separately.
//

type syscallEnterFilter struct {
	nr   int64
	args map[uint8]uint64
}

type syscallExitFilter struct {
	nr int64

	// Pointer is nil if no return value was specified
	ret *int64
}

type syscallFilterSet struct {
	// enter events
	enter []syscallEnterFilter

	// exit events
	exit []syscallExitFilter
}

func (ses *syscallFilterSet) addEnter(sef *api.SyscallEventFilter) {
	if sef.Id == nil {
		// No system call wildcards for now
		return
	}

	syscallEnter := &syscallEnterFilter{
		nr: sef.Id.Value,
	}

	m := make(map[uint8]uint64)

	if sef.Arg0 != nil {
		m[0] = sef.Arg0.Value
	}

	if sef.Arg1 != nil {
		m[1] = sef.Arg1.Value
	}

	if sef.Arg2 != nil {
		m[2] = sef.Arg2.Value
	}

	if sef.Arg3 != nil {
		m[3] = sef.Arg3.Value
	}

	if sef.Arg4 != nil {
		m[4] = sef.Arg4.Value
	}

	if sef.Arg5 != nil {
		m[5] = sef.Arg5.Value
	}

	if len(m) > 0 {
		syscallEnter.args = m
	}

	found := false
	for _, v := range ses.enter {
		if reflect.DeepEqual(syscallEnter, v) {
			found = true
			break
		}
	}

	if !found {
		ses.enter = append(ses.enter, *syscallEnter)
	}
}

func (ses *syscallFilterSet) addExit(sef *api.SyscallEventFilter) {
	if sef.Id == nil {
		// No system call wildcards for now
		return
	}

	syscallExit := &syscallExitFilter{
		nr: sef.Id.Value,
	}

	if sef.Ret != nil {
		syscallExit.ret = &sef.Ret.Value
	}

	found := false
	for _, v := range ses.enter {
		if reflect.DeepEqual(syscallExit, v) {
			found = true
			break
		}
	}

	if !found {
		ses.exit = append(ses.exit, *syscallExit)
	}
}

func (ses *syscallFilterSet) add(sef *api.SyscallEventFilter) {
	switch sef.Type {
	case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
		ses.addEnter(sef)

	case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
		ses.addExit(sef)
	}
}

func (ses *syscallFilterSet) len() int {
	return len(ses.enter) + len(ses.exit)
}

type processFilterSet struct {
	events map[api.ProcessEventType]struct{}
}

func (pes *processFilterSet) add(pef *api.ProcessEventFilter) {
	if pes.events == nil {
		pes.events = make(map[api.ProcessEventType]struct{})
	}

	pes.events[pef.Type] = struct{}{}
}

func (pes *processFilterSet) len() int {
	return len(pes.events)
}

type fileOpenFilter struct {
	filename        string
	filenamePattern string
	openFlagsMask   int32
	createModeMask  int32
}

type fileFilterSet struct {
	filters []*fileOpenFilter
}

func (fes *fileFilterSet) add(fef *api.FileEventFilter) {
	if fef.Type != api.FileEventType_FILE_EVENT_TYPE_OPEN {
		// Do nothing if UNKNOWN
		return
	}

	filter := new(fileOpenFilter)

	if fef.Filename != nil {
		filter.filename = fef.Filename.Value
	}

	if fef.FilenamePattern != nil {
		filter.filenamePattern = fef.FilenamePattern.Value
	}

	if fef.OpenFlagsMask != nil {
		filter.openFlagsMask = fef.OpenFlagsMask.Value
	}

	if fef.CreateModeMask != nil {
		filter.createModeMask = fef.CreateModeMask.Value
	}

	found := false
	for _, v := range fes.filters {
		if reflect.DeepEqual(filter, v) {
			found = true
			break
		}
	}

	if !found {
		fes.filters = append(fes.filters, filter)
	}
}

func (fes *fileFilterSet) len() int {
	return len(fes.filters)
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

// ---------------------------------------------------------------------

func (sfs *syscallFilterSet) getPerfEventAttrs() []*perf.EventAttr {
	var eventAttrs []*perf.EventAttr

	if len(sfs.enter) > 0 {
		if ea := newTraceEventAttr("raw_syscalls/sys_enter"); ea != nil {
			eventAttrs = append(eventAttrs, ea)
		}

	}

	if len(sfs.exit) > 0 {
		if ea := newTraceEventAttr("raw_syscalls/sys_exit"); ea != nil {
			eventAttrs = append(eventAttrs, ea)
		}

	}

	return eventAttrs
}

func (sfs *syscallFilterSet) getPerfFilters(filters map[uint16]string) {
	sysEnterID, err :=
		perf.GetTraceEventID("raw_syscalls/sys_enter")
	if err != nil {
		return
	}

	sysExitID, err :=
		perf.GetTraceEventID("raw_syscalls/sys_exit")
	if err != nil {
		return
	}

	var enter_filters []string
	var exit_filters []string

	for _, f := range sfs.enter {
		var filters []string

		s := fmt.Sprintf("id == %d", f.nr)
		filters = append(filters, s)

		for a, v := range f.args {
			s := fmt.Sprintf("args[%d] == %d", a, v)
			filters = append(filters, s)
		}

		s = fmt.Sprintf("(%s)", strings.Join(filters, " && "))
		enter_filters = append(enter_filters, s)
	}

	for _, f := range sfs.exit {
		var filters []string

		s := fmt.Sprintf("id == %d", f.nr)
		filters = append(filters, s)

		if f.ret != nil {
			s := fmt.Sprintf("ret == %d", *f.ret)
			filters = append(filters, s)
		}

		s = fmt.Sprintf("(%s)", strings.Join(filters, " && "))
		exit_filters = append(exit_filters, s)
	}

	if len(enter_filters) > 0 {
		filters[sysEnterID] = strings.Join(enter_filters, " || ")
	}

	if len(exit_filters) > 0 {
		filters[sysExitID] = strings.Join(exit_filters, " || ")
	}
}

func (pfs *processFilterSet) getPerfEventAttrs() []*perf.EventAttr {
	var eventAttrs []*perf.EventAttr

	_, ok := pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_FORK]
	if ok {
		if ea := newTraceEventAttr("sched/sched_process_fork"); ea != nil {
			eventAttrs = append(eventAttrs, ea)
		}
	}

	_, ok = pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC]
	if ok {
		if ea := newTraceEventAttr("sched/sched_process_exec"); ea != nil {
			eventAttrs = append(eventAttrs, ea)
		}
	}

	_, ok = pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT]
	if ok {
		if ea := newTraceEventAttr("syscalls/sys_enter_exit_group"); ea != nil {
			eventAttrs = append(eventAttrs, ea)
		}
	}

	return eventAttrs
}

func (pfs *processFilterSet) getPerfFilters(filters map[uint16]string) {
	_, ok := pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_FORK]
	if ok {
		eventID, _ := perf.GetTraceEventID("sched/sched_process_fork")

		filters[eventID] = ""
	}

	_, ok = pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC]
	if ok {
		eventID, _ := perf.GetTraceEventID("sched/sched_process_exec")

		filters[eventID] = ""
	}

	_, ok = pfs.events[api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT]
	if ok {
		eventID, _ :=
			perf.GetTraceEventID("syscalls/sys_enter_exit_group")

		filters[eventID] = ""
	}
}

func (ffs *fileFilterSet) getPerfEventAttrs() []*perf.EventAttr {
	if ffs.filters != nil {
		if ea := newTraceEventAttr("fs/do_sys_open"); ea != nil {
			return []*perf.EventAttr{ea}
		} else {
			return []*perf.EventAttr{}
		}
	}

	return nil
}

func (ffs *fileFilterSet) getPerfFilters(filters map[uint16]string) {
	eventID, err := perf.GetTraceEventID("fs/do_sys_open")
	if err != nil {
		return
	}

	var openFilters []string

	for _, f := range ffs.filters {
		var filters []string

		if len(f.filename) > 0 {
			s := fmt.Sprintf("filename == %s", f.filename)
			filters = append(filters, s)
		}

		if len(f.filenamePattern) > 0 {
			s := fmt.Sprintf("filename == %s", f.filenamePattern)
			filters = append(filters, s)
		}

		if f.openFlagsMask != 0 {
			s := fmt.Sprintf("flags & %d", f.openFlagsMask)
			filters = append(filters, s)
		}

		if f.createModeMask != 0 {
			s := fmt.Sprintf("mode & %d", f.createModeMask)
			filters = append(filters, s)
		}

		if filters != nil {
			s := fmt.Sprintf("(%s)", strings.Join(filters, " && "))
			openFilters = append(openFilters, s)
		}
	}

	filters[eventID] = strings.Join(openFilters, " || ")
}

func (fs *filterSet) getPerfEventAttrs() []*perf.EventAttr {
	var eventAttrs []*perf.EventAttr

	if fs.syscalls != nil {
		ea := fs.syscalls.getPerfEventAttrs()
		eventAttrs = append(eventAttrs, ea...)
	}

	if fs.processes != nil {
		ea := fs.processes.getPerfEventAttrs()
		eventAttrs = append(eventAttrs, ea...)
	}

	if fs.files != nil {
		ea := fs.files.getPerfEventAttrs()
		if ea != nil {
			eventAttrs = append(eventAttrs, ea...)
		}
	}

	return eventAttrs
}

func (fs *filterSet) getPerfFilters() map[uint16]string {
	filters := make(map[uint16]string)

	if fs.syscalls != nil {
		fs.syscalls.getPerfFilters(filters)
	}

	if fs.processes != nil {
		fs.processes.getPerfFilters(filters)
	}

	if fs.files != nil {
		fs.files.getPerfFilters(filters)
	}

	return filters
}

// ---------------------------------------------------------------------

type Sensor struct {
	mu           sync.Mutex
	decoders     atomic.Value // map[uint16]traceEventDecoderFn
	events       []*perf.EventAttr
	filters      []string
	perf         *perf.Perf
	eventStreams map[*api.Subscription]chan interface{}
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
		sensor = new(Sensor)
	})

	return sensor
}

type traceEventDecoderFn func([]byte) (interface{}, error)

func (s *Sensor) registerDecoder(ID uint16, decoder traceEventDecoderFn) {
	var decoders map[uint16]traceEventDecoderFn

	d := s.decoders.Load()

	if d == nil {
		decoders = make(map[uint16]traceEventDecoderFn)
	} else {
		decoders = d.(map[uint16]traceEventDecoderFn)
	}

	decoders[ID] = decoder

	s.decoders.Store(decoders)
}

func (s *Sensor) update() error {
	fs := new(filterSet)

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

	eventAttrs := fs.getPerfEventAttrs()
	filterMap := fs.getPerfFilters()
	filters := make([]string, len(eventAttrs))

	for i := range eventAttrs {
		filters[i] = filterMap[uint16(eventAttrs[i].Config)]
	}

	// Stop old perf session
	if s.perf != nil {
		s.perf.Disable()
		s.perf.Close()
		s.perf = nil
	}
	// Create a new perf session only if we have perf event config info
	if len(eventAttrs) > 0 {
		//
		// Attach to root cgroup of all docker containers
		//
		p, err := perf.NewWithCgroup(eventAttrs, filters, "/docker")
		if err != nil {
			return err
		}
		go func() {
			p.Run(s.onSampleEvent)
		}()
		p.Enable()
		s.perf = p
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

		joiner.Add(pes)
	}

	if len(sub.EventFilter.ContainerEvents) > 0 {
		//
		// Create a container event stream
		//
		ces, err := container.NewEventStream()
		if err != nil {
			joiner.Close()
			return nil, err
		}

		// Translate container events to protobuf versions
		ces = stream.Map(ces, translateContainerEvents)
		ces = stream.Filter(ces, filterNils)

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
		return nil, err
	}

	return eventStream, nil
}

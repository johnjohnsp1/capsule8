package sensor

import (
	"fmt"
	"strings"
	"syscall"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"

	"golang.org/x/sys/unix"
)

const (
	exitSymbol    = "do_exit"
	exitFetchargs = "code=%di:s64"
)

type processFilter struct {
	sensor *Sensor
}

func (f *processFilter) decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	childPid := data["child_pid"].(int32)
	childID := processId(childPid)

	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:         api.ProcessEventType_PROCESS_EVENT_TYPE_FORK,
			ForkChildPid: childPid,
			ForkChildId:  childID,
		},
	}

	return ev, nil
}

func (f *processFilter) decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	//
	// Grab command line out of procfs ASAP
	//
	hostPid := data["common_pid"].(int32)
	filename := data["filename"].(string)
	commandLine := sys.HostProcFS().CommandLine(hostPid)

	ev := f.sensor.NewEventFromSample(sample, data)
	processEvent := &api.ProcessEvent{
		Type:            api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
		ExecFilename:    filename,
		ExecCommandLine: commandLine,
	}

	ev.Event = &api.Event_Process{
		Process: processEvent,
	}

	return ev, nil
}

func (f *processFilter) decodeDoExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	var exitStatus int
	var exitSignal syscall.Signal
	var coreDumped bool

	// The kprobe fetches the argument 'long code' as an s64. It's
	// the same value returned as a status via waitpid(2).
	code := data["code"].(int64)

	ws := unix.WaitStatus(code)
	if ws.Exited() {
		exitStatus = ws.ExitStatus()
	} else if ws.Signaled() {
		exitSignal = ws.Signal()
		coreDumped = ws.CoreDump()
	}

	ev := f.sensor.NewEventFromSample(sample, data)
	ev.Event = &api.Event_Process{
		Process: &api.ProcessEvent{
			Type:           api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT,
			ExitCode:       int32(code),
			ExitStatus:     uint32(exitStatus),
			ExitSignal:     uint32(exitSignal),
			ExitCoreDumped: coreDumped,
		},
	}

	return ev, nil
}

func processFilterString(wildcard bool, filters map[string]bool) string {
	if wildcard {
		return ""
	}

	parts := make([]string, 0, len(filters))
	for k := range filters {
		parts = append(parts, fmt.Sprintf("(%s)", k))
	}
	return strings.Join(parts, " || ")
}

func registerProcessEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.ProcessEventFilter) {
	forkFilter := false
	execFilters := make(map[string]bool)
	execWildcard := false
	exitFilters := make(map[string]bool)
	exitWildcard := false

	for _, pef := range events {
		switch pef.Type {
		case api.ProcessEventType_PROCESS_EVENT_TYPE_FORK:
			forkFilter = true
		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC:
			if pef.ExecFilename != nil {
				s := fmt.Sprintf("filename == %s", pef.ExecFilename.Value)
				execFilters[s] = true
			} else if pef.ExecFilenamePattern != nil {
				s := fmt.Sprintf("filenamePattern ~ %s", pef.ExecFilenamePattern.Value)
				execFilters[s] = true
			} else {
				execWildcard = true
			}
		case api.ProcessEventType_PROCESS_EVENT_TYPE_EXIT:
			if pef.ExitCode != nil {
				s := fmt.Sprintf("code == %d", pef.ExitCode.Value)
				exitFilters[s] = true
			} else {
				exitWildcard = true
			}
			break
		default:
			continue
		}
	}

	f := processFilter{
		sensor: sensor,
	}

	if forkFilter {
		eventName := "sched/sched_process_fork"
		err := monitor.RegisterEvent(eventName,
			f.decodeSchedProcessFork, "", nil)

		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v",
				eventName, err)
		}
	}

	if execWildcard || len(execFilters) > 0 {
		filter := processFilterString(execWildcard, execFilters)

		eventName := "sched/sched_process_exec"
		err := monitor.RegisterEvent(eventName,
			f.decodeSchedProcessExec, filter, nil)
		if err != nil {
			glog.V(1).Infof("Couldn't get %s event id: %v",
				eventName, err)
		}
	}

	if exitWildcard || len(exitFilters) > 0 {
		filter := processFilterString(exitWildcard, exitFilters)

		name := perf.UniqueProbeName("capsule8", "do_exit")
		_, err := monitor.RegisterKprobe(name, exitSymbol,
			false, exitFetchargs, f.decodeDoExit, filter, nil)
		if err != nil {
			glog.Errorf("Couldn't register kprobe for %s: %s",
				exitSymbol, err)
		}
	}
}

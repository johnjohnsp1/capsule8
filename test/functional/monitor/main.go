package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"
)

const (
	exitSymbol    = "do_exit"
	exitFetchargs = "code=%di:s64"
)

var nEvents uint
var nSamples uint

func decodeSchedProcessFork(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	nEvents++
	glog.V(2).Infof("%+v %+v", sample, data)

	return nil, nil
}

func decodeSchedProcessExec(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	nEvents++
	glog.V(2).Infof("%+v %+v", sample, data)

	return nil, nil
}

func decodeDoExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	nEvents++
	glog.V(2).Infof("%+v %+v", sample, data)

	return nil, nil
}

func onSample(sample interface{}, err error) {
	nSamples++
	// Do nothing
}

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	glog.Info("Creating monitor on cgroup /docker")
	monitor, err := perf.NewEventMonitorWithCgroup("/docker", 0, 0, nil)

	eventName := "sched/sched_process_fork"
	err = monitor.RegisterEvent(eventName, decodeSchedProcessFork,
		"", nil)
	if err != nil {
		glog.Fatal(err)
	}

	eventName = "sched/sched_process_exec"
	err = monitor.RegisterEvent(eventName, decodeSchedProcessExec,
		"", nil)
	if err != nil {
		glog.Fatal(err)
	}

	_, err = monitor.RegisterKprobe("capsule8/do_exit", exitSymbol,
		false, exitFetchargs, decodeDoExit, "", nil)
	if err != nil {
		glog.Fatal(err)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		glog.Infof("Caught signal %v, stopping monitor", sig)
		monitor.Stop(true)
	}()

	glog.Info("Enabling monitor")
	monitor.Enable()

	glog.Info("Running monitor")
	monitor.Run(onSample)
	glog.Info("Monitor stopped")

	glog.Infof("Received %d samples, %d events",
		nSamples, nEvents)
}

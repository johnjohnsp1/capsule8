// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	_, err = monitor.RegisterTracepoint(eventName, decodeSchedProcessFork,
		"", nil)
	if err != nil {
		glog.Fatal(err)
	}

	eventName = "sched/sched_process_exec"
	_, err = monitor.RegisterTracepoint(eventName, decodeSchedProcessExec,
		"", nil)
	if err != nil {
		glog.Fatal(err)
	}

	_, err = monitor.RegisterKprobe(exitSymbol,
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
	monitor.EnableAll()

	glog.Info("Running monitor")
	monitor.Run(onSample)
	glog.Info("Monitor stopped")

	glog.Infof("Received %d samples, %d events",
		nSamples, nEvents)
}

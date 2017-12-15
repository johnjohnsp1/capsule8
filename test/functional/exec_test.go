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

package functional

import (
	"testing"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type execTest struct {
	testContainer   *Container
	err             error
	containerID     string
	containerExited bool
	processID       string
}

func (ct *execTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "exec")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	ct.testContainer = c
	return ct.testContainer.ImageID
}

func (ct *execTest) RunContainer(t *testing.T) {
	err := ct.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
}

func (ct *execTest) CreateSubscription(t *testing.T) *api.Subscription {
	processEvents := []*api.ProcessEventFilter{
		&api.ProcessEventFilter{
			Type: api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC,
			ExecFilenamePattern: &wrappers.StringValue{
				// Want to only match execs of 'uname'
				Value: "*nam*",
			},
		},
	}

	eventFilter := &api.EventFilter{
		ProcessEvents: processEvents,
	}

	sub := &api.Subscription{
		EventFilter: eventFilter,
	}

	return sub
}

func (ct *execTest) HandleTelemetryEvent(t *testing.T, telemetryEvent *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", telemetryEvent)

	switch event := telemetryEvent.Event.Event.(type) {
	case *api.Event_Process:
		if event.Process.Type == api.ProcessEventType_PROCESS_EVENT_TYPE_EXEC &&
			event.Process.ExecFilename == "/bin/uname" {

			if len(ct.processID) > 0 {
				t.Error("Already saw process exec")
				return false
			}

			ct.processID = telemetryEvent.Event.ProcessId
			glog.V(1).Infof("processID = %s", ct.processID)
		}
	}

	return len(ct.processID) == 0
}

// TestExec exercises filtering PROCESS_EVENT_TYPE_EXEC by ExecFilenamePattern
func TestExec(t *testing.T) {
	et := &execTest{}
	tt := NewTelemetryTester(et)
	tt.RunTest(t)
}

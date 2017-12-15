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
	"time"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"
)

const (
	testInterval   = 100 * time.Millisecond
	expectedEvents = 10
)

type tickerTest struct {
	firstEventNanos int64
	count           int
}

func (tickTest *tickerTest) BuildContainer(t *testing.T) string {
	// No container is needed for testing
	return ""
}

func (tickTest *tickerTest) RunContainer(t *testing.T) {
	// No container is needed for testing, nothing to do.
}

func (tickTest *tickerTest) CreateSubscription(t *testing.T) *api.Subscription {
	tickerEvents := []*api.TickerEventFilter{
		&api.TickerEventFilter{
			Interval: int64(testInterval),
		},
	}

	eventFilter := &api.EventFilter{
		TickerEvents: tickerEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (tickTest *tickerTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", te)

	switch event := te.Event.Event.(type) {
	case *api.Event_Ticker:
		if tickTest.count == 0 {
			tickTest.firstEventNanos = event.Ticker.Nanoseconds
		}
		tickTest.count++

		glog.V(1).Infof("tickTest.count = %d", tickTest.count)

		if tickTest.count == expectedEvents {
			nanos := event.Ticker.Nanoseconds

			d := time.Duration(nanos - tickTest.firstEventNanos)
			glog.V(1).Infof("Duration: %s", d)

			//
			// Duration should be =~ (expectedEvents - 1) * testInterval
			//
			if d < (expectedEvents-2)*testInterval ||
				d > expectedEvents*testInterval {
				t.Error("Observed event duration out of range")
				return false
			}
		}

		return tickTest.count < expectedEvents

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

//
// TestChargen checks that with a subscription including a TickerEvents filter,
// the sensor will generate the expected number of ticker events in a given time
// interval.
//
func TestTicker(t *testing.T) {
	tickTest := &tickerTest{}

	tt := NewTelemetryTester(tickTest)
	tt.RunTest(t)

	if tickTest.count != expectedEvents {
		t.Errorf("Expected %d ticker events, got %d\n", expectedEvents, tickTest.count)
	}
}

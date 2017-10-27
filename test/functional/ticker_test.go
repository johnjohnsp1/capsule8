package functional

import (
	"testing"
	"time"

	api "github.com/capsule8/api/v0"
	"github.com/golang/glog"
)

const (
	testInterval   = 100 * time.Millisecond
	expectedEvents = 10
)

type tickerTest struct {
	count    int
	deadline time.Time
}

func (tickTest *tickerTest) BuildContainer(t *testing.T) {
	// No container is needed for testing, nothing to do.
}

func (tickTest *tickerTest) RunContainer(t *testing.T) {
	// No container is needed for testing, use this to set the deadline.
	tickTest.deadline = time.Now().Add(expectedEvents * testInterval)
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
		tickTest.count++
		return time.Now().Before(tickTest.deadline)

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

	if diff := tickTest.count - expectedEvents; diff < -1 || diff > 1 {
		t.Errorf("Expected %d ticker events, got %d\n", expectedEvents, tickTest.count)
	}
}

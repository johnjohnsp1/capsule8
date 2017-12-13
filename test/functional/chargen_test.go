package functional

import (
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"
)

const chargenLength = 40
const chargenEventCount = 32

type chargenTest struct {
	count int
}

func (ct *chargenTest) BuildContainer(t *testing.T) string {
	// No container is needed for testing, nothing to do.
	return ""
}

func (ct *chargenTest) RunContainer(t *testing.T) {
	// No container is needed for testing, nothing to do.
}

func (ct *chargenTest) CreateSubscription(t *testing.T) *api.Subscription {
	chargenEvents := []*api.ChargenEventFilter{
		&api.ChargenEventFilter{
			Length: chargenLength,
		},
	}

	eventFilter := &api.EventFilter{
		ChargenEvents: chargenEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (ct *chargenTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", te)

	switch event := te.Event.Event.(type) {
	case *api.Event_Chargen:
		if len(event.Chargen.Characters) != chargenLength {
			t.Errorf("Event %#v has the wrong number of characters.\n", *event.Chargen)
			return false
		}

		ct.count++
		return ct.count < chargenEventCount

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

//
// TestChargen checks that with a subscription including a ChargenEvents filter,
// the sensor will generate appropriate Chargen events.
//
func TestChargen(t *testing.T) {

	ct := &chargenTest{}

	tt := NewTelemetryTester(ct)
	tt.RunTest(t)
}

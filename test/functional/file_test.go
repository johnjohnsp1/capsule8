package functional

import (
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/golang/glog"
	"github.com/golang/protobuf/ptypes/wrappers"
)

type fileTest struct {
	testContainer *Container
	openEvts      map[string]*api.FileEvent
}

func newFileTest() (*fileTest, error) {
	oe, err := fileTestDataMap()
	if err != nil {
		return nil, err
	}
	return &fileTest{openEvts: oe}, nil
}

func (ft *fileTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "file")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	glog.V(1).Infof("Built container %s\n", c.ImageID[0:12])
	ft.testContainer = c
	return ft.testContainer.ImageID
}

func (ft *fileTest) RunContainer(t *testing.T) {
	err := ft.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(1).Infof("Running container %s\n", ft.testContainer.ImageID[0:12])
}

func (ft *fileTest) CreateSubscription(t *testing.T) *api.Subscription {
	fileEvents := []*api.FileEventFilter{}
	for _, fe := range ft.openEvts {
		fileEvents = append(fileEvents, filterForTestData(fe))
	}

	eventFilter := &api.EventFilter{
		FileEvents: fileEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (ft *fileTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {
	glog.V(2).Infof("%+v", te)
	switch event := te.Event.Event.(type) {
	case *api.Event_File:
		if td, ok := ft.openEvts[event.File.Filename]; ok {
			if !eventMatchFileTestData(event.File, td) {
				t.Errorf("Expected %#v, got %#v\n", td, event.File)
			}
			delete(ft.openEvts, event.File.Filename)
		}
	}

	glog.V(1).Infof("openEvts = %+v", ft.openEvts)
	return len(ft.openEvts) > 0
}

func filterForTestData(fe *api.FileEvent) *api.FileEventFilter {
	return &api.FileEventFilter{
		Type:     api.FileEventType_FILE_EVENT_TYPE_OPEN,
		Filename: &wrappers.StringValue{Value: fe.Filename},
	}
}

//
// TestFile checks that the sensor generates file open events when requested by
// the subscription. The file file/testdata/filedata.txt specifies the test
// cases.
//
func TestFile(t *testing.T) {
	ft, err := newFileTest()
	if err != nil {
		t.Fatal(err)
	}

	tt := NewTelemetryTester(ft)
	tt.RunTest(t)
}

package sensor

import (
	"testing"

	api "github.com/capsule8/capsule8/api/v0"
)

func TestFilterContainerId(t *testing.T) {
	cf := newContainerFilter(&api.ContainerFilter{
		Ids: []string{
			"alice",
			"bob",
		},
	})

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "alice",
	}); !match {
		t.Error("No matching container ID found for alice")
	}

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "bill",
	}); match {
		t.Error("Unexpected matching container ID found for bill")
	}
}

func TestFilterContainerImageId(t *testing.T) {
	cf := newContainerFilter(&api.ContainerFilter{
		ImageIds: []string{
			"alice",
			"bob",
		},
	})

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "pass",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				ImageId: "alice",
			},
		},
	}); !match {
		t.Error("No matching container image ID found for alice")
	}

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "fail",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				ImageId: "bill",
			},
		},
	}); match {
		t.Error("Unexpected matching container image ID found for bill")
	}
}

func TestFilterContainerImageNames(t *testing.T) {
	cf := newContainerFilter(&api.ContainerFilter{
		ImageNames: []string{
			"alice",
			"bob",
		},
	})

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "pass",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				ImageName: "alice",
			},
		},
	}); !match {
		t.Error("No matching image name found for alice")
	}

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "fail",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				ImageName: "bill",
			},
		},
	}); match {
		t.Error("Unexpected matching image name found for bill")
	}
}

func TestFilterContainerNames(t *testing.T) {
	cf := newContainerFilter(&api.ContainerFilter{
		Names: []string{
			"alice",
			"bob",
		},
	})

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "pass",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				Name: "alice",
			},
		},
	}); !match {
		t.Error("No matching container name found for alice")
	}

	if match := cf.FilterFunc(&api.Event{
		ContainerId: "fail",
		Event: &api.Event_Container{
			Container: &api.ContainerEvent{
				Name: "bill",
			},
		},
	}); match {
		t.Error("Unexpected matching container name found for bill")
	}
}

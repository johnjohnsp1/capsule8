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

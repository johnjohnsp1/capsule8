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
	"encoding/binary"
	"fmt"
	"syscall"
	"testing"
	"unsafe"

	api "github.com/capsule8/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/golang/glog"
)

const (
	// These need to be coordinated with the code in network/main.go
	testNetworkPort    = 8080
	testNetworkBacklog = 128
	testNetworkMsgLen  = 5
)

var (
	testNetworkPortN = hton16(testNetworkPort)
)

func hton16(port uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, port)
	return *(*uint16)(unsafe.Pointer(&b[0]))
}

type networkTest struct {
	testContainer *Container
	containerID   string
	serverSocket  uint64
	clientSocket  uint64
	seenEvents    map[api.NetworkEventType]bool
}

func (nt *networkTest) BuildContainer(t *testing.T) string {
	c := NewContainer(t, "network")
	err := c.Build()
	if err != nil {
		t.Error(err)
		return ""
	}

	glog.V(2).Infof("Built container %s\n", c.ImageID[0:12])
	nt.testContainer = c
	return nt.testContainer.ImageID
}

func portListItem(port uint16) string {
	return fmt.Sprintf("%d:%d", port, port)
}

func (nt *networkTest) RunContainer(t *testing.T) {
	err := nt.testContainer.Run()
	if err != nil {
		t.Error(err)
	}
	glog.V(2).Infof("Running container %s\n", nt.testContainer.ImageID[0:12])
}

func (nt *networkTest) CreateSubscription(t *testing.T) *api.Subscription {
	familyFilter := expression.Equal(
		expression.Identifier("sa_family"),
		expression.Value(uint16(syscall.AF_INET)))
	portFilter := expression.Equal(
		expression.Identifier("sin_port"),
		expression.Value(testNetworkPortN))
	resultFilter := expression.Equal(
		expression.Identifier("ret"),
		expression.Value(int32(0)))
	backlogFilter := expression.Equal(
		expression.Identifier("backlog"),
		expression.Value(int32(testNetworkBacklog)))
	/*
		goodFDFilter := expression.GreaterThan(
			expression.Identifier("ret"),
			expression.Value(int32(-1)))
	*/

	msgLenFilter := expression.Equal(
		expression.Identifier("ret"),
		expression.Value(int32(testNetworkMsgLen)))

	networkEvents := []*api.NetworkEventFilter{
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT,
			FilterExpression: expression.LogicalAnd(familyFilter, portFilter),
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT,
			//FilterExpression: resultFilter,
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT,
			FilterExpression: expression.LogicalAnd(familyFilter, portFilter),
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT,
			FilterExpression: resultFilter,
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT,
			FilterExpression: backlogFilter,
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT,
			FilterExpression: resultFilter,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT,
			//FilterExpression: goodFDFilter,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT,
			FilterExpression: msgLenFilter,
		},
		&api.NetworkEventFilter{
			Type: api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT,
		},
		&api.NetworkEventFilter{
			Type:             api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT,
			FilterExpression: msgLenFilter,
		},
	}

	// Subscribing to container created events are currently necessary
	// to get imageIDs in other events.
	containerEvents := []*api.ContainerEventFilter{
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED,
		},
		&api.ContainerEventFilter{
			Type: api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED,
		},
	}

	eventFilter := &api.EventFilter{
		NetworkEvents:   networkEvents,
		ContainerEvents: containerEvents,
	}

	return &api.Subscription{
		EventFilter: eventFilter,
	}
}

func (nt *networkTest) HandleTelemetryEvent(t *testing.T, te *api.TelemetryEvent) bool {

	switch event := te.Event.Event.(type) {
	case *api.Event_Container:
		switch event.Container.Type {
		case api.ContainerEventType_CONTAINER_EVENT_TYPE_CREATED:
			return true

		case api.ContainerEventType_CONTAINER_EVENT_TYPE_EXITED:
			unseen := []api.NetworkEventType{}
			for i := api.NetworkEventType(1); i <= 12; i++ {
				if !nt.seenEvents[i] {
					unseen = append(unseen, i)
				}
			}
			if len(unseen) > 0 {
				t.Logf("Never saw network event(s) %+v\n", unseen)
			}
			return true

		default:
			t.Errorf("Unexpected Container event %+v\n", event)
			return false
		}

	case *api.Event_Network:
		glog.V(2).Infof("Got Network Event %+v\n", te.Event)
		if te.Event.ImageId == nt.testContainer.ImageID {
			switch event.Network.Type {
			case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT:
				nt.clientSocket = event.Network.Sockfd

			case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT:
				// The golang runtime uses non-blocking sockets, so a successful connect will
				// return an EINPROGRESS. The container also attempts connecting to TCP6, so
				// we also get an EADDRNOTAVAIL (-99).
				if event.Network.Result != 0 && event.Network.Result != -int64(syscall.EADDRNOTAVAIL) {
					// Don't mark this event as successfully received
					return true
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT:
				if event.Network.Address.Family != api.NetworkAddressFamily_NETWORK_ADDRESS_FAMILY_INET {
					t.Errorf("Expected bind family %s, got %s",
						api.NetworkAddressFamily_NETWORK_ADDRESS_FAMILY_INET,
						event.Network.Address.Family)
					return false
				} else {
					addr, have_addr := event.Network.Address.Address.(*api.NetworkAddress_Ipv4Address)

					if !have_addr {
						t.Errorf("Unexpected bind address %+v", event.Network.Address.Address)
						return false
					} else if addr.Ipv4Address.Port != uint32(testNetworkPortN) {
						t.Errorf("Expected bind port %d, got %d",
							testNetworkPortN, addr.Ipv4Address.Port)
						return false
					}
				}
				nt.serverSocket = event.Network.Sockfd

			case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT:
				if event.Network.Result != 0 {
					t.Errorf("Expected bind result 0, got %d",
						event.Network.Result)
					return false
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT:
				if event.Network.Backlog != testNetworkBacklog {
					t.Errorf("Expected listen backlog %d, got %d",
						testNetworkBacklog, event.Network.Backlog)
					return false
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT:
				if event.Network.Result != 0 {
					t.Errorf("Expected listen result 0, got %d",
						event.Network.Result)
					return false
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT:
				if nt.serverSocket != 0 && event.Network.Sockfd != nt.serverSocket {
					// This is not the accept() attempt we are looking for
					return true
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT:
				if event.Network.Result < 0 && event.Network.Result != -int64(syscall.EAGAIN) {
					t.Errorf("Expected accept result > -1, got %d",
						event.Network.Result)
					return false
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT:
				if event.Network.Result != int64(testNetworkMsgLen) {
					t.Errorf("Expected sendto result %d, got %d",
						testNetworkMsgLen, event.Network.Result)
					return false
				}

			case api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT:
				if event.Network.Result != int64(testNetworkMsgLen) {
					t.Errorf("Expected recvfrom result %d, got %d",
						testNetworkMsgLen, event.Network.Result)
					return false
				}

			}

			nt.seenEvents[event.Network.Type] = true
		}

		return len(nt.seenEvents) < 12

	default:
		t.Errorf("Unexpected event type %T\n", event)
		return false
	}
}

// TestNetwork exercises the network events.
func TestNetwork(t *testing.T) {
	nt := &networkTest{seenEvents: make(map[api.NetworkEventType]bool)}

	tt := NewTelemetryTester(nt)
	tt.RunTest(t)
}

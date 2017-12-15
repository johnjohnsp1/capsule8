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

	"github.com/capsule8/capsule8/pkg/container/c8dockerclient"
	"github.com/golang/glog"
)

func inspectContainer(client *c8dockerclient.Client, id string) {
	containerInfo, err := client.InspectContainer(id)
	if err == nil {
		var imageInfo *c8dockerclient.DockerImageInfo

		imageInfo, err = client.InspectImage(containerInfo.ImageID)
		if err != nil {
			glog.Infoln("Error:", err)
			return
		}

		glog.Infoln(imageInfo.String())
		glog.Infoln(containerInfo.String())

		for networkKey, network := range containerInfo.NetworkSettings.Networks {
			var networkInfo *c8dockerclient.DockerNetworkInfo
			networkInfo, err = client.InspectNetwork(network.NetworkID)

			if err == nil {
				glog.Infoln("Network entry:", networkKey)
				glog.Infoln(networkInfo.String())
			} else {
				glog.Infoln("Error enumerating network:", err)
			}

		}

		var processes []*c8dockerclient.ProcessEntry
		processes, err = client.ContainerTop(id)
		if err != nil {
			glog.Infoln("Error:", err)
			return
		}

		for _, process := range processes {
			glog.Infoln(process.String())
		}

	} else {
		glog.Infoln("Error", err)
	}

}

func main() {
	flag.Parse()
	client := c8dockerclient.NewClient()
	dockerInfo, err := client.DockerInfo()
	if err != nil {
		glog.Fatal("Error:", err)
	}

	glog.Infoln(dockerInfo.String())

	eventChannel, stopChan, err := client.EventChannel()
	defer close(stopChan)

	if err != nil {
		glog.Fatal("Error: ", err)
	}

	glog.Infoln("Enumerating existing containers")
	containers, err := client.ListContainers()
	if err != nil {
		glog.Fatal("Error:", err)
	}

	for _, enumeratedContainer := range containers {
		glog.Infoln("Inspecting:", enumeratedContainer.ContainerID)
		inspectContainer(client, enumeratedContainer.ContainerID)
	}

	glog.Infoln("Entering loop for event messages")
	for {
		event := <-eventChannel
		switch event.Type {
		case "network":
			// tell the pcapper
			if event.Action == "connect" {
				// connect: docker inspect to get info about network
				glog.Infoln("container network connect")
				glog.Infoln(event.String())

			} else if event.Action == "disconnect" {
				// do inspect on docker to get all interfaces/ips ??
				// maybe this was the last container with this interface
				// and so pcapping can conclude
				glog.Infoln("container network disconnect")
				glog.Infoln(event.String())
			}

		case "container":
			// container event
			if event.Action == "start" {
				// creation
				// tell aggregator
				// since we do not know a "connect" will happen
				glog.Infoln("container created")
				inspectContainer(client, event.ID)

			} else if event.Action == "die" {
				// death
				// tell aggregator
				glog.Infoln("container death")
				glog.Infoln(event.String())
			}

		default:
			glog.Infoln("UNKNOWN event action ", event.Action)
			glog.Infoln(event.String())
		}
	}
}

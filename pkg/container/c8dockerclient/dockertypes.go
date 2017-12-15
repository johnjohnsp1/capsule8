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

//  Name:           c8dockerclient dockertypes
//  Description:    types from Docker source (JSON objects)
//  Author:         Brandon Edwards, brandon@capsule8.io
//  Copyright:      Copyright (c) 2016 Capsule8, Inc.
//

package c8dockerclient

import (
	"fmt"
	"strings"
	"time"
)

const (
	MOUNT_TYPE_BIND = "bind"
)

//DockerInfo represents the information extracted from a docker info command.
type DockerInfo struct {
	DockerID          string `json:"ID"`
	DockerVersion     string `json:"ServerVersion"`
	KernelVersion     string `json:"KernelVersion"`
	OS                string `json:"OperatingSystem"`
	OSType            string `json:"OSType"`
	Hostname          string `json:"Name"`
	Architecture      string `json:"Architecture"`
	ContainersRunning int    `json:"ContainersRunning"`
}

func (info *DockerInfo) String() string {
	output := "DockerID: " + info.DockerID + "\n"
	output += "DockerVersion: " + info.DockerVersion + "\n"
	output += "OS: " + info.OS + "\n"
	output += "OSType: " + info.OSType + "\n"
	output += "KernelVersion: " + info.KernelVersion + "\n"
	output += "Hostname: " + info.Hostname + "\n"
	output += "Architecture: " + info.Architecture + "\n"
	return output
}

//DockerEventActor represents the container or image that a Docker Event affects
type DockerEventActor struct {
	ID         string            `json:"ID"`
	Attributes map[string]string `json:"Attributes"`
}

func (actor *DockerEventActor) String() string {
	output := "ActorID: " + actor.ID + "\n"
	for key, value := range actor.Attributes {
		output += fmt.Sprintf("\t- %s: %s\n", key, value)
	}

	return output
}

//DockerEventMessage encapsulates all information about a Docker event.
type DockerEventMessage struct {
	Status   string `json:"status,omitempty"`
	ID       string `json:"id,omitempty"`
	From     string `json:"from,omitempty"`
	Type     string `json:"Type"`
	Action   string `json:"Action"`
	Actor    DockerEventActor
	Time     int64 `json:"time,omitempty"` //docker uses unix timestamps for these
	TimeNano int64 `json:"timeNano,omitempty"`
}

func (event *DockerEventMessage) String() string {
	output := "Status: " + event.Status + "\n"
	output += "Id: " + event.ID + "\n"
	output += "From: " + event.From + "\n"
	output += "Type: " + event.Type + "\n"
	output += "Action: " + event.Action + "\n"
	output += event.Actor.String()

	output += fmt.Sprintf("Time: %v\n", event.Time)
	output += fmt.Sprintf("Nano: %v\n", event.TimeNano)
	return output
}

//DockerPortForward contains the container's host's forwarded IP and ports
//e.g. port 2222 on an external IP of a host is actually passed to that container
type DockerPortForward struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// DockerNetwork represents a docker network that a container is attached to.
// This is a substructure of DockerContainerNetwork settings
type DockerNetwork struct {
	NetworkID string `json:"NetworkID"`
	IPAddress string `json:"IPAddress"`
}

func (network *DockerNetwork) String() string {
	output := fmt.Sprintf("IP: %s, Network ID: %s\n", network.IPAddress,
		network.NetworkID)
	return output
}

// DockerContainerNetworkSettings holds the network configuration information
// for a running Docker container. This is a substructure of DockerContainerInfo
type DockerContainerNetworkSettings struct {
	// This IP might NOT be what we want
	// there is also an IP in each DockerNetwork entry in Networks map
	IPAddress   string                         `json:"IPAddress"`
	IPPrefixLen uint32                         `json:"IPPrefixLen"`
	Ports       map[string][]DockerPortForward `json:"Ports"`
	Networks    map[string]DockerNetwork       `json:"Networks"` //maps network type to network ID
}

func (settings *DockerContainerNetworkSettings) String() string {
	output := fmt.Sprintf("IpAddress: %s\n", settings.IPAddress)
	for networkType, network := range settings.Networks {
		output += "Network Type: " + networkType + "\n"
		output += "\t" + network.String()
	}

	return output
}

//DockerContainerState represents when the container was started and its ProcessID.
//This is a sub-structure of DockerContainerInfo
type DockerContainerState struct {
	StartTime time.Time `json:"StartedAt"`
	//PIDs in linux are limited to 4,000,000
	//see http://lxr.free-electrons.com/source/include/linux/threads.h#L33
	ProcessID uint64 `json:"Pid"`
}

//DockerVolumeMounts represents the metadata associated.
type DockerVolumeMounts struct {
	Type        string `json:"Type"`        // this is the type of volume (bind)
	Source      string `json:"Source"`      // this is the file path in the container host
	Destination string `json:"Destination"` // this is the file path inside the container
	ReadWrite   bool   `json:"RW"`          // is this mounted readwrite (true = yes)
	Mode        string `json:"Mode"`        // the mode the volume uses
	Propagation string `json:"Propagation"` // this lets us know if the volume changes get
	// propagated see https://lwn.net/Articles/689856/
}

//DockerContainerInfo represents the result of inspecting a docker container.
type DockerContainerInfo struct {
	Name            string                         `json:"Name"`
	Path            string                         `json:"Path"`
	Arguments       []string                       `json:"Args"`
	ContainerID     string                         `json:"Id"`
	ImageID         string                         `json:"Image"`
	State           DockerContainerState           `json:"State"`
	NetworkSettings DockerContainerNetworkSettings `json:"NetworkSettings"`
	Mounts          []DockerVolumeMounts           `json:"Mounts"`
}

func (info *DockerContainerInfo) String() string {
	output := "Name: " + info.Name + "\n"
	output += "Path: " + info.Path + "\n"
	output += "ContainerID: " + info.ContainerID + "\n"
	output += "ImageID: " + info.ImageID + "\n"
	output += "IpAddress: " + info.NetworkSettings.IPAddress + "\n"
	output += "StartTime: " + info.State.StartTime.Format(time.RFC3339Nano) + "\n"
	output += fmt.Sprintf("ProcessId: %d\n", info.State.ProcessID)
	output += "Arguments:\n"
	for _, argument := range info.Arguments {
		output += fmt.Sprintf("\t%s\n", argument)
	}

	output += info.NetworkSettings.String()
	return output
}

//DockerContainerListInfo lists the ContainerID.
//The information returned by ContainerList (/containers/json)
//This type is used in container-enumeration at daemon start-up
type DockerContainerListInfo struct {
	//The only field we need is ContainerID, we then Inspect it to
	//get DockerContainerInfo
	ContainerID string `json:"ID"`
}

func (container *DockerContainerListInfo) String() string {
	return "ContainerId: " + container.ContainerID + "\n"
}

//RootFSLayers represents the RootFS json object found when inspecting an image.
//This represents all of the Image SHA1s that comprise the layers of this image.
type RootFSLayers struct {
	Type   string   `json:"Type"`
	Layers []string `json:"Layers"`
}

//DockerImageInfo represents the JSON object returned by ImageInspect
// (/images/<imageID>/json). The only field currently of interest at
// time of writing is RepoTags
//TODO: add layer information here
type DockerImageInfo struct {
	ImageID  string       `json:"ID"`
	ParentID string       `json:"Parent"`
	RepoTags []string     `json:"RepoTags"`
	Layers   RootFSLayers `json:"RootFS"`
}

func (imageInfo *DockerImageInfo) String() string {
	st := "DockerImageInfo{ImageID: \""
	st += imageInfo.ImageID + "\", "
	st += "ParentID: \"" + imageInfo.ParentID + "\", "
	st += "RepoTags: [" + strings.Join(imageInfo.RepoTags, ",") + "],"
	st += "Layers: " + strings.Join(imageInfo.Layers.Layers, ",") + "}"
	return "ImageName: " + imageInfo.RepoTags[0]
}

// DockerNetworkInfo is returned by NetworkInspect (/networks/<networkdID>)
type DockerNetworkInfo struct {
	Name    string
	Options map[string]string `json:"Options"`
}

func (network *DockerNetworkInfo) String() string {
	output := "Network name: " + network.Name + "\n"
	for key, value := range network.Options {
		output += fmt.Sprintf("\t%s: %s\n", key, value)
	}

	return output
}

//ProcessEntry represents a node in a process tree as found from docker top
type ProcessEntry struct {
	User            string `json:"user"`
	Command         string `json:"command"`
	ProcessID       uint64 `json:"pid"`
	ParentProcessID uint64 `json:"ppid"`
	CGroup          uint64 `json:"cgroup"`
}

func (process *ProcessEntry) String() string {
	output := "User: " + process.User + "\n"
	output += "Command: " + process.Command + "\n"
	output += fmt.Sprintf("ProcessID: %d\n", process.ProcessID)
	output += fmt.Sprintf("ParentProcessID: %d\n", process.ParentProcessID)
	output += fmt.Sprintf("CGroup: %d\n", process.CGroup)
	return output
}

//DockerContainerProcessList is a helper struct for parsing the json
//from Docker's Top command
type DockerContainerProcessList struct {
	Processes [][]string `json:"Processes"`
	Titles    []string   `json:"Titles"`
}

func (processList *DockerContainerProcessList) String() string {
	output := ""
	for _, title := range processList.Titles {
		output += title + "\t"
	}
	output += "\n"

	for _, process := range processList.Processes {
		for _, info := range process {
			output += info + "\t"
		}
		output += "\n"
	}

	return output
}

const (
	DIFF_MODIFIED uint8 = 0
	DIFF_ADDED    uint8 = 1
	DIFF_DELETED  uint8 = 2
)

//DockerFileChange is the json object returned by docker diff
type DockerFileChange struct {
	Kind uint8  `json:"kind"`
	Path string `json:"path"`
}

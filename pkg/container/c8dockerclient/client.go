//  Name:           c8dockerclient
//  Description:    Our own docker client (avoid silly versioning)
//  Author:         Brandon Edwards, brandon@capsule8.io
//  Copyright:      Copyright (c) 2016 Capsule8, Inc.
//                  All rights reserved.

//Package c8dockerclient is a homegrown HTTP Docker API client.
//It is primarily used with the Docker unix socket /var/run/docker.sock
//to retrieve information about running containers when using the Docker
//container engine.
package c8dockerclient

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

//DockerSocketPath is the filesytem path to the docker socket
const ApiPrefix = "/v1.24"
const DockerSocketPath = "/var/run/docker.sock"

//ClientError encapsulates all errors
type ClientError struct {
	message string
}

//Return the error
func (c *ClientError) Error() string {
	return c.message
}

//Client serves as the main structure for dealing with the docker socket
type Client struct {
	SocketPath string
}

//Request makes an HTTP request
func (client *Client) Request(path, method string, values *url.Values) (resp *http.Response, err error) {
	//for now assume api version is in the path already (if necessary)
	var request *http.Request
	if values == nil {
		request, err = http.NewRequest(method, path, nil)
	} else {
		request, err = http.NewRequest(method, path,
			strings.NewReader(values.Encode()))
		request.Header.Add("Content-Type", "application/json")
	}

	if err != nil {
		return nil, err
	}

	// "connect" to the unix socket
	connection, err := net.Dial("unix", client.SocketPath)
	if err != nil {
		return nil, err
	}

	// get an http client
	clientConnection := httputil.NewClientConn(connection, nil)

	// make the request
	response, err := clientConnection.Do(request)
	if err != nil {
		return nil, err
	}

	// check response status
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		if len(body) == 0 {
			err = &ClientError{message: http.StatusText(response.StatusCode)}
			return nil, err
		}

		err = &ClientError{message: fmt.Sprintf("%s: %s",
			http.StatusText(response.StatusCode), body)}

		return nil, err
	}
	return response, nil
}

//DockerInfo gets the docker engine version, OS and more.
func (client *Client) DockerInfo() (*DockerInfo, error) {
	var info DockerInfo

	response, err := client.Request(ApiPrefix+"/info", "GET", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

//EventChannel connects to the Docker socket and executes the docker events
//command, this returns a channel for receiving those events, and an error
func (client *Client) EventChannel() (chan DockerEventMessage, chan interface{}, error) {
	response, err := client.Request(ApiPrefix+"/events", "GET", nil)

	if err != nil {
		return nil, nil, err
	}

	stopChan := make(chan interface{})
	eventChannel := make(chan DockerEventMessage, 64)
	go func() {
		defer response.Body.Close()
		defer close(eventChannel)
		jsonDecoder := json.NewDecoder(response.Body)
	sendLoop:
		for {
			select {
			case <-stopChan:
				break sendLoop
			default:
				eventMessage := DockerEventMessage{}
				err := jsonDecoder.Decode(&eventMessage)
				if err != nil {
					break sendLoop
				}

				eventChannel <- eventMessage
			}
		}
	}()

	return eventChannel, stopChan, nil
}

//InspectContainer gets all of the information the docker engine has on a container
//via it's /inspect URI
func (client *Client) InspectContainer(containerID string) (*DockerContainerInfo, error) {
	var info DockerContainerInfo
	urlPath := ApiPrefix + "/containers/" + containerID + "/json"
	response, err := client.Request(urlPath, "GET", nil)

	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

//ListContainers lists all of the running containers.
func (client *Client) ListContainers() ([]DockerContainerListInfo, error) {
	var info []DockerContainerListInfo
	response, err := client.Request(ApiPrefix+"/containers/json", "GET", nil)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &info)
	if err != nil {
		return nil, err
	}

	return info, err
}

//InspectImage retrieves all information about an image for the given imageID
func (client *Client) InspectImage(imageID string) (*DockerImageInfo, error) {
	var info DockerImageInfo

	urlPath := ApiPrefix + "/images/" + imageID + "/json"
	response, err := client.Request(urlPath, "GET", nil)

	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &info)
	if err != nil {
		return nil, err
	}
	return &info, err
}

//InspectNetwork gets all network information
func (client *Client) InspectNetwork(networkID string) (*DockerNetworkInfo, error) {
	var info DockerNetworkInfo

	urlPath := ApiPrefix + "/networks/" + networkID
	response, err := client.Request(urlPath, "GET", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &info)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

//parseProcessEntryDocker parses a Process list object from Docker Top output.
func parseProcessEntryDocker(entry []string, titles []string) (process *ProcessEntry, err error) {
	process = &ProcessEntry{}
	for i, title := range titles {
		switch title {
		case "UID":
			fallthrough
		case "USER":
			process.User = entry[i]
		case "CMD":
			fallthrough
		case "COMMAND":
			process.Command = entry[i]
		case "PPID":
			ppid, err := strconv.Atoi(entry[i])
			if err != nil {
				return nil, err
			}
			process.ParentProcessID = uint64(ppid)
		case "PID":
			pid, err := strconv.Atoi(entry[i])
			if err != nil {
				return nil, err
			}
			process.ProcessID = uint64(pid)
		}
	}
	return process, nil
}

//ContainerTop gets the processes in the container specified by id. This is primarily
//used to list the processes in running containers that may have started before our
//instrumentation.
func (client *Client) ContainerTop(containerID string) ([]*ProcessEntry, error) {
	var processes []*ProcessEntry
	var processList DockerContainerProcessList

	urlPath := ApiPrefix + "/containers/" + containerID + "/top"
	response, err := client.Request(urlPath, "GET", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &processList)
	if err != nil {
		return nil, err
	}

	for _, entry := range processList.Processes {
		process, err := parseProcessEntryDocker(entry, processList.Titles)
		if err != nil {
			log.Println(string(jsonText))
			log.Println(err)
			return nil, err
		}
		processes = append(processes, process)
	}

	return processes, nil
}

//ContainerDiff diffs the file system from when the container was started to
//when the function was called
func (client *Client) ContainerDiff(containerID string) (fileList []DockerFileChange,
	err error) {

	urlPath := ApiPrefix + "/containers/" + containerID + "/changes"
	response, err := client.Request(urlPath, "GET", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	jsonText, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(jsonText, &fileList)
	if err != nil {
		return nil, err
	}
	return fileList, nil
}

//RestartContainer restarts the container with the given containerID.
func (client *Client) RestartContainer(containerID string) (err error) {
	urlPath := ApiPrefix + "/containers/" + containerID + "/restart"
	response, err := client.Request(urlPath, "POST", nil)
	if err != nil {
		return err
	}

	response.Body.Close()
	return nil
}

// KillContainer terminates the container process by sending the signal
// but does not remove the container from the docker host.
func (client *Client) KillContainer(containerID, signal string) (err error) {
	query := url.Values{}
	query.Set("signal", signal)

	url := ApiPrefix + "/containers/" + containerID + "/kill"
	resp, err := client.Request(url, "POST", &query)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		msg := "Failed to kill container " + containerID
		return &ClientError{message: msg}
	}
	return nil
}

//NewClient creates a new C8DockerClient instance tied to local docker socket.
func NewClient() (cli *Client) {
	return &Client{SocketPath: DockerSocketPath}
}

/*
TODO: implement these functions
func (cli *Client) CheckpointCreate(ctx context.Context, container string, options types.CheckpointCreateOptions) error {
	resp, err := cli.post(ctx, "/containers/"+container+"/checkpoints", nil, options, nil)
	ensureReaderClosed(resp)
	return err
}

//CreateCheckpoint creates a memory snapshot of a container.
/*TODO implement
func (client *Client) CreateCheckpoint(containerID string) (err error) {
	resp, err := client.Request("/containers/"+containerID+"/checkpoints", "POST", nil)
	if err != nil {
		return
	}
	return nil
}

//TODO implement
func (client *Client) CommitContainerAndTag() {
	return
}

//PushImage pushes an image to the desired docker registry
func (client *Client) PushImage(id string, registryUrl string) (err error) {
	return nil
}
*/

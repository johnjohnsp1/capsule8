//  Name:           testingc8client
//  Description:
//  Author:         Brandon Edwards, brandon@capsule8.io
//  Copyright:      Copyright (c) 2016 Capsule8, Inc.
//                  All rights reserved.

package main

import (
	"log"

	"github.com/capsule8/reactive8/pkg/container/c8dockerclient"
)

func inspectContainer(client *c8dockerclient.Client, id string) {
	containerInfo, err := client.InspectContainer(id)
	if err == nil {
		imageInfo, err := client.InspectImage(containerInfo.ImageID)
		if err != nil {
			log.Println("Error:", err)
			return
		}

		log.Println(imageInfo.String())
		log.Println(containerInfo.String())

		for networkKey, network := range containerInfo.NetworkSettings.Networks {
			networkInfo, err := client.InspectNetwork(network.NetworkID)

			if err == nil {
				log.Println("Network entry:", networkKey)
				log.Println(networkInfo.String())
			} else {
				log.Println("Error enumerating network:", err)
			}

		}

		processes, err := client.ContainerTop(id)
		if err != nil {
			log.Println("Error:", err)
			return
		}

		for _, process := range processes {
			log.Println(process.String())
		}

	} else {
		log.Println("Error", err)
	}

}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	client := c8dockerclient.NewClient()
	dockerInfo, err := client.DockerInfo()
	if err != nil {
		log.Fatal("Error:", err)
	}

	log.Println(dockerInfo.String())

	eventChannel, err := client.EventChannel()

	if err != nil {
		log.Fatal("Error: ", err)
	}

	log.Println("Enumerating existing containers")
	containers, err := client.ListContainers()
	if err != nil {
		log.Fatal("Error:", err)
	}

	for _, enumeratedContainer := range containers {
		log.Println("Inspecting:", enumeratedContainer.ContainerID)
		inspectContainer(client, enumeratedContainer.ContainerID)
	}

	log.Println("Entering loop for event messages")
	for {
		event := <-eventChannel
		switch event.Type {
		case "network":
			// tell the pcapper
			if event.Action == "connect" {
				// connect: docker inspect to get info about network
				log.Println("container network connect")
				log.Println(event.String())

			} else if event.Action == "disconnect" {
				// do inspect on docker to get all interfaces/ips ??
				// maybe this was the last container with this interface
				// and so pcapping can conclude
				log.Println("container network disconnect")
				log.Println(event.String())
			}

		case "container":
			// container event
			if event.Action == "start" {
				// creation
				// tell aggregator
				// since we do not know a "connect" will happen
				log.Println("container created")
				inspectContainer(client, event.ID)

			} else if event.Action == "die" {
				// death
				// tell aggregator
				log.Println("container death")
				log.Println(event.String())
			}

		default:
			log.Println("UNKNOWN event action ", event.Action)
			log.Println(event.String())
		}
	}
}

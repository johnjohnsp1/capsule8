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
	"fmt"
	"net"
	"sync"

	"github.com/golang/glog"
)

const (
	tcpPort = 8080
	udpPort = 8081
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Parse()

	tcp4(tcpPort)
	tcp6(tcpPort)
	udp4(udpPort)
	udp6(udpPort)
}

func tcp4(port int) {
	glog.Infof("Creating TCP listener on port %d", port)
	ln, err := net.Listen("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Errorf("Couldn't create listener: %v", err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		glog.Infof("Accepting TCP connections on port %d", port)
		conn, err := ln.Accept()
		if err != nil {
			glog.Errorf("Couldn't accept connection: %v", err)
			return
		}

		// Send hello
		conn.Write([]byte("PING\n"))

		// Receive response
		pong := make([]byte, 5)
		n, err := conn.Read(pong)
		if err != nil {
			glog.Errorf("Couldn't read pong: %v", err)
			return

		} else if n < len(pong) {
			glog.Errorf("Short read (%d < %d)", n, len(pong))
			return
		}

		glog.Infof("Got: %v", pong)
		wg.Done()
	}()

	conn, err := net.Dial("tcp4", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Errorf("Couldn't dial TCP port %d: %v", port, err)
		return
	}

	// Read Hello from server
	ping := make([]byte, 5)
	n, err := conn.Read(ping)
	if err != nil {
		glog.Errorf("Couldn't read ping: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short read (%d < %d)", n, len(ping))
		return
	}

	n, err = conn.Write([]byte("PONG\n"))
	if err != nil {
		glog.Errorf("Couldn't write pong: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short write (%d < %d)", n, len(ping))
		return
	}

	wg.Wait()
}

func tcp6(port int) {
	glog.Infof("Creating TCP listener on port %d", port)
	ln, err := net.Listen("tcp6", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Errorf("Couldn't create listener: %v", err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		glog.Infof("Accepting TCP6 connections on port %d", port)
		conn, err := ln.Accept()
		if err != nil {
			glog.Errorf("Couldn't accept connection: %v", err)
			return
		}

		// Send hello
		conn.Write([]byte("PING\n"))

		// Receive response
		pong := make([]byte, 5)
		n, err := conn.Read(pong)
		if err != nil {
			glog.Errorf("Couldn't read pong: %v", err)
			return
		} else if n < len(pong) {
			glog.Errorf("Short read (%d < %d)", n, len(pong))
			return
		}

		glog.Infof("Got: %v", pong)
		wg.Done()
	}()

	conn, err := net.Dial("tcp6", fmt.Sprintf(":%d", port))
	if err != nil {
		glog.Errorf("Couldn't dial TCP6 port %d: %v", port, err)
		return
	}

	// Read Hello from server
	ping := make([]byte, 5)
	n, err := conn.Read(ping)
	if err != nil {
		glog.Errorf("Couldn't read ping: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short read (%d < %d)", n, len(ping))
		return
	}

	n, err = conn.Write([]byte("PONG\n"))
	if err != nil {
		glog.Errorf("Couldn't write pong: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short write (%d < %d)", n, len(ping))
		return
	}

	wg.Wait()
}

func udp4(port int) {
	addr := net.UDPAddr{Port: port}
	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		glog.Errorf("Couldn't listen on %v: %v", addr, err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		ping := make([]byte, 5)
		glog.Infof("Reading PING from UDP port %d", port)
		n, _, err := conn.ReadFromUDP(ping)
		if err != nil {
			glog.Errorf("Couldn't send ping: %v", err)
			return
		} else if n < len(ping) {
			glog.Errorf("Short write (%d < %d)", n, len(ping))
			return
		}

		glog.Infof("Got: %v", ping)
		wg.Done()
	}()

	ping := []byte("PING\n")
	glog.Infof("Sending PING to UDP port %d", port)
	n, err := conn.WriteToUDP(ping, &addr)
	if err != nil {
		glog.Errorf("Couldn't send ping: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short write (%d < %d)", n, len(ping))
		return
	}

	wg.Wait()
}

func udp6(port int) {
	addr := net.UDPAddr{IP: net.IPv6loopback, Port: port}
	conn, err := net.ListenUDP("udp6", &addr)
	if err != nil {
		glog.Errorf("Couldn't listen on %v: %v", addr, err)
		return
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		ping := make([]byte, 5)
		glog.Infof("Reading PING from UDP6 port %d", port)
		n, _, err := conn.ReadFromUDP(ping)
		if err != nil {
			glog.Errorf("Couldn't send ping: %v", err)
			return
		} else if n < len(ping) {
			glog.Errorf("Short write (%d < %d)", n, len(ping))
			return
		}

		glog.Infof("Got: %v", ping)
		wg.Done()
	}()

	ping := []byte("PING\n")
	glog.Infof("Sending PING to UDP6 port %d", port)
	n, err := conn.WriteToUDP(ping, &addr)
	if err != nil {
		glog.Errorf("Couldn't send ping: %v", err)
		return
	} else if n < len(ping) {
		glog.Errorf("Short write (%d < %d)", n, len(ping))
		return
	}

	wg.Wait()
}

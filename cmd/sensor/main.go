// Copyright 2017 Capsule8 Inc. All rights reserved.

package main

import (
	"flag"

	"github.com/capsule8/capsule8/pkg/sensor"
	"github.com/capsule8/capsule8/pkg/version"
)

func main() {
	// Set "alsologtostderr" flag so that glog messages go stderr as well as /tmp.
	flag.Set("alsologtostderr", "true")
	flag.Parse()

	// Log version and build at "Starting ..." for debugging
	version.InitialBuildLog("sensor")

	sensor.Main()
}

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

// Package version defines globals containing version metadata set at build-time
package version

import (
	"fmt"

	"github.com/golang/glog"
)

var (
	// Version is a SemVer 2.0 formatted version string
	Version string

	// Build is an opaque string identifying the automated build job
	Build string
)

type versionResponse struct {
	Version string `json:"version"`
	Build   string `json:"build,omitempty"`
}

// InitialBuildLog uses glog.Info to log version information
// On an initial creation event all components should announce their creation, version and build information
func InitialBuildLog(componentName string) {
	var buildLog string
	if Build != "" {
		buildLog = fmt.Sprintf(" [%s]", Build)
	}

	glog.Infof("Starting %s (%s)%s", componentName, Version, buildLog)
}

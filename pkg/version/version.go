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

// InitialBuildLog uses glog.Info to log version information
// On an initial creation event all components should announce their creation, version and build information
func InitialBuildLog(componentName string) {
	var buildLog string
	if Build != "" {
		buildLog = fmt.Sprintf(" [%s]", Build)
	}

	glog.Infof("Starting %s (%s)%s", componentName, Version, buildLog)
}

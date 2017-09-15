// Package version defines globals containing version metadata set at build-time
package version

import (
	"bytes"
	"fmt"

	"github.com/golang/glog"
)

var (
	// Version is a SemVer 2.0 formatted version string
	Version string

	// Build is an opaque string identifying the automated build job
	Build  string
	buffer bytes.Buffer
)

// InitialBuildLog uses glog.Info to log version information
// On an initial creation event all components should announce their creation, version and build information
func InitialBuildLog(componentName string) {
	buffer.WriteString(fmt.Sprintf("Started Capsule8 %s Version: %s", componentName, Version))

	// If the Build string is empty we only log the version information
	if Build != "" {
		buffer.WriteString(fmt.Sprintf(" Build: %s", Build))
	}

	glog.Infof(buffer.String())
}

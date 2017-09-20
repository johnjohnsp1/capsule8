// Package version defines globals containing version metadata set at build-time
package version

import (
	"fmt"
	"net/http"

	"github.com/coreos/pkg/httputil"
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

// HTTPHandler exposes version information through an http handler
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	err := httputil.WriteJSONResponse(w, http.StatusOK, versionResponse{
		Version: Version,
		Build:   Build,
	})
	if err != nil {
		glog.Errorf("Failed to write JSON response: %v", err)
	}
}

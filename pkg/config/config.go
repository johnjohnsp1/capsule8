package config

import (
	"github.com/golang/glog"
	"github.com/kelseyhightower/envconfig"
)

// Global contains overridable configuration options that apply globally
var Global struct {
	// RunDir is the path to the runtime state directory for Capsule8
	RunDir string `split_words:"true" default:"/var/run/capsule8"`

	// HTTP port for the pprof runtime profiling endpoint.
	ProfilingPort int `split_words:"true"`
}

// Sensor contains overridable configuration options for the sensor
var Sensor struct {
	// Node name to use if not the value returned from uname(2)
	NodeName string

	// DockerContainerDir is the path to the directory used for docker
	// container local storage areas (i.e. /var/lib/docker/containers)
	DockerContainerDir string `split_words:"true" default:"/var/lib/docker/containers"`

	// OciContainerDir is the path to the directory used for the
	// container runtime's container state directories
	// (i.e. /var/run/docker/libcontainerd)
	OciContainerDir string `split_words:"true" default:"/var/run/docker/libcontainerd"`

	// Subscription timeout in seconds
	SubscriptionTimeout int64 `default:"5"`

	// Sensor gRPC API Server listen address may be specified as any of:
	//   unix:/path/to/socket
	//   127.0.0.1:8484
	//   :8484
	ListenAddr string `split_words:"true" default:"unix:/var/run/capsule8/sensor.sock"`

	// Name of Cgroup to monitor for events. The cgroup specified must
	// exist within the perf_event cgroup hierarchy. If this is set to
	// "docker", the Sensor will monitor containers for events and ignore
	// processes not running in Docker containers. To monitor the entire
	// system, this can be set to "" or "/".
	CgroupName string `split_words:"true"`

	// The default size of ring buffers used for kernel perf_event
	// monitors. The size is defined in units of pages.
	RingBufferPages int `split_words:"true" default:"8"`

	// Ignore missing debugfs/tracefs mount (useful for automated testing)
	DontMountTracing bool `split_words:"true"`

	// Ignore missing perf_event cgroup filesystem mount
	DontMountPerfEvent bool `split_words:"true"`
}

func init() {
	err := envconfig.Process("C8", &Global)
	if err != nil {
		glog.Fatal(err)
	}

	err = envconfig.Process("C8_SENSOR", &Sensor)
	if err != nil {
		glog.Fatal(err)
	}
}

package config

import (
	"log"
	"os"

	"github.com/kelseyhightower/envconfig"
)

// Config contains overridable configuration options for the Sensor
var Sensor struct {
	ProcFs   string `split_words:"true" default:"/proc"`
	CgroupFs string `split_words:"true" default:"/sys/fs/cgroup"`
	TraceFs  string `split_words:"true" default:"/sys/kernel/debug/tracing"`

	// Node name to use if not the value returned from uname(2)
	NodeName string

	// DockerContainerDir is the path to the directory used for docker
	// container local storage areas (i.e. /var/lib/docker/containers)
	DockerContainerDir string `split_words:"true" default:"/var/lib/docker/containers"`

	// OciContainerDir is the path to the directory used for the
	// container runtime's container state directories
	// (i.e. /var/run/docker/libcontainerd)
	OciContainerDir string `split_words:"true" default:"/var/run/docker/libcontainerd"`

	// Pubsub backend implementation to use
	Pubsub string `default:"stan"`

	// Subscription timeout in seconds
	SubscriptionTimeout int64 `default:"5"`
}

var ApiServer struct {
	Pubsub    string `default:"stan"`
	Port      int    `default:"8080"`
	ProxyPort int    `default:"8081"`
}

var Backplane struct {
	ClusterName string `default:"c8-backplane"`
	NatsURL     string `default:"nats://localhost:4222"`
	AckWait     int    `default:"1"`
}

func init() {
	err := envconfig.Process("C8_APISERVER", &ApiServer)
	if err != nil {
		log.Fatal(err)
	}

	err = envconfig.Process("C8_SENSOR", &Sensor)
	if err != nil {
		log.Fatal(err)
	}

	err = envconfig.Process("C8_BACKPLANE", &Backplane)
	if err != nil {
		log.Fatal(err)
	}

	fi, err := os.Stat(Sensor.ProcFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Printf("C8_SENSOR_PROC_FS %s not a directory\n",
			Sensor.ProcFs)
	}

	fi, err = os.Stat(Sensor.CgroupFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Printf("C8_SENSOR_CGROUP_FS %s not a directory\n",
			Sensor.CgroupFs)
	}

	fi, err = os.Stat(Sensor.TraceFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Printf("C8_SENSOR_TRACE_FS %s not a directory\n",
			Sensor.TraceFs)
	}

	fi, err = os.Stat(Sensor.DockerContainerDir)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Printf("C8_SENSOR_DOCKER_CONTAINER_DIR %s not a directory\n",
			Sensor.DockerContainerDir)
	}

	fi, err = os.Stat(Sensor.OciContainerDir)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Printf("C8_SENSOR_OCI_CONTAINER_DIR %s not a directory\n",
			Sensor.OciContainerDir)
	}

}

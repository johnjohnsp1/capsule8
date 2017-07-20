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
}

func init() {
	err := envconfig.Process("C8_SENSOR", &Sensor)
	if err != nil {
		log.Fatal(err)
	}

	fi, err := os.Stat(Sensor.ProcFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Print("C8_SENSOR_PROC_FS %s not a directory\n",
			Sensor.ProcFs)
	}

	fi, err = os.Stat(Sensor.CgroupFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Print("C8_SENSOR_CGROUP_FS %s not a directory\n",
			Sensor.CgroupFs)
	}

	fi, err = os.Stat(Sensor.TraceFs)
	if err != nil {
		log.Print(err)
	} else if !fi.IsDir() {
		log.Print("C8_SENSOR_TRACE_FS %s not a directory\n",
			Sensor.TraceFs)
	}
}

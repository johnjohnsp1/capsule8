package subscription

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sync/atomic"
	"time"

	"github.com/capsule8/capsule8/pkg/container"
	"github.com/capsule8/capsule8/pkg/process"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/go/src/math/rand"

	api "github.com/capsule8/api/v0"
	"golang.org/x/sys/unix"
)

// Number of random bytes to generate for Sensor ID
const sensorIDLengthBytes = 32

var (
	// SensorID is a unique ID of the running instance of the
	// Sensor. A restart of the Sensor generates a new ID.
	SensorID string

	// Sensor-unique event sequence number. Each event sent from
	// the Sensor to any Subscription has a unique sequence number
	// for the indicated Sensor ID.
	sequenceNumber uint64

	// Record the value of CLOCK_MONOTONIC_RAW when the Sensor
	// starts up. All event monotimes are relative to this value.
	sensorBootMonotimeNanos int64
)

func init() {
	randomBytes := make([]byte, sensorIDLengthBytes)
	rand.Read(randomBytes)

	SensorID = hex.EncodeToString(randomBytes[:])

	sequenceNumber = 0

	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)

	d := ts.Nsec
	sensorBootMonotimeNanos = d
}

// HostMonotimeNanosToSensor converts the given host monotonic clock
// time to monotonic nanos since Sensor start time. This is done to
// ensure that timestamps on emitted events can only be compared among
// events with the same SensorID value.
func HostMonotimeNanosToSensor(hostMonotime int64) int64 {
	return hostMonotime - sensorBootMonotimeNanos
}

func getMonotimeNanos() int64 {
	ts := unix.Timespec{}
	unix.ClockGettime(unix.CLOCK_MONOTONIC_RAW, &ts)
	d := ts.Nsec + (ts.Sec * int64(time.Second))

	return HostMonotimeNanosToSensor(d)
}

func getNextSequenceNumber() uint64 {
	//
	// The rirst sequence number is intentionally 1 to disambiguate
	// from no sequence number being included in the protobuf message.
	//
	sequenceNumber++
	return sequenceNumber
}

// NewEvent creates a new api.Event
func NewEvent() *api.Event {
	monotime := getMonotimeNanos()
	sequenceNumber := getNextSequenceNumber()

	var b []byte
	buf := bytes.NewBuffer(b)

	binary.Write(buf, binary.LittleEndian, SensorID)
	binary.Write(buf, binary.LittleEndian, sequenceNumber)
	binary.Write(buf, binary.LittleEndian, monotime)

	h := sha256.Sum256(buf.Bytes())
	eventID := hex.EncodeToString(h[:])

	atomic.AddUint64(&Metrics.Events, 1)

	return &api.Event{
		Id:                   eventID,
		SensorId:             SensorID,
		SensorMonotimeNanos:  monotime,
		SensorSequenceNumber: sequenceNumber,
	}
}

func newEventFromContainer(containerID string) *api.Event {
	e := NewEvent()
	e.ContainerId = containerID
	return e
}

func newEventFromSample(sample *perf.SampleRecord, data map[string]interface{}) *api.Event {
	e := NewEvent()

	// Use monotime based on perf event vs. Event construction
	e.SensorMonotimeNanos = HostMonotimeNanosToSensor(int64(sample.Time))

	//
	// When both the Sensor and the process generating the sample
	// are in containers, the sample.Pid and sample.Tid fields
	// will be zero. Use the common_pid from the trace event
	// instead.
	//
	e.ProcessPid = data["common_pid"].(int32)

	// If Tid is available, use it
	if sample.Tid > 0 {
		e.ProcessTid = int32(sample.Tid)
	}

	e.Cpu = int32(sample.CPU)

	// Add an associated containerID
	procInfo := process.GetInfo(e.ProcessPid)
	if procInfo != nil {
		e.ProcessId = procInfo.UniqueID

		containerID := procInfo.ContainerID
		if len(containerID) > 0 {
			containerInfo := container.GetInfo(containerID)
			if containerInfo != nil {
				e.ContainerId = containerID
				e.ContainerName = containerInfo.Name
				e.ImageId = containerInfo.ImageID
				e.ImageName = containerInfo.ImageName
			}
		}
	}

	return e
}

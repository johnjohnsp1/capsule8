package sensor

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"encoding/hex"

	"encoding/binary"

	api "github.com/capsule8/reactive8/pkg/api/v0"
	"github.com/capsule8/reactive8/pkg/perf"
	"golang.org/x/sys/unix"
)

// Number of random bytes to generate for Sensor ID
const sensorIDLengthBytes = 32

// Sensor ID that is unique to the running instance of the Sensor. A restart
// of the Sensor generates a new SensorID.
var SensorID string

// Sensor-unique event sequence number. Each event sent from the Sensor to any
// Subscription has a unique sequence number for the indicated Sensor ID.
var sequenceNumber uint64

// Record the value of CLOCK_MONOTONIC_RAW when the Sensor starts up. All event
// monotimes are relative to this value.
var sensorBootMonotimeNanos int64

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
	sequenceNumber += 1
	return sequenceNumber
}

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

	return &api.Event{
		Id:                   eventID,
		SensorId:             SensorID,
		SensorMonotimeNanos:  monotime,
		SensorSequenceNumber: sequenceNumber,
	}
}

func newEventFromTraceEvent(traceEvent *perf.TraceEvent) *api.Event {
	e := NewEvent()

	e.ContainerId, _ = pidMapGetContainerID(traceEvent.Pid)

	// Even when the Sensor is running in a container and the event
	// occurs within a different container, the traceEvent.Pid field
	// is present and contains the host pid.
	e.ProcessPid = traceEvent.Pid

	return e
}

func newEventFromContainer(containerID string) *api.Event {
	e := NewEvent()
	e.ContainerId = containerID
	return e
}

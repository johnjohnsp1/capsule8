package perf

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/unix"
)

// mmap'd ring buffer must be 1+2^n pages. We optimize for low latency, so
// we shouldn't need a large ringbuffer memory region.
const numRingBufferPages = 1 + (1 << 0)

type ringBuffer struct {
	fd         int
	sampleType uint64
	readFormat uint64
	memory     []byte
	metadata   *metadata
	data       []byte
}

func newRingBuffer(fd int, sampleType uint64, readFormat uint64) (*ringBuffer, error) {
	pageSize := os.Getpagesize()

	memory, err := unix.Mmap(fd, 0, numRingBufferPages*pageSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	rb := new(ringBuffer)
	rb.fd = fd
	rb.sampleType = sampleType
	rb.readFormat = readFormat
	rb.memory = memory
	rb.metadata = (*metadata)(unsafe.Pointer(&memory[0]))
	rb.data = memory[os.Getpagesize():]

	for {
		seq := atomic.LoadUint32(&rb.metadata.Lock)
		if seq%2 != 0 {
			// seqlock must be even before value is read
			continue
		}

		version := atomic.LoadUint32(&rb.metadata.Version)
		compatVersion := atomic.LoadUint32(&rb.metadata.CompatVersion)

		if atomic.LoadUint32(&rb.metadata.Lock) != seq {
			// seqlock must be even and the same after values have been read
			continue
		}

		if version != 0 || compatVersion != 0 {
			return nil, errors.New("Incompatible ring buffer memory layout version")
		}

		break
	}

	return rb, nil
}

// Read calls the given function on each available record in the ringbuffer
func (rb *ringBuffer) read(f func(*Sample, error)) {
	var dataHead, dataTail uint64

	dataTail = rb.metadata.DataTail
	dataHead = atomic.LoadUint64(&rb.metadata.DataHead)

	for dataTail < dataHead {
		dataBegin := dataTail % uint64(len(rb.data))
		dataEnd := dataHead % uint64(len(rb.data))

		var data []byte
		if dataEnd >= dataBegin {
			data = rb.data[dataBegin:dataEnd]
		} else {
			data = rb.data[dataBegin:]
			data = append(data, rb.data[:dataEnd]...)
		}

		reader := bytes.NewReader(data)

		// Read all events in the buffer
		for reader.Len() > 0 {
			sample := Sample{}
			err := sample.read(reader, rb.sampleType, rb.readFormat)

			// Pass err to callback to notify caller of it.
			f(&sample, err)
		}

		//
		// Write dataHead to dataTail to let kernel know that we've
		// consumed the data up to it.
		//
		dataTail = dataHead
		atomic.StoreUint64(&rb.metadata.DataTail, dataTail)

		// Update dataHead in case it has been advanced in the interim
		dataHead = atomic.LoadUint64(&rb.metadata.DataHead)
	}
}

func (rb *ringBuffer) readOnCond(cond *sync.Cond, fn func(*Sample, error)) {
	for {
		cond.L.Lock()
		cond.Wait()

		if rb.memory != nil {
			rb.read(fn)
			cond.L.Unlock()
		} else {
			cond.L.Unlock()
			break
		}
	}
}

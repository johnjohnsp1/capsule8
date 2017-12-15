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

package perf

import (
	"errors"
	"os"
	"sync/atomic"
	"unsafe"

	"github.com/capsule8/capsule8/pkg/config"

	"golang.org/x/sys/unix"
)

type ringBuffer struct {
	fd       int
	memory   []byte
	metadata *metadata
	data     []byte
}

func newRingBuffer(fd int, pageCount int) (*ringBuffer, error) {
	pageSize := os.Getpagesize()

	if pageCount <= 0 {
		pageCount = config.Sensor.RingBufferPages
	}

	memory, err := unix.Mmap(fd, 0, (pageCount+1)*pageSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}

	rb := &ringBuffer{
		fd:       fd,
		memory:   memory,
		metadata: (*metadata)(unsafe.Pointer(&memory[0])),
		data:     memory[pageSize:],
	}

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

func (rb *ringBuffer) unmap() error {
	return unix.Munmap(rb.memory)
}

func (rb *ringBuffer) timeRunning() uint64 {
	return atomic.LoadUint64(&rb.metadata.TimeRunning)
}

// Read calls the given function on each available record in the ringbuffer
func (rb *ringBuffer) read(f func([]byte)) {
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

		f(data)

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

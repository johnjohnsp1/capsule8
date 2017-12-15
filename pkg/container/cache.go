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

package container

import "sync"

//
// This container cache maps container IDs to additional information
// about containers on the Node. The cache stores information about
// containers that have been created and the information is removed
// when the container is removed.
//

var (
	cache     map[string]*Info
	cacheLock sync.Mutex
	cacheOnce sync.Once
)

// Info describes a created, running or stopped container on the Node
type Info struct {
	ID        string
	Name      string
	ImageID   string
	ImageName string
}

func cacheUpdate(cID string, cName string, iID string, iName string) {
	// Initialize container cache if this is the first event
	cacheOnce.Do(func() {
		cache = make(map[string]*Info)
	})

	cacheLock.Lock()
	defer cacheLock.Unlock()
	_, ok := cache[cID]
	if !ok {
		i := &Info{
			ID:        cID,
			Name:      cName,
			ImageID:   iID,
			ImageName: iName,
		}

		cache[cID] = i
	}
}

func cacheDelete(containerID string) {
	// Initialize container cache if this is the first event
	cacheOnce.Do(func() {
		cache = make(map[string]*Info)
	})

	cacheLock.Lock()
	delete(cache, containerID)
	cacheLock.Unlock()
}

// GetInfo returns cached container information for the
// container with the given ID or nil if none was found.
func GetInfo(containerID string) *Info {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	return cache[containerID]
}

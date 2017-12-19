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

package sensor

import (
	"os"
	"testing"
)

/*

Current benchmark results:

BenchmarkArrayCache-8                   	500000000	         3.18 ns/op
BenchmarkMapCache-8                     	10000000	       152 ns/op
BenchmarkArrayCacheParallel-8           	2000000000	         1.56 ns/op
BenchmarkMapCacheParallel-8             	 5000000	       252 ns/op
BenchmarkStatCacheMiss-8                	  200000	     11180 ns/op
BenchmarkStatCacheMissParallel-8        	  500000	      3460 ns/op
BenchmarkContainerCacheMiss-8           	  100000	     14729 ns/op
BenchmarkContainerCacheMissParallel-8   	  300000	      5789 ns/op

*/

var values []task = []task{
	{1, 2, 3, 0x120011, "foo", nil, cred{}, "6e250051f33e0988aa6e549daa6c36de5ddf296bced4f31cf1b8249556f27ed2"},
	{1, 2, 3, 0x120011, "bar", nil, cred{}, "6e250051f33e0988aa6e549daa6c36de5ddf296bced4f31cf1b8249556f27ed2"},
	{1, 2, 3, 0x120011, "baz", nil, cred{}, "6e250051f33e0988aa6e549daa6c36de5ddf296bced4f31cf1b8249556f27ed2"},
	{1, 2, 3, 0x120011, "qux", nil, cred{}, "6e250051f33e0988aa6e549daa6c36de5ddf296bced4f31cf1b8249556f27ed2"},
}

func TestCaches(t *testing.T) {

	arrayCache := newArrayTaskCache()
	mapCache := newMapTaskCache()

	for i := 0; i < arrayTaskCacheSize; i++ {
		arrayCache.InsertTask(i, values[i%4])
		mapCache.InsertTask(i, values[i%4])
	}

	for i := arrayTaskCacheSize - 1; i >= 0; i-- {
		var tk task
		if arrayCache.LookupTask(i, &tk) {
			cid := tk.containerId

			if cid != values[i%4].containerId {
				t.Fatalf("Expected %s for pid %d, got %s",
					values[i%4].containerId, i, cid)
			}

		}

		if mapCache.LookupTask(i, &tk) {
			cid := tk.containerId

			if cid != values[i%4].containerId {
				t.Fatalf("Expected %s for pid %d, got %s",
					values[i%4].containerId, i, cid)
			}

		}
	}
}

func BenchmarkArrayCache(b *testing.B) {
	cache := newArrayTaskCache()
	var tk task

	for i := 0; i < b.N; i++ {
		cache.InsertTask((i % arrayTaskCacheSize), values[i%4])
	}

	for i := 0; i < b.N; i++ {
		_ = cache.LookupTask((i % arrayTaskCacheSize), &tk)
	}
}

func BenchmarkMapCache(b *testing.B) {
	cache := newMapTaskCache()
	var tk task

	for i := 0; i < b.N; i++ {
		cache.InsertTask((i % arrayTaskCacheSize), values[i%4])
	}

	for i := 0; i < b.N; i++ {
		_ = cache.LookupTask((i % arrayTaskCacheSize), &tk)
	}
}

func BenchmarkArrayCacheParallel(b *testing.B) {
	cache := newArrayTaskCache()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.InsertTask((i % arrayTaskCacheSize), values[i%4])
			i++
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		var tk task

		i := 0
		for pb.Next() {
			_ = cache.LookupTask((i % arrayTaskCacheSize), &tk)
			i++
		}
	})
}

func BenchmarkMapCacheParallel(b *testing.B) {
	cache := newMapTaskCache()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.InsertTask((i % arrayTaskCacheSize), values[i%4])
			i++
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		var tk task

		i := 0
		for pb.Next() {
			_ = cache.LookupTask((i % arrayTaskCacheSize), &tk)
			i++
		}
	})
}

func BenchmarkStatCacheMiss(b *testing.B) {
	pid := os.Getpid()

	for i := 0; i < b.N; i++ {
		_ = procFS.Stat(pid)
	}
}

func BenchmarkStatCacheMissParallel(b *testing.B) {
	pid := os.Getpid()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = procFS.Stat(pid)
		}
	})
}

func BenchmarkContainerCacheMiss(b *testing.B) {
	pid := os.Getpid()

	for i := 0; i < b.N; i++ {
		_, err := procFS.ContainerID(pid)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContainerCacheMissParallel(b *testing.B) {
	pid := os.Getpid()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := procFS.ContainerID(pid)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

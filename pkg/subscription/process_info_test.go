package subscription

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

var values = []struct {
	pid         int32
	containerID string
}{
	{1, "foo"},
	{2, "bar"},
	{3, "baz"},
	{4, "qux"},
}

func TestCaches(t *testing.T) {

	arrayCache := newArrayProcessInfoCache()
	mapCache := newMapProcessInfoCache()

	for i := int32(0); i < cacheArraySize; i++ {
		arrayCache.setContainerID(i, values[i%4].containerID)
		mapCache.setContainerID(i, values[i%4].containerID)
	}

	for i := int32(cacheArraySize) - 1; i >= 0; i-- {
		cid1 := arrayCache.containerID(i)
		cid2 := mapCache.containerID(i)

		if cid1 != values[i%4].containerID {
			t.Fatalf("Expected %s for pid %d, got %s",
				values[i%4].containerID, i, cid1)
		}

		if cid2 != values[i%4].containerID {
			t.Fatalf("Expected %s for pid %d, got %s",
				values[i%4].containerID, i, cid2)
		}
	}

}

func BenchmarkArrayCache(b *testing.B) {
	cache := newArrayProcessInfoCache()

	for i := int32(0); i < int32(b.N); i++ {
		cache.setContainerID((i % cacheArraySize),
			values[i%4].containerID)
	}

	for i := int32(0); i < int32(b.N); i++ {
		_ = cache.containerID((i % cacheArraySize))
	}
}

func BenchmarkMapCache(b *testing.B) {
	cache := newMapProcessInfoCache()

	for i := int32(0); i < int32(b.N); i++ {
		cache.setContainerID((i % cacheArraySize),
			values[i%4].containerID)
	}

	for i := int32(0); i < int32(b.N); i++ {
		_ = cache.containerID((i % cacheArraySize))
	}
}

func BenchmarkArrayCacheParallel(b *testing.B) {
	cache := newArrayProcessInfoCache()

	b.RunParallel(func(pb *testing.PB) {
		i := int32(0)
		for pb.Next() {
			cache.setContainerID((i % cacheArraySize),
				values[i%4].containerID)
			i++
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		i := int32(0)
		for pb.Next() {
			_ = cache.containerID((i % cacheArraySize))
			i++
		}
	})
}

func BenchmarkMapCacheParallel(b *testing.B) {
	cache := newMapProcessInfoCache()

	b.RunParallel(func(pb *testing.PB) {
		i := int32(0)
		for pb.Next() {
			cache.setContainerID((i % cacheArraySize),
				values[i%4].containerID)
			i++
		}
	})

	b.RunParallel(func(pb *testing.PB) {
		i := int32(0)
		for pb.Next() {
			_ = cache.containerID((i % cacheArraySize))
			i++
		}
	})
}

func BenchmarkStatCacheMiss(b *testing.B) {
	pid := os.Getpid()

	for i := 0; i < b.N; i++ {
		_ = procFS().Stat(int32(pid))
	}
}

func BenchmarkStatCacheMissParallel(b *testing.B) {
	pid := os.Getpid()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = procFS().Stat(int32(pid))
		}
	})
}

func BenchmarkContainerCacheMiss(b *testing.B) {
	pid := os.Getpid()

	for i := 0; i < b.N; i++ {
		_, err := procFS().ContainerID(int32(pid))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkContainerCacheMissParallel(b *testing.B) {
	pid := os.Getpid()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := procFS().ContainerID(int32(pid))
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

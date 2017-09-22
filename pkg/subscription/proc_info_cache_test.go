package subscription

import (
	"os"
	"testing"
)

func TestGetProcCacheEntryNew(t *testing.T) {
	pid := int32(os.Getpid())

	procEntry := getProcCacheEntry(pid)
	if procEntry == nil {
		t.Error("expected a new getProcCacheEntry to be created, got", procEntry)
	}
}

func BenchmarkNewProcCacheEntry(b *testing.B) {
	pid := int32(os.Getpid())

	for n := 0; n < b.N; n++ {
		newProcCacheEntry(pid)
	}
}

func BenchmarkGetProcCacheEntry(b *testing.B) {
	pid := int32(os.Getpid())
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			getProcCacheEntry(pid)
		}
	})
}

func BenchmarkGetProcCacheEntryA(b *testing.B) {
	pid := int32(os.Getpid())

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// One big lock to prevent duplication of effort by
			// multiple goroutines calling newProcCacheEntry(pid)
			mu.Lock()
			_, ok := pidMap[pid]

			if !ok {
				procEntry := newProcCacheEntry(pid)

				pidMap[pid] = procEntry

			}

			delete(pidMap, pid)

			mu.Unlock()
		}
	})
}

func BenchmarkGetProcCacheEntryB(b *testing.B) {
	pid := int32(os.Getpid())

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Finer-grained locking, allowing multiple goroutines
			// to call newProcCacheEntry(pid) and throwing one away.
			mu.Lock()
			_, ok := pidMap[pid]
			mu.Unlock()

			if !ok {
				procEntry := newProcCacheEntry(pid)

				mu.Lock()
				// Check if some other go routine has added an entry for this process.
				if currEntry, ok := pidMap[pid]; ok {
					procEntry = currEntry
				} else {
					pidMap[pid] = procEntry
				}

				delete(pidMap, pid)

				mu.Unlock()
			}
		}
	})
}

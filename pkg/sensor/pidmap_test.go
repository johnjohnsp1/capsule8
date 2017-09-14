package sensor

import (
	"os"
	"testing"
)

func BenchmarkReadStat(b *testing.B) {
	pid := int32(os.Getpid())

	for n := 0; n < b.N; n++ {
		readStat(pid)
	}
}

func BenchmarkReadCgroup(b *testing.B) {
	pid := int32(os.Getpid())

	for n := 0; n < b.N; n++ {
		readCgroup(pid)
	}
}

func BenchmarkGetCommandLine(b *testing.B) {
	pid := int32(os.Getpid())

	for n := 0; n < b.N; n++ {
		pidMapGetCommandLine(pid)
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

func BenchmarkGetProcCacheEntry1(b *testing.B) {
	pid := int32(os.Getpid())

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// One big lock to prevent duplication of effort by
			// multiple goroutines calling newProcCacheEntry(pid)
			mu.Lock()
			_, ok := pidMap[pid]

			if !ok {
				procEntry, err := newProcCacheEntry(pid)
				if err != nil {
					b.Fatal(err)
				}

				pidMap[pid] = procEntry

			}

			delete(pidMap, pid)

			mu.Unlock()
		}
	})
}

func BenchmarkGetProcCacheEntry2(b *testing.B) {
	pid := int32(os.Getpid())

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Finer-grained locking, allowing multiple goroutines
			// to call newProcCacheEntry(pid) and throwing one away.
			mu.Lock()
			_, ok := pidMap[pid]
			mu.Unlock()

			if !ok {
				procEntry, err := newProcCacheEntry(pid)
				if err != nil {
					b.Fatal(err)
				}

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

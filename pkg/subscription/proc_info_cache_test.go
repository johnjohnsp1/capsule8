package subscription

import (
	"crypto/sha256"
	"fmt"
	"hash/crc64"
	"os"
	"testing"
)

func TestGetProcCacheEntryNew(t *testing.T) {
	pid := int32(os.Getpid())

	procEntry, err := getProcCacheEntry(pid)
	if err != nil {
		t.Error("unexpected err trying to getProcCacheEntry:", err)
	}
	if procEntry == nil {
		t.Error("expected a new getProcCacheEntry to be created, got", procEntry)
	}
}

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
		procInfoGetCommandLine(pid)
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

func BenchmarkGetProcessIDCRC64(b *testing.B) {
	pid := int32(os.Getpid())
	hashTbl := crc64.MakeTable(crc64.ISO)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bId, err := getBootId()
			if err != nil {
				b.Fatal(err)
			}

			// Compute the raw process ID from process info
			stat, err := readStat(pid)
			if err != nil {
				b.Fatal(err)
			}

			rawId := fmt.Sprintf("%s/%s/%s", bId, stat[STAT_FIELD_PID], stat[STAT_FIELD_STARTTIME])

			// Now hash the raw ID

			hasher := crc64.New(hashTbl)
			hasher.Write([]byte(rawId))
			hashedId := hasher.Sum([]byte{})
			_ = fmt.Sprintf("%X", hashedId)
		}
	})
}

func BenchmarkGetProcessIDSHA256(b *testing.B) {
	pid := int32(os.Getpid())

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bId, err := getBootId()
			if err != nil {
				b.Fatal(err)
			}

			// Compute the raw process ID from process info
			stat, err := readStat(pid)
			if err != nil {
				b.Fatal(err)
			}

			rawId := fmt.Sprintf("%s/%s/%s", bId, stat[STAT_FIELD_PID], stat[STAT_FIELD_STARTTIME])

			// Now hash the raw ID
			hasher := sha256.New()
			hasher.Write([]byte(rawId))
			hashedId := hasher.Sum([]byte{})
			_ = fmt.Sprintf("%X", hashedId)
		}
	})
}

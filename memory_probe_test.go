package qpbenchmark

import (
	"os"
	"runtime"
	"strconv"
	"testing"
)

// Run each backend in its own test process so retained heap deltas are isolated.
func TestRetainedMemory(t *testing.T) {
	name := os.Getenv("MEM_BACKEND")
	if name == "" {
		t.Skip("set MEM_BACKEND to qp, map, btree, or radix")
	}
	valid := false
	for _, backend := range backends {
		valid = valid || name == backend
	}
	if !valid {
		t.Fatalf("invalid MEM_BACKEND %q", name)
	}
	n := 100000
	if raw := os.Getenv("BENCH_N"); raw != "" {
		var err error
		n, err = strconv.Atoi(raw)
		if err != nil || n < 16 {
			t.Fatal("BENCH_N must be an integer >= 16")
		}
	}
	shape := os.Getenv("BENCH_SHAPE")
	if shape == "" {
		shape = "random16"
	}
	c := makeCorpus(shape, n)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	s := build(name, c.insert)
	if s.len() != n {
		t.Fatal("wrong build size")
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	bytes := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	objects := int64(after.HeapObjects) - int64(before.HeapObjects)
	t.Logf("backend=%s shape=%s keys=%d retained_heap_bytes=%d bytes/key=%.2f retained_objects=%d objects/key=%.3f",
		name, shape, n, bytes, float64(bytes)/float64(n), objects, float64(objects)/float64(n))
	if os.Getenv("MEM_SNAPSHOT") == "1" {
		before = after
		next := s.snapshotUpdate(c.hits[:16], 2)
		if next.len() != n {
			t.Fatal("snapshot changed size")
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		bytes = int64(after.HeapAlloc) - int64(before.HeapAlloc)
		objects = int64(after.HeapObjects) - int64(before.HeapObjects)
		t.Logf("backend=%s shape=%s snapshot_update_keys=16 added_heap_bytes=%d added_objects=%d",
			name, shape, bytes, objects)
		runtime.KeepAlive(next)
	}
	runtime.KeepAlive(s)
	runtime.KeepAlive(c)
}

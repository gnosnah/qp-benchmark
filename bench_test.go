package qpbenchmark

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"os"
	"runtime"
	"strconv"
	"testing"
)

type corpus struct {
	insert [][]byte
	hits   [][]byte
	misses [][]byte
}

func makeCorpus(shape string, n int) corpus {
	if n < 16 {
		panic("BENCH_N must be at least 16")
	}
	rng := rand.New(rand.NewPCG(42, 17))
	c := corpus{insert: make([][]byte, n), hits: make([][]byte, n), misses: make([][]byte, n)}
	for i := range c.insert {
		var k []byte
		switch shape {
		case "random16":
			k = make([]byte, 16)
			binary.BigEndian.PutUint64(k[:8], rng.Uint64())
			binary.BigEndian.PutUint64(k[8:], uint64(i))
		case "prefix64":
			k = make([]byte, 64)
			for j := range k[:56] {
				k[j] = 'a'
			}
			binary.BigEndian.PutUint64(k[56:], uint64(i))
		case "domain":
			k = []byte(fmt.Sprintf("host-%08d.service-%03d.example.org", i, i%97))
		default:
			panic("BENCH_SHAPE must be random16, prefix64, or domain")
		}
		c.insert[i] = k
		c.hits[i] = append([]byte(nil), k...)
		c.misses[i] = append(append([]byte(nil), k...), 0)
	}
	for i := n - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		c.insert[i], c.insert[j] = c.insert[j], c.insert[i]
	}
	for i := n - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		c.hits[i], c.hits[j] = c.hits[j], c.hits[i]
	}
	for i := n - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		c.misses[i], c.misses[j] = c.misses[j], c.misses[i]
	}
	return c
}

func benchmarkConfig(b *testing.B) (string, corpus) {
	b.Helper()
	n := 10000
	if s := os.Getenv("BENCH_N"); s != "" {
		var err error
		n, err = strconv.Atoi(s)
		if err != nil || n < 16 {
			b.Fatal("BENCH_N must be an integer >= 16")
		}
	}
	shape := os.Getenv("BENCH_SHAPE")
	if shape == "" {
		shape = "random16"
	}
	return shape, makeCorpus(shape, n)
}

var resultStore store
var resultValue uint64

func BenchmarkBuild(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		b.Run(fmt.Sprintf("%s/%s/n=%d", name, shape, len(c.insert)), func(b *testing.B) {
			b.ReportAllocs()
			var s store
			for b.Loop() {
				s = build(name, c.insert)
			}
			if s.len() != len(c.insert) {
				b.Fatal("wrong build size")
			}
			resultStore = s
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(c.insert)), "ns/key")
		})
	}
}

func BenchmarkGet(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		for _, hit := range []bool{true, false} {
			label := "miss"
			queries := c.misses
			if hit {
				label, queries = "hit", c.hits
			}
			b.Run(fmt.Sprintf("%s/%s/n=%d/%s", name, shape, len(c.insert), label), func(b *testing.B) {
				s := build(name, c.insert)
				b.ReportAllocs()
				var sum uint64
				var found int
				i := 0
				for b.Loop() {
					v, ok := s.get(queries[i])
					sum += v
					if ok {
						found++
					}
					i++
					if i == len(queries) {
						i = 0
					}
				}
				want := 0
				if hit {
					want = b.N
				}
				if found != want || sum != uint64(want) {
					b.Fatalf("lookup mismatch: found=%d sum=%d want=%d", found, sum, want)
				}
				runtime.KeepAlive(s)
			})
		}
	}
}

func BenchmarkUpdate(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		b.Run(fmt.Sprintf("%s/%s/n=%d", name, shape, len(c.insert)), func(b *testing.B) {
			s := build(name, c.insert)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				s.put(c.hits[i], uint64(i+2))
				i++
				if i == len(c.hits) {
					i = 0
				}
			}
			if s.len() != len(c.insert) {
				b.Fatal("update changed size")
			}
			resultStore = s
		})
	}
}

func BenchmarkDelete(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		b.Run(fmt.Sprintf("%s/%s/n=%d", name, shape, len(c.insert)), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				s := build(name, c.insert)
				b.StartTimer()
				for _, key := range c.hits {
					s.delete(key)
				}
				b.StopTimer()
				if s.len() != 0 {
					b.Fatal("delete left keys")
				}
				resultStore = s
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(c.insert)), "ns/key")
		})
	}
}

func BenchmarkScan(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		b.Run(fmt.Sprintf("%s/%s/n=%d", name, shape, len(c.insert)), func(b *testing.B) {
			s := build(name, c.insert)
			b.ReportAllocs()
			var count int
			var sum uint64
			for b.Loop() {
				count, sum = s.scan()
			}
			if count != len(c.insert) || sum != uint64(len(c.insert)) {
				b.Fatal("scan mismatch")
			}
			resultValue = sum
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(c.insert)), "ns/key")
		})
	}
}

func BenchmarkSnapshotUpdate16(b *testing.B) {
	shape, c := benchmarkConfig(b)
	for _, name := range backends {
		b.Run(fmt.Sprintf("%s/%s/n=%d", name, shape, len(c.insert)), func(b *testing.B) {
			base := build(name, c.insert)
			keys := c.hits[:16]
			b.ReportAllocs()
			var next store
			for b.Loop() {
				next = base.snapshotUpdate(keys, 2)
			}
			if next.len() != len(c.insert) || base.len() != len(c.insert) {
				b.Fatal("snapshot size mismatch")
			}
			if v, _ := base.get(keys[0]); v != 1 {
				b.Fatal("base snapshot changed")
			}
			if v, _ := next.get(keys[0]); v != 2 {
				b.Fatal("new snapshot not updated")
			}
			resultStore = next
		})
	}
}

# qp benchmark

Independent Go benchmarks of
[`github.com/gnosnah/qp@v1.0.0`](https://github.com/gnosnah/qp) against
`map[string]uint64`, [Google B-tree](https://github.com/google/btree), and
[HashiCorp immutable radix](https://github.com/hashicorp/go-immutable-radix).

## Headline results

At N=10,000 random 16-byte keys (median ns; lower is faster):

| | qp | map | B-tree | radix |
|---|---:|---:|---:|---:|
| **Build** | 359 ns/key | **100** ns/key | 519 ns/key | 1,019 ns/key |
| **Get hit** | 76 ns | **27** ns | 378 ns | 261 ns |
| **Ordered scan** | 50 ns/key | 240 ns/key | **5.6** ns/key | 12.6 ns/key |
| **Snapshot 16 keys** | **19 µs** | 302 µs | 27 µs | 41 µs |

Retained heap after GC at N=100,000, and cost of one 16-key COW snapshot on
that base:

| | qp | map | B-tree | radix |
|---|---:|---:|---:|---:|
| Retained (random16) | 102 B/key | **51** B/key | 60 B/key | 421 B/key |
| Retained (prefix64) | 131 B/key | 99 B/key | **60** B/key | 356 B/key |
| Snapshot +16 keys | **14 KiB** | 3.3 MiB | 44 KiB | 70 KiB |

**Where qp lands**

- **Lookups:** ~3× map, ~3–5× faster than B-tree/radix; Get/Update allocate
  nothing.
- **Versioned writes:** cheapest small snapshot by a wide margin (COW vs full
  `maps.Clone`).
- **Scan:** slower than B-tree/radix; much faster than map (which re-sorts).
- **Memory:** ~2× map on random16 because qp owns copied keys and per-key
  leaves; still ~4× leaner than immutable radix. B-tree/radix Build numbers
  exclude key storage they borrow.

Full medians (all shapes / ops): [RESULTS.md](RESULTS.md).  
Profiles and optimization notes: [ANALYSIS.md](ANALYSIS.md).

Short exploratory runs — directional, not statistical proof.

## Run

Requires Go 1.24+.

```sh
go test ./...
BENCH_N=10000 BENCH_SHAPE=random16 go test -run '^$' -bench . -benchmem -benchtime=500ms -count=5 -cpu=1
MEM_BACKEND=qp BENCH_N=100000 BENCH_SHAPE=random16 go test -run '^TestRetainedMemory$' -count=1 -v
```

`BENCH_SHAPE`: `random16` (default), `prefix64`, `domain`. Workloads: Build,
Get hit/miss, Update, Delete, Scan, SnapshotUpdate16. Compare allocation and
write results with ownership in mind (qp/map copy keys; B-tree/radix borrow;
radix Update/Delete are immutable per call).

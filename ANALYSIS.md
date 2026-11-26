# QP performance and memory analysis

Deep dive for `github.com/gnosnah/qp@v1.0.0`. Headline numbers and takeaways:
[README.md](README.md). Full timing tables: [RESULTS.md](RESULTS.md).

## Allocations and profiles

qp reports zero allocations on Get and in-place Update. Build is about 114 B and
3.03 allocs per inserted 16-byte key. Full scan allocates once per iterator
(~2,304 B) — transient, not retained.

[Build CPU](profiles/qp-build.cpu), [build alloc](profiles/qp-build.mem), and
[Get CPU](profiles/qp-get.cpu) profiles (`pprof -top` / `-list`):

- **Build:** `newLeafNode` 41.6% of allocated bytes, `bytes.Clone` 12.8%,
  `growTwigs` 24.3%, `newBranchNode` 20.5%. The `append` in `growTwigs` is all
  of that function's sampled alloc and 11.2% cumulative build CPU at its line.
- **Get** (hit+miss, 64-byte common-prefix keys): `findMatch` 50.9% cumulative;
  node-type switch 14.9% flat; `nibbleBit` 16.9% flat; `twigOffset` 8.1%;
  `bytes.Equal` 31.2% cumulative. Cumulative shares overlap — do not add them.

## Retained heap

[memory_probe_test.go](memory_probe_test.go) builds N=100,000 in a separate
process and measures `HeapAlloc` after GC with the corpus still live:

| Key shape | qp B/key | map B/key | B-tree B/key | radix B/key |
|---|---:|---:|---:|---:|
| random16 | 101.86 | 50.95 | 59.72 | 421.39 |
| prefix64 | 131.20 | 98.95 | 59.73 | 355.80 |

qp: 2.749 objects/key (random16), 2.133 (prefix64). Map: ~1.003. B-tree/radix
borrow input key buffers; qp/map own copies — heap deltas are not apples-to-
apples on key storage. One 16-key snapshot on a live 100k random16 base:
+14,256 B (qp), +3,496,368 B (map), +44,592 B (B-tree), +71,584 B (radix).

## Optimization candidates

1. **Branch child-slice growth.** `growTwigs` grows one slot at a time (24.3%
   of sampled build alloc). A small initial capacity or different growth policy
   may cut realloc/copy; watch retained B/key for unused capacity.
2. **Avoid second descent on insert.** `Upsert` does `findMatch` then
   `findInsert` from the root again (`findInsert` ~8.6% cumulative build CPU on
   random16). A saved path could help; prove on the mutable path before Txn.
3. **Right-size iterator stack.** `Iterator()` allocates 256 pointer slots even
   for shallow tries (~2,304 B/scan). Grow on demand; re-check deep-prefix walks.
4. **Node / child-slice overhead.** Per-key leaf + cloned bytes, plus branch
   objects, drive ~2.75 objects/key. Compact or pooled storage is the main
   retained-memory lever; avoid a fixed 17-wide inline array without evidence.
5. **COW is already strong.** First write to a shared branch copies the child
   slice; qp still had the smallest snapshot time and heap delta here. Any
   change must keep old-version readers correct.

Point-query cost sits in node dispatch, nibble/bitmap indexing, and full-key
compare. A more compact node layout might help locality and memory together,
but needs a concrete prototype before trading away snapshot or key-ownership
guarantees.

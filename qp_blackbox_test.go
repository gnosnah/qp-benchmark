package qpbenchmark_test

import (
	"bytes"
	"math/rand"
	"slices"
	"strconv"
	"testing"

	"github.com/gnosnah/qp"
)

func modelKeys(m map[string]uint64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func checkTrie(t *testing.T, tr *qp.Trie[uint64], model map[string]uint64, queries [][]byte) {
	t.Helper()
	keys := modelKeys(model)
	if got := tr.Size(); got != len(keys) {
		t.Fatalf("Size() = %d, want %d", got, len(keys))
	}

	it := tr.Iterator()
	for _, want := range keys {
		key, value, ok := it.Next()
		if !ok || string(key) != want || value != model[want] {
			t.Fatalf("Iterator.Next() = (%x, %d, %v), want (%x, %d, true)", key, value, ok, want, model[want])
		}
	}
	if key, value, ok := it.Next(); ok || key != nil || value != 0 {
		t.Fatalf("Iterator.Next() after end = (%x, %d, %v)", key, value, ok)
	}

	walked := tr.Walk(len(keys)+1, nil)
	if len(walked) != len(keys) {
		t.Fatalf("Walk length = %d, want %d", len(walked), len(keys))
	}
	for i, pair := range walked {
		if string(pair.Key) != keys[i] || pair.Value != model[keys[i]] {
			t.Fatalf("Walk[%d] = (%x, %d), want (%x, %d)", i, pair.Key, pair.Value, keys[i], model[keys[i]])
		}
	}
	var filtered []string
	for _, key := range keys {
		if key[0]&1 == 0 {
			filtered = append(filtered, key)
		}
	}
	limit := min(3, len(filtered))
	selected := tr.Walk(limit, func(key []byte, _ uint64) bool { return key[0]&1 == 0 })
	if len(selected) != limit {
		t.Fatalf("filtered Walk length = %d, want %d", len(selected), limit)
	}
	for i, pair := range selected {
		if string(pair.Key) != filtered[i] || pair.Value != model[filtered[i]] {
			t.Fatalf("filtered Walk[%d] = (%x, %d), want (%x, %d)",
				i, pair.Key, pair.Value, filtered[i], model[filtered[i]])
		}
	}

	for _, query := range queries {
		got, found := tr.Get(query)
		want, wantFound := model[string(query)]
		if found != wantFound || got != want {
			t.Fatalf("Get(%x) = (%d, %v), want (%d, %v)", query, got, found, want, wantFound)
		}
		idx, exact := slices.BinarySearch(keys, string(query))
		if !exact {
			idx--
		}
		key, value, isExact, ok := tr.GetLessOrEqual(query)
		if idx < 0 {
			if ok || isExact || key != nil || value != 0 {
				t.Fatalf("GetLessOrEqual(%x) = (%x, %d, %v, %v), want miss", query, key, value, isExact, ok)
			}
		} else if !ok || isExact != exact || string(key) != keys[idx] || value != model[keys[idx]] {
			t.Fatalf("GetLessOrEqual(%x) = (%x, %d, %v, %v), want (%x, %d, %v, true)",
				query, key, value, isExact, ok, keys[idx], model[keys[idx]], exact)
		}
	}
}

func randomKey(r *rand.Rand) []byte {
	alphabet := [...]byte{0, 1, 15, 16, 'a', 'b', 240, 255}
	key := make([]byte, 1+r.Intn(5))
	for i := range key {
		key[i] = alphabet[r.Intn(len(alphabet))]
	}
	return key
}

func TestQPBlackBoxModel(t *testing.T) {
	for _, seed := range []int64{1, 17, 2026} {
		t.Run("seed="+strconv.FormatInt(seed, 10), func(t *testing.T) {
			r := rand.New(rand.NewSource(seed))
			tr := qp.New[uint64]()
			model := make(map[string]uint64)
			queries := [][]byte{{0}, {1}, {'a'}, {'a', 0}, {'a', 255}, {255, 255}}
			for step := 0; step < 2000; step++ {
				key := randomKey(r)
				want, exists := model[string(key)]
				switch r.Intn(4) {
				case 0, 1:
					value := uint64(r.Intn(10)) // Include stored zero values.
					old, updated := tr.Upsert(key, value)
					if old != want || updated != exists {
						t.Fatalf("step %d: Upsert(%x) = (%d, %v), want (%d, %v)", step, key, old, updated, want, exists)
					}
					model[string(key)] = value
					key[0] ^= 0xff // An inserted key must be owned by the trie.
				case 2:
					old, found := tr.Delete(key)
					if old != want || found != exists {
						t.Fatalf("step %d: Delete(%x) = (%d, %v), want (%d, %v)", step, key, old, found, want, exists)
					}
					delete(model, string(key))
				case 3:
					got, found := tr.Get(key)
					if got != want || found != exists {
						t.Fatalf("step %d: Get(%x) = (%d, %v), want (%d, %v)", step, key, got, found, want, exists)
					}
				}
				if step%50 == 0 {
					checkTrie(t, tr, model, append(queries, randomKey(r)))
				}
			}
			checkTrie(t, tr, model, queries)
		})
	}
}

func TestQPBlackBoxTransaction(t *testing.T) {
	base := qp.New[uint64]()
	for key, value := range map[string]uint64{"a": 0, "ab": 1, "b": 2, "\xff": 3} {
		base.Upsert([]byte(key), value)
	}
	baseModel := map[string]uint64{"a": 0, "ab": 1, "b": 2, "\xff": 3}
	queries := [][]byte{[]byte("a"), []byte("aa"), []byte("ab"), []byte("b"), []byte("c"), {255}}

	tx := base.Txn()
	if old, updated := tx.Upsert([]byte("a"), 10); !updated || old != 0 {
		t.Fatalf("transaction update = (%d, %v)", old, updated)
	}
	if old, updated := tx.Upsert([]byte("c"), 4); updated || old != 0 {
		t.Fatalf("transaction insert = (%d, %v)", old, updated)
	}
	if old, found := tx.Delete([]byte("ab")); !found || old != 1 {
		t.Fatalf("transaction delete = (%d, %v)", old, found)
	}
	checkTrie(t, base, baseModel, queries)
	committed := tx.Commit()
	expectPanic(t, func() { tx.Get([]byte("a")) })
	committedModel := map[string]uint64{"a": 10, "b": 2, "c": 4, "\xff": 3}
	checkTrie(t, committed, committedModel, queries)
	checkTrie(t, base, baseModel, queries)

	abort := committed.Txn()
	abort.Upsert([]byte("a"), 99)
	abort.Delete([]byte("b"))
	aborted := abort.Abort()
	expectPanic(t, func() { abort.Upsert([]byte("a"), 3) })
	checkTrie(t, aborted, committedModel, queries)
	checkTrie(t, base, baseModel, queries)

	query := []byte("d")
	key, _, _, found := committed.GetLessOrEqual(query)
	if !found || !bytes.Equal(key, []byte("c")) {
		t.Fatalf("predecessor before query reuse = %x, %v", key, found)
	}
	query[0] = 'a'
	if !bytes.Equal(key, []byte("c")) {
		t.Fatalf("returned predecessor aliases query: %x", key)
	}
}

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestQPBlackBoxKeyValidation(t *testing.T) {
	tr := qp.New[uint64]()
	for _, key := range [][]byte{nil, {}, bytes.Repeat([]byte{'x'}, 32768)} {
		expectPanic(t, func() { tr.Upsert(key, 1) })
		expectPanic(t, func() { tr.Get(key) })
		expectPanic(t, func() { tr.Delete(key) })
		expectPanic(t, func() { tr.GetLessOrEqual(key) })
	}
	if tr.Size() != 0 {
		t.Fatal("invalid keys changed trie")
	}
}

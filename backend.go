package qpbenchmark

import (
	"bytes"
	"maps"
	"slices"

	"github.com/gnosnah/qp"
	"github.com/google/btree"
	iradix "github.com/hashicorp/go-immutable-radix/v2"
)

type store interface {
	get([]byte) (uint64, bool)
	put([]byte, uint64)
	delete([]byte)
	len() int
	scan() (int, uint64)
	snapshotUpdate([][]byte, uint64) store
}

var backends = []string{"qp", "map", "btree", "radix"}

func newStore(name string, capacity int) store {
	switch name {
	case "qp":
		return &qpStore{qp.New[uint64]()}
	case "map":
		return &mapStore{make(map[string]uint64, capacity)}
	case "btree":
		return &btreeStore{btree.New(32)}
	case "radix":
		return &radixStore{iradix.New[uint64]()}
	default:
		panic("unknown backend: " + name)
	}
}

func build(name string, keys [][]byte) store {
	s := newStore(name, len(keys))
	if r, ok := s.(*radixStore); ok {
		tx := r.tree.Txn()
		for _, key := range keys {
			tx.Insert(key, 1)
		}
		r.tree = tx.Commit()
		return r
	}
	for _, key := range keys {
		s.put(key, 1)
	}
	return s
}

type qpStore struct{ tree *qp.Trie[uint64] }

func (s *qpStore) get(k []byte) (uint64, bool) { return s.tree.Get(k) }
func (s *qpStore) put(k []byte, v uint64)      { s.tree.Upsert(k, v) }
func (s *qpStore) delete(k []byte)             { s.tree.Delete(k) }
func (s *qpStore) len() int                    { return s.tree.Size() }
func (s *qpStore) scan() (count int, sum uint64) {
	it := s.tree.Iterator()
	for {
		_, v, ok := it.Next()
		if !ok {
			return
		}
		count++
		sum += v
	}
}
func (s *qpStore) snapshotUpdate(keys [][]byte, v uint64) store {
	tx := s.tree.Txn()
	for _, key := range keys {
		tx.Upsert(key, v)
	}
	return &qpStore{tx.Commit()}
}

type mapStore struct{ data map[string]uint64 }

func (s *mapStore) get(k []byte) (uint64, bool) { v, ok := s.data[string(k)]; return v, ok }
func (s *mapStore) put(k []byte, v uint64)      { s.data[string(k)] = v }
func (s *mapStore) delete(k []byte)             { delete(s.data, string(k)) }
func (s *mapStore) len() int                    { return len(s.data) }
func (s *mapStore) scan() (count int, sum uint64) {
	keys := make([]string, 0, len(s.data))
	for key := range s.data {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		sum += s.data[key]
	}
	return len(keys), sum
}
func (s *mapStore) snapshotUpdate(keys [][]byte, v uint64) store {
	next := &mapStore{maps.Clone(s.data)}
	for _, key := range keys {
		next.put(key, v)
	}
	return next
}

type item struct {
	key []byte
	val uint64
}

func (x *item) Less(other btree.Item) bool { return bytes.Compare(x.key, other.(*item).key) < 0 }

type btreeStore struct{ tree *btree.BTree }

func (s *btreeStore) get(k []byte) (uint64, bool) {
	x := s.tree.Get(&item{key: k})
	if x == nil {
		return 0, false
	}
	return x.(*item).val, true
}
func (s *btreeStore) put(k []byte, v uint64) { s.tree.ReplaceOrInsert(&item{key: k, val: v}) }
func (s *btreeStore) delete(k []byte)        { s.tree.Delete(&item{key: k}) }
func (s *btreeStore) len() int               { return s.tree.Len() }
func (s *btreeStore) scan() (count int, sum uint64) {
	s.tree.Ascend(func(x btree.Item) bool {
		count++
		sum += x.(*item).val
		return true
	})
	return
}
func (s *btreeStore) snapshotUpdate(keys [][]byte, v uint64) store {
	next := &btreeStore{s.tree.Clone()}
	for _, key := range keys {
		next.put(key, v)
	}
	return next
}

type radixStore struct{ tree *iradix.Tree[uint64] }

func (s *radixStore) get(k []byte) (uint64, bool) { return s.tree.Get(k) }
func (s *radixStore) put(k []byte, v uint64) {
	s.tree, _, _ = s.tree.Insert(k, v)
}
func (s *radixStore) delete(k []byte) { s.tree, _, _ = s.tree.Delete(k) }
func (s *radixStore) len() int        { return s.tree.Len() }
func (s *radixStore) scan() (count int, sum uint64) {
	s.tree.Root().Walk(func(_ []byte, v uint64) bool {
		count++
		sum += v
		return false
	})
	return
}
func (s *radixStore) snapshotUpdate(keys [][]byte, v uint64) store {
	tx := s.tree.Txn()
	for _, key := range keys {
		tx.Insert(key, v)
	}
	return &radixStore{tx.Commit()}
}

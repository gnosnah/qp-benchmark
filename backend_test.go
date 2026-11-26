package qpbenchmark

import "testing"

func TestBackends(t *testing.T) {
	for _, shape := range []string{"random16", "prefix64", "domain"} {
		c := makeCorpus(shape, 129)
		for _, name := range backends {
			t.Run(shape+"/"+name, func(t *testing.T) {
				s := build(name, c.insert)
				if s.len() != 129 {
					t.Fatal("build size")
				}
				for i := range c.hits {
					if v, ok := s.get(c.hits[i]); !ok || v != 1 {
						t.Fatal("hit")
					}
					if _, ok := s.get(c.misses[i]); ok {
						t.Fatal("miss")
					}
				}
				if n, sum := s.scan(); n != 129 || sum != 129 {
					t.Fatal("scan")
				}
				next := s.snapshotUpdate(c.hits[:16], 2)
				for _, k := range c.hits[:16] {
					if v, _ := s.get(k); v != 1 {
						t.Fatal("base changed")
					}
					if v, _ := next.get(k); v != 2 {
						t.Fatal("snapshot update")
					}
				}
				deleting := build(name, c.insert)
				for _, k := range c.hits {
					deleting.delete(k)
				}
				if deleting.len() != 0 || next.len() != 129 {
					t.Fatal("delete or snapshot isolation")
				}
			})
		}
	}
}

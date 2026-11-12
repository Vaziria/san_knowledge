package san_btree

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"testing"
	"unsafe"

	bolt "go.etcd.io/bbolt"
)

// Benchmarks against bbolt (go.etcd.io/bbolt, used by the tower), with real
// fsync on both: Put (one commit per key), Batch (1000 keys per commit),
// Get, View (zero copy) and Range (1000 keys from a random start).
//
//	go test -run '^$' -bench . -benchtime 2s

type benchValue struct {
	A, B int64
	C    float64
}

const benchKeys = 100_000

func benchKey(i int) string { return fmt.Sprintf("key%08d", i) }

func valueBytes(v *benchValue) []byte {
	return unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(*v))
}

var bucket = []byte("b")

func openBolt(b *testing.B) *bolt.DB {
	db, err := bolt.Open(filepath.Join(b.TempDir(), "bolt.db"), 0o600, nil)
	if err != nil {
		b.Fatal(err)
	}
	db.Update(func(tx *bolt.Tx) error { _, err := tx.CreateBucketIfNotExists(bucket); return err })
	b.Cleanup(func() { db.Close() })
	return db
}

func openSan(b *testing.B) *Index[string, benchValue] {
	ix, err := NewIndex[string, benchValue](filepath.Join(b.TempDir(), "kw"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { ix.Close() })
	return ix
}

func fillBolt(b *testing.B, db *bolt.DB) {
	for i := 0; i < benchKeys; i += 10_000 {
		db.Update(func(tx *bolt.Tx) error {
			bk := tx.Bucket(bucket)
			for j := i; j < i+10_000; j++ {
				v := benchValue{A: int64(j)}
				bk.Put([]byte(benchKey(j)), append([]byte(nil), valueBytes(&v)...))
			}
			return nil
		})
	}
}

func fillSan(b *testing.B, ix *Index[string, benchValue]) {
	for i := 0; i < benchKeys; i += 10_000 {
		ix.Batch(func(bt *Batch[string, benchValue]) error {
			for j := i; j < i+10_000; j++ {
				bt.Put(benchKey(j), benchValue{A: int64(j)})
			}
			return nil
		})
	}
}

func BenchmarkPut(b *testing.B) {
	b.Run("san_btree", func(b *testing.B) {
		ix := openSan(b)
		for i := 0; b.Loop(); i++ {
			if err := ix.Put(benchKey(i), benchValue{A: int64(i)}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("bbolt", func(b *testing.B) {
		db := openBolt(b)
		for i := 0; b.Loop(); i++ {
			v := benchValue{A: int64(i)}
			err := db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucket).Put([]byte(benchKey(i)), valueBytes(&v)) })
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBatch1000(b *testing.B) {
	b.Run("san_btree", func(b *testing.B) {
		ix := openSan(b)
		r := rand.New(rand.NewPCG(1, 2))
		for b.Loop() {
			ix.Batch(func(bt *Batch[string, benchValue]) error {
				for range 1000 {
					j := r.IntN(10 * benchKeys)
					bt.Put(benchKey(j), benchValue{A: int64(j)})
				}
				return nil
			})
		}
	})
	b.Run("bbolt", func(b *testing.B) {
		db := openBolt(b)
		r := rand.New(rand.NewPCG(1, 2))
		for b.Loop() {
			db.Update(func(tx *bolt.Tx) error {
				bk := tx.Bucket(bucket)
				for range 1000 {
					j := r.IntN(10 * benchKeys)
					v := benchValue{A: int64(j)}
					bk.Put([]byte(benchKey(j)), append([]byte(nil), valueBytes(&v)...))
				}
				return nil
			})
		}
	})
}

func BenchmarkGet(b *testing.B) {
	b.Run("san_btree", func(b *testing.B) {
		ix := openSan(b)
		fillSan(b, ix)
		r := rand.New(rand.NewPCG(3, 4))
		for b.Loop() {
			j := r.IntN(benchKeys)
			v, ok, err := ix.Get(benchKey(j))
			if !ok || err != nil || v.A != int64(j) {
				b.Fatal("miss")
			}
		}
	})
	b.Run("bbolt", func(b *testing.B) {
		db := openBolt(b)
		fillBolt(b, db)
		r := rand.New(rand.NewPCG(3, 4))
		for b.Loop() {
			j := r.IntN(benchKeys)
			var v benchValue
			db.View(func(tx *bolt.Tx) error {
				copy(valueBytes(&v), tx.Bucket(bucket).Get([]byte(benchKey(j))))
				return nil
			})
			if v.A != int64(j) {
				b.Fatal("miss")
			}
		}
	})
}

func BenchmarkView(b *testing.B) {
	b.Run("san_btree", func(b *testing.B) {
		ix := openSan(b)
		fillSan(b, ix)
		r := rand.New(rand.NewPCG(3, 4))
		var sum int64
		for b.Loop() {
			ix.View(benchKey(r.IntN(benchKeys)), func(v *benchValue) { sum += v.A })
		}
	})
	b.Run("bbolt", func(b *testing.B) {
		db := openBolt(b)
		fillBolt(b, db)
		r := rand.New(rand.NewPCG(3, 4))
		var sum int64
		for b.Loop() {
			db.View(func(tx *bolt.Tx) error {
				v := tx.Bucket(bucket).Get([]byte(benchKey(r.IntN(benchKeys))))
				sum += (*benchValue)(unsafe.Pointer(&v[0])).A
				return nil
			})
		}
	})
}

func BenchmarkRange1000(b *testing.B) {
	b.Run("san_btree", func(b *testing.B) {
		ix := openSan(b)
		fillSan(b, ix)
		r := rand.New(rand.NewPCG(5, 6))
		for b.Loop() {
			j := r.IntN(benchKeys - 1000)
			n := 0
			for range ix.Range(benchKey(j), benchKey(j+1000)) {
				n++
			}
			if n != 1000 {
				b.Fatal(n)
			}
		}
	})
	b.Run("bbolt", func(b *testing.B) {
		db := openBolt(b)
		fillBolt(b, db)
		r := rand.New(rand.NewPCG(5, 6))
		for b.Loop() {
			j := r.IntN(benchKeys - 1000)
			n := 0
			db.View(func(tx *bolt.Tx) error {
				c := tx.Bucket(bucket).Cursor()
				end := []byte(benchKey(j + 1000))
				for k, v := c.Seek([]byte(benchKey(j))); k != nil && string(k) < string(end); k, v = c.Next() {
					var bv benchValue
					copy(valueBytes(&bv), v)
					_ = string(k)
					n++
				}
				return nil
			})
			if n != 1000 {
				b.Fatal(n)
			}
		}
	})
}

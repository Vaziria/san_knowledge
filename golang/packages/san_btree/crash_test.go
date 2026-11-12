package san_btree

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

type crashCase struct {
	name string
	op   func(ix *Index[string, Keyword]) error
}

func crashCases() []crashCase {
	kw := func(i int, n int) Keyword {
		return Keyword{Name: strings.Repeat("n", n), Interest: int64(i), Tags: [2]string{fmt.Sprint(i), "t"}}
	}
	return []crashCase{
		{"put new key", func(ix *Index[string, Keyword]) error { return ix.Put("k0100x", kw(1, 50)) }},
		{"update", func(ix *Index[string, Keyword]) error { return ix.Put("k0100", kw(2, 70)) }},
		{"update fn", func(ix *Index[string, Keyword]) error {
			return ix.Update("k0101", func(v *Keyword) error { v.Interest += 5; return nil })
		}},
		{"delete", func(ix *Index[string, Keyword]) error { _, err := ix.Delete("k0102"); return err }},
		{"batch: new data file and index growth", func(ix *Index[string, Keyword]) error {
			return ix.Batch(func(b *Batch[string, Keyword]) error {
				for i := range 400 {
					b.Put(fmt.Sprintf("b%04d%s", i, strings.Repeat("x", 200)), kw(i, 120))
				}
				for i := range 50 {
					b.Delete(fmt.Sprintf("k%04d", i*3))
				}
				return nil
			})
		}},
		{"batch: automatic compaction", func(ix *Index[string, Keyword]) error {
			return ix.Batch(func(b *Batch[string, Keyword]) error {
				for i := range 400 {
					b.Put(fmt.Sprintf("k%04d", i), kw(i, 10))
				}
				return nil
			})
		}},
		{"compact", func(ix *Index[string, Keyword]) error { return ix.Compact() }},
		{"rebuild", func(ix *Index[string, Keyword]) error { return ix.Rebuild() }},
	}
}

// TestCrashAtEveryStep fails each file step (write, sync, resize, create) of
// an operation in turn, also as a torn write, then opens the files again:
// the contents must be exactly the state before the operation or after it,
// the tree whole, and the index must take writes again.
func TestCrashAtEveryStep(t *testing.T) {
	opts := []Option{ChunkSize(4 << 10), DataFileSize(64 << 10), withGrowCap(128 << 10), withNoSync()}
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	base := filepath.Join(baseDir, "kw")
	ix := mustOpen[string, Keyword](t, base, opts...)
	for i := range 400 {
		ix.Put(fmt.Sprintf("k%04d", i), Keyword{Name: strings.Repeat("v", 60), Interest: int64(i)})
	}
	for i := range 100 {
		ix.Put(fmt.Sprintf("k%04d", i*2), Keyword{Name: "second", Interest: int64(i)})
	}
	before := dump(t, ix)
	ix.Close()

	run := 0
	for _, cc := range crashCases() {
		t.Run(cc.name, func(t *testing.T) {
			// Count the steps, and get the state after.
			dir := filepath.Join(root, fmt.Sprintf("r%d", run))
			run++
			copyDir(t, baseDir, dir)
			counter := &stepFault{}
			ix := mustOpen[string, Keyword](t, filepath.Join(dir, "kw"), append(opts, withFault(counter.hook))...)
			if err := cc.op(ix); err != nil {
				t.Fatal(err)
			}
			after := dump(t, ix)
			st := ix.Stats()
			if st.CompactErr != nil {
				t.Fatal(st.CompactErr)
			}
			firstSeq := st.DataFiles[0].Seq
			ix.Close()
			steps := counter.count
			if steps == 0 {
				t.Fatal("no steps counted")
			}
			t.Logf("%d steps: %s", steps, strings.Join(counter.steps, ", "))
			all := strings.Join(counter.steps, ",")
			checkCommitOrder(t, counter.steps)
			if cc.name == "put new key" {
				want := "write kw_0.data,write kw.index,write kw.index,sync kw_0.data,sync kw.index,write kw.index,sync kw.index"
				if all != want {
					t.Fatalf("put steps: %s; want (data and nodes written, one sync round, then the meta): %s", all, want)
				}
			}
			switch {
			case strings.HasPrefix(cc.name, "batch: new"):
				if !strings.Contains(all, "resize kw.index") || !strings.Contains(all, "create kw_") {
					t.Fatal("the batch did not grow the index and start a data file")
				}
			case strings.HasPrefix(cc.name, "batch: auto") || cc.name == "compact":
				if firstSeq != 1 {
					t.Fatalf("data file 0 was not compacted away (first file %d)", firstSeq)
				}
			}
			for n := 1; n <= steps; n++ {
				for _, torn := range []bool{false, true} {
					if torn && !strings.HasPrefix(counter.steps[n-1], "write") {
						continue
					}
					dir := filepath.Join(root, fmt.Sprintf("r%d", run))
					run++
					copyDir(t, baseDir, dir)
					b := filepath.Join(dir, "kw")
					f := &stepFault{n: n, torn: torn}
					ix := mustOpen[string, Keyword](t, b, append(opts, withFault(f.hook))...)
					err := cc.op(ix)
					if err != nil && !errors.Is(err, errInjected) {
						t.Fatalf("step %d: %v", n, err)
					}
					// The same index goes on after a failed write, unless the
					// meta may have been written.
					if err != nil && !errors.Is(err, ErrBroken) {
						if err := cc.op(ix); err != nil {
							t.Fatalf("step %d: retry: %v", n, err)
						}
						if got := dump(t, ix); !equalMaps(got, after) {
							t.Fatalf("step %d: retry gave the wrong state", n)
						}
					}
					ix.Close()

					ix = mustOpen[string, Keyword](t, b, opts...)
					got := dump(t, ix)
					if !equalMaps(got, before) && !equalMaps(got, after) {
						t.Fatalf("step %d (%s, torn %v): state is neither before nor after", n, f.steps[n-1], torn)
					}
					checkTree(t, ix.c)
					if err := ix.Put("probe", Keyword{Name: "probe"}); err != nil {
						t.Fatalf("step %d: write after reopen: %v", n, err)
					}
					ix.Close()
					ix = mustOpen[string, Keyword](t, b, opts...)
					if v, ok, err := ix.Get("probe"); !ok || err != nil || v.Name != "probe" {
						t.Fatalf("step %d: probe lost: %v", n, err)
					}
					checkTree(t, ix.c)
					ix.Close()
				}
			}
		})
	}
}

func equalMaps[K, V comparable](a, b map[K]V) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// checkCommitOrder checks the rule the commit order keeps: the meta (the
// fname.index write right after a sync round) is written only when every
// file written before it has been synced since, and it is synced itself
// before anything else is written.
func checkCommitOrder(t *testing.T, steps []string) {
	t.Helper()
	dirty := map[string]bool{}
	for i, s := range steps {
		op, file, _ := strings.Cut(s, " ")
		switch op {
		case "write", "resize", "create":
			if op == "write" && file == "kw.index" && i > 0 && strings.HasPrefix(steps[i-1], "sync ") {
				for f := range dirty {
					t.Fatalf("step %d: meta written while %s is not synced: %s", i+1, f, strings.Join(steps, ", "))
				}
				if i+1 >= len(steps) || steps[i+1] != "sync kw.index" {
					t.Fatalf("step %d: meta not synced next: %s", i+1, strings.Join(steps, ", "))
				}
			}
			dirty[file] = true
		case "sync":
			delete(dirty, file)
		}
	}
}

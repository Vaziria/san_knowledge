// Package san_btree is "San Btree+ Mmap": a key-value index on disk. The
// keys live in a B+tree in fname.index and the values in append-only data
// files fname_0.data, fname_1.data, ..., both read through read-only mmap
// (github.com/edsrzf/mmap-go) and written with WriteAt and Sync.
//
//	idx, err := san_btree.NewIndex[string, Keyword]("data/kw")
//	defer idx.Close()
//	err = idx.Put("aplikasi kasir", Keyword{Name: "aplikasi kasir", Interest: 100})
//	v, ok, err := idx.Get("aplikasi kasir")
//	for k, v := range idx.Prefix("aplikasi") { ... }
//
// Keys are numbers or strings, stored as bytes that sort like Go sorts them.
// Values are numbers, bools, strings, fixed arrays and structs of them: no
// pointers, maps, slices or interfaces. A value without strings is stored as
// its memory bytes, so View hands out a pointer into the mapped file.
//
// Every write is copy-on-write and ends in a commit (records synced, new
// nodes synced, then one of two metas written and synced), so a crash leaves
// the last commit or the one before. Many goroutines may read while one
// writes; one process opens an index at a time.
//
// The README next to this file has the whole design.
package san_btree

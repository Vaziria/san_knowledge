//go:build !windows && !linux

package san_btree

import "os"

// dataSync makes f's written bytes durable (fsync; F_FULLFSYNC on macOS
// through os.File.Sync).
func dataSync(f *os.File, full bool) (fallback bool, err error) { return false, f.Sync() }

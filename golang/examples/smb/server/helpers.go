package server

import (
	"encoding/binary"
	"net"
	"strings"
	"time"
)

func putTime(b []byte, off int, t time.Time) { set64(b, off, filetime(t)) }

func set64(b []byte, off int, v uint64) { binary.LittleEndian.PutUint64(b[off:], v) }

// foldName lower-cases a share name for case-insensitive lookup.
func foldName(s string) string { return strings.ToLower(s) }

// shareFromUNC extracts the share from a \\server\share tree-connect path.
func shareFromUNC(unc string) string {
	s := strings.ReplaceAll(unc, "/", `\`)
	s = strings.TrimLeft(s, `\`)
	if i := strings.IndexByte(s, '\\'); i >= 0 { // drop server
		s = s[i+1:]
	}
	if i := strings.IndexByte(s, '\\'); i >= 0 { // drop trailing path
		s = s[:i]
	}
	return s
}

func remote(nc any) string {
	if c, ok := nc.(net.Conn); ok {
		return c.RemoteAddr().String()
	}
	return ""
}

// oplockAck answers an OPLOCK_BREAK with a matching acknowledgement. The
// server never grants oplocks, so this is only a courtesy.
func oplockAck(reqBody []byte) []byte {
	b := make([]byte, 24)
	set16(b, 0, 24)
	if len(reqBody) >= 24 {
		copy(b[8:24], reqBody[8:24]) // echo FileId
	}
	return b
}

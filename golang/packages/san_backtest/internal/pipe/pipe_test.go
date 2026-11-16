package pipe

import (
	"bytes"
	"errors"
	"testing"
)

// limited refuses writes bigger than max, as a Windows pipe refuses one of
// tens of MB.
type limited struct {
	max int
	got bytes.Buffer
}

func (l *limited) Write(p []byte) (int, error) {
	if len(p) > l.max {
		return 0, errors.New("Insufficient system resources exist to complete the requested service.")
	}
	return l.got.Write(p)
}

func TestWriterSplitsBigWrites(t *testing.T) {
	l := &limited{max: maxWrite}
	msg := bytes.Repeat([]byte("0123456789"), 5<<20) // 50 MB
	n, err := Writer(l).Write(msg)
	if err != nil || n != len(msg) {
		t.Fatalf("wrote %d, %v", n, err)
	}
	if !bytes.Equal(l.got.Bytes(), msg) {
		t.Fatal("the bytes differ")
	}
}

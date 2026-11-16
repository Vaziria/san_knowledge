// Package pipe is the reading side of an approach plugin's pipes, shared by
// the host (package san_backtest) and the plugin (package plugin).
//
// A step is one small message each way, so a backtest's time over a plugin
// is mostly the pipe's wake-up latency: a read blocked in the kernel costs
// a thread switch on each side of every step. Reader first polls the pipe
// for a short while (the other side usually answers within microseconds)
// and only then blocks, which about halves a round trip on Windows. An idle
// side (a live session between candles) stops polling after Spin and
// blocks, so it uses no CPU while it waits.
package pipe

import (
	"io"
	"os"
	"time"
)

// Spin is how long a read polls before it blocks.
const Spin = 100 * time.Microsecond

// Reader reads f, polling for data for up to Spin before a blocking read
// where the system can tell how much is waiting (Windows pipes); elsewhere
// it is f itself.
func Reader(f io.Reader) io.Reader {
	if file, ok := f.(*os.File); ok {
		if r := spinReader(file); r != nil {
			return r
		}
	}
	return f
}

// maxWrite is the most one write to a pipe carries. Windows refuses a
// single pipe write of tens of MB ("Insufficient system resources exist to
// complete the requested service"), and bufio passes a message bigger than
// its buffer straight through as one write.
const maxWrite = 1 << 20

// Writer splits writes to w into pieces of at most 1 MB.
func Writer(w io.Writer) io.Writer { return chunked{w} }

type chunked struct{ w io.Writer }

func (c chunked) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		k := min(len(p), maxWrite)
		m, err := c.w.Write(p[:k])
		n += m
		if err != nil {
			return n, err
		}
		p = p[k:]
	}
	return n, nil
}

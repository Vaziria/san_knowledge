//go:build !windows

package audio_contextual_compiler

import (
	"errors"
	"io"
	"time"
)

// muteEngine silences what a render plays; only Windows' volume mixer is
// done so far.
func muteEngine(time.Time, io.Writer) (func(), error) {
	return nil, errors.New("silent rendering is done on Windows only so far")
}

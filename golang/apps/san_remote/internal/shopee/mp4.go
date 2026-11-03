package shopee

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// MP4Duration reads how long an MP4 file plays, from its boxes: the movie
// header (moov/mvhd: a timescale, and a duration in its units, 32-bit in
// version 0 and 64-bit in version 1), or for a fragmented file whose header
// gives no duration, the fragments' (moov/mvex/mehd). It reads only the box
// headers and those two boxes, wherever moov sits (before or after the
// media data), so no ffprobe is needed.
func MP4Duration(path string) (time.Duration, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return 0, err
	}
	return mp4Duration(f, fi.Size())
}

var errNoDuration = errors.New("not an MP4 with a duration")

func mp4Duration(r io.ReaderAt, size int64) (time.Duration, error) {
	moov, err := findBox(r, 0, size, "moov")
	if err != nil {
		return 0, err
	}
	if moov == nil {
		return 0, fmt.Errorf("%w: no moov box", errNoDuration)
	}
	mvhd, err := findBox(r, moov.body, moov.end, "mvhd")
	if err != nil {
		return 0, err
	}
	if mvhd == nil {
		return 0, fmt.Errorf("%w: no mvhd box", errNoDuration)
	}
	// version(1) flags(3), then creation and modification times, the
	// timescale and the duration.
	var timescale uint32
	var units uint64
	switch v, err := readAt(r, mvhd.body, 1, mvhd.end); {
	case err != nil:
		return 0, err
	case v[0] == 1:
		b, err := readAt(r, mvhd.body+4+16, 12, mvhd.end)
		if err != nil {
			return 0, err
		}
		timescale, units = binary.BigEndian.Uint32(b[:4]), binary.BigEndian.Uint64(b[4:])
	case v[0] == 0:
		b, err := readAt(r, mvhd.body+4+8, 8, mvhd.end)
		if err != nil {
			return 0, err
		}
		timescale, units = binary.BigEndian.Uint32(b[:4]), uint64(binary.BigEndian.Uint32(b[4:]))
		if units == 0xFFFFFFFF { // "unknown"
			units = 0
		}
	default:
		return 0, fmt.Errorf("%w: mvhd version %d", errNoDuration, v[0])
	}
	if timescale == 0 {
		return 0, fmt.Errorf("%w: a timescale of 0", errNoDuration)
	}
	if units == 0 {
		// A fragmented MP4 gives its length in mvex/mehd, in the same units.
		if mvex, err := findBox(r, moov.body, moov.end, "mvex"); err == nil && mvex != nil {
			if mehd, err := findBox(r, mvex.body, mvex.end, "mehd"); err == nil && mehd != nil {
				if v, err := readAt(r, mehd.body, 1, mehd.end); err == nil {
					if v[0] == 1 {
						if b, err := readAt(r, mehd.body+4, 8, mehd.end); err == nil {
							units = binary.BigEndian.Uint64(b)
						}
					} else if b, err := readAt(r, mehd.body+4, 4, mehd.end); err == nil {
						units = uint64(binary.BigEndian.Uint32(b))
					}
				}
			}
		}
	}
	if units == 0 {
		return 0, fmt.Errorf("%w: its header gives no duration", errNoDuration)
	}
	secs := float64(units) / float64(timescale)
	if secs > 1e7 {
		return 0, fmt.Errorf("%w: a duration of %.0f s", errNoDuration, secs)
	}
	return time.Duration(secs * float64(time.Second)).Round(time.Millisecond), nil
}

// box is where a box's body starts and ends in the file.
type box struct{ body, end int64 }

// findBox walks the boxes between off and end and returns the first of the
// type, or nil.
func findBox(r io.ReaderAt, off, end int64, typ string) (*box, error) {
	for off+8 <= end {
		h, err := readAt(r, off, 8, end)
		if err != nil {
			return nil, err
		}
		size, kind := int64(binary.BigEndian.Uint32(h[:4])), string(h[4:8])
		body := off + 8
		switch size {
		case 0: // to the end
			size = end - off
		case 1: // a 64-bit size follows the type
			b, err := readAt(r, off+8, 8, end)
			if err != nil {
				return nil, err
			}
			u := binary.BigEndian.Uint64(b)
			if u > uint64(end-off) {
				return nil, fmt.Errorf("%w: the box %q runs past the end", errNoDuration, kind)
			}
			size, body = int64(u), off+16
		}
		if size < body-off || off+size > end {
			return nil, fmt.Errorf("%w: a broken box %q at %d", errNoDuration, kind, off)
		}
		if kind == typ {
			return &box{body: body, end: off + size}, nil
		}
		off += size
	}
	return nil, nil
}

func readAt(r io.ReaderAt, off int64, n int, end int64) ([]byte, error) {
	if off+int64(n) > end {
		return nil, fmt.Errorf("%w: a box ends too soon", errNoDuration)
	}
	b := make([]byte, n)
	// ReadAt may say io.EOF with the last bytes of the file.
	if got, err := r.ReadAt(b, off); got < n {
		if err == nil || errors.Is(err, io.EOF) {
			err = fmt.Errorf("%w: the file ends inside a box", errNoDuration)
		}
		return nil, err
	}
	return b, nil
}

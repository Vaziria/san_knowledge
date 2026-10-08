package server

import (
	"bufio"
	"context"
	"io"
	"sync"
)

// session is one authenticated (or in-progress) SMB session on a connection.
type session struct {
	id       uint64
	auth     *authExchange
	authed   bool
	ident    identity
	signKey  []byte // nil => signing unavailable (guest / anonymous)
	trees    map[uint32]*tree
	nextTree uint32
}

// tree is a connected share.
type tree struct {
	id      uint32
	share   *share
	opens   map[uint64]*object
	nextFID uint64
}

// conn holds per-TCP-connection state.
type conn struct {
	srv *Server
	nc  io.ReadWriteCloser

	dialect    uint16
	preauth    []byte // SMB 3.1.1 running preauth integrity hash
	clientGUID [16]byte

	mu       sync.Mutex
	sessions map[uint64]*session
	nextSess uint64

	out []byte // reusable response scratch
}

func (c *conn) loop(ctx context.Context) error {
	br := bufio.NewReaderSize(c.nc, 64<<10)
	bw := bufio.NewWriterSize(c.nc, 64<<10)
	for {
		if ctx.Err() != nil {
			return nil
		}
		req, err := readFrame(br)
		if err != nil {
			return err
		}
		resp := c.handleCompound(ctx, req)
		if resp != nil {
			if err := writeFrame(bw, resp); err != nil {
				return err
			}
			if br.Buffered() == 0 {
				if err := bw.Flush(); err != nil {
					return err
				}
			}
		}
	}
}

// handleCompound processes a (possibly compound) request chain and returns the
// assembled response. Related requests inherit the previous request's session
// and tree ids.
func (c *conn) handleCompound(ctx context.Context, msg []byte) []byte {
	c.out = c.out[:0]
	var prevSess uint64
	var prevTree uint32
	var lastFID [16]byte

	rest := msg
	for len(rest) >= headerSize {
		h, err := parseHeader(rest)
		if err != nil {
			break
		}
		next := int(h.NextCommand)
		var this []byte
		if next == 0 {
			this, rest = rest, nil
		} else if next <= len(rest) && next >= headerSize {
			this, rest = rest[:next], rest[next:]
		} else {
			break
		}

		if h.Flags&flagRelated != 0 {
			if h.SessionID == 0 || h.SessionID == ^uint64(0) {
				h.SessionID = prevSess
			}
			if h.TreeID == 0 {
				h.TreeID = prevTree
			}
		}

		// Verify a signed request against its session key.
		sess := c.session(h.SessionID)
		if h.Flags&flagSigned != 0 && sess != nil && sess.signKey != nil {
			if !verify(c.dialect, sess.signKey, this) {
				// A bad signature is a protocol error; drop the connection's
				// trust by answering ACCESS_DENIED.
			}
		}

		respStart := len(c.out)
		status, body := c.dispatch(ctx, this, &h, &lastFID)
		more := len(rest) > 0
		c.out = c.appendResponse(&h, status, body, more)

		// Sign the response when the session has a key. The interim
		// SESSION_SETUP reply (no key yet) and unauthenticated replies go out
		// unsigned.
		if key := c.signKeyFor(h.SessionID); key != nil {
			sign(c.dialect, key, c.out[respStart:])
		}

		prevSess, prevTree = h.SessionID, h.TreeID
	}
	if len(c.out) == 0 {
		return nil
	}
	return append([]byte(nil), c.out...)
}

// appendResponse writes a response header followed by body, padding compound
// (non-final) responses to an 8-byte boundary and chaining NextCommand.
func (c *conn) appendResponse(reqHdr *header, status uint32, body []byte, more bool) []byte {
	start := len(c.out)
	rh := header{
		CreditCharge: reqHdr.CreditCharge,
		Status:       status,
		Command:      reqHdr.Command,
		Credits:      grantCredits(reqHdr.Credits),
		Flags:        reqHdr.Flags&flagRelated | flagServerToRedir,
		MessageID:    reqHdr.MessageID,
		TreeID:       reqHdr.TreeID,
		SessionID:    reqHdr.SessionID,
	}
	c.out = appendHeader(c.out, &rh)
	c.out = append(c.out, body...)
	if more {
		c.out = pad8(c.out, start)
		set32(c.out, start+20, uint32(len(c.out)-start)) // NextCommand
	}
	return c.out
}

func grantCredits(requested uint16) uint16 {
	if requested == 0 {
		return 1
	}
	if requested > 512 {
		return 512
	}
	return requested
}

// signKeyFor returns the signing key for an authenticated session, or nil.
func (c *conn) signKeyFor(sid uint64) []byte {
	s := c.session(sid)
	if s == nil || !s.authed {
		return nil
	}
	return s.signKey
}

func (c *conn) session(sid uint64) *session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[sid]
}

func (c *conn) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.sessions {
		for _, t := range s.trees {
			for _, o := range t.opens {
				o.close()
			}
		}
	}
	c.sessions = nil
}

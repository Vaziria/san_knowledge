package server

import (
	"context"
	"crypto/rand"
	"time"
)

// maxDialect is the highest dialect this server speaks. Capped at 3.0.2 to
// keep NEGOTIATE simple: 3.1.1 would add preauth-integrity hashing and
// negotiate contexts. 3.0.2 still gives AES-CMAC signing and NTLMv2.
const maxDialect = dialect302

var serverGUID = func() [16]byte {
	var g [16]byte
	rand.Read(g[:])
	return g
}()

// SMB2 field offsets are measured from the start of the header, so handlers
// take the whole message (msg) and slice the body themselves.
//
// dispatch handles one request and returns its status and response body (the
// bytes after the 64-byte header). It may update h — e.g. SessionId on the
// first SESSION_SETUP, or TreeId on TREE_CONNECT.
func (c *conn) dispatch(ctx context.Context, msg []byte, h *header, lastFID *[16]byte) (uint32, []byte) {
	switch h.Command {
	case cmdNegotiate:
		return c.doNegotiate(msg)
	case cmdSessionSetup:
		return c.doSessionSetup(ctx, msg, h)
	case cmdLogoff:
		return c.doLogoff(h)
	case cmdTreeConnect:
		return c.doTreeConnect(msg, h)
	case cmdTreeDisconnect:
		return c.doTreeDisconnect(h)
	case cmdCreate:
		return c.doCreate(msg, h, lastFID)
	case cmdClose:
		return c.doClose(msg, h, lastFID)
	case cmdRead:
		return c.doRead(msg, h, lastFID)
	case cmdWrite:
		return c.doWrite(msg, h, lastFID)
	case cmdQueryDirectory:
		return c.doQueryDirectory(msg, h, lastFID)
	case cmdQueryInfo:
		return c.doQueryInfo(msg, h, lastFID)
	case cmdSetInfo:
		return c.doSetInfo(msg, h, lastFID)
	case cmdFlush:
		return c.doFlush(msg, h, lastFID)
	case cmdIoctl:
		return c.doIoctl(msg, h, lastFID)
	case cmdEcho:
		return statusSuccess, []byte{4, 0, 0, 0}
	case cmdCancel:
		return statusCancelled, nil // handleCompound drops CANCEL responses
	case cmdChangeNotify:
		return statusNotImplemented, errorBody()
	case cmdOplockBreak:
		return statusSuccess, oplockAck(msg[headerSize:])
	default:
		return statusNotImplemented, errorBody()
	}
}

func (c *conn) doNegotiate(msg []byte) (uint32, []byte) {
	body := msg[headerSize:]
	if len(body) < 36 {
		return statusInvalidParameter, errorBody()
	}
	count := int(le16(body[2:]))
	best := uint16(0)
	for i := 0; i < count; i++ {
		d, ok := field(body, 36+2*i, 2)
		if !ok {
			break
		}
		if dl := le16(d); dl <= maxDialect && dl >= dialect202 && dl > best {
			best = dl
		}
	}
	if best == 0 {
		return statusNotSupported, errorBody()
	}
	c.dialect = best

	sec := negHint()
	b := make([]byte, 64)
	set16(b, 0, 65)   // StructureSize
	set16(b, 2, 0x01) // SecurityMode: signing enabled
	set16(b, 4, best) // DialectRevision
	copy(b[8:24], serverGUID[:])
	set32(b, 24, 0x00000004) // Capabilities: LARGE_MTU
	set32(b, 28, 8<<20)      // MaxTransactSize
	set32(b, 32, c.srv.maxRead)
	set32(b, 36, c.srv.maxWrite)
	putTime(b, 40, time.Now())
	set16(b, 56, 128) // SecurityBufferOffset = header(64)+fixed(64)
	set16(b, 58, uint16(len(sec)))
	return statusSuccess, append(b, sec...)
}

func (c *conn) doSessionSetup(ctx context.Context, msg []byte, h *header) (uint32, []byte) {
	body := msg[headerSize:]
	if len(body) < 24 {
		return statusInvalidParameter, errorBody()
	}
	tok, ok := field(msg, int(le16(body[12:])), int(le16(body[14:])))
	if !ok {
		return statusInvalidParameter, errorBody()
	}

	c.mu.Lock()
	sess := c.sessions[h.SessionID]
	if sess == nil {
		c.nextSess++
		id := c.nextSess
		sess = &session{
			id:    id,
			auth:  &authExchange{ntlm: ntlmServer{users: c.srv.cfg.Users, allowGuest: c.srv.cfg.AllowGuest, name: c.srv.name}},
			trees: make(map[uint32]*tree),
		}
		c.sessions[id] = sess
		h.SessionID = id
	}
	c.mu.Unlock()

	out, done, id, key, err := sess.auth.step(tok)
	if err != nil {
		return statusLogonFailure, errorBody()
	}

	flags := uint16(0)
	if done {
		sess.authed = true
		sess.ident = id
		if key != nil {
			sess.signKey = signingKey(c.dialect, key, nil)
		}
		if id.Guest {
			flags |= 0x0001 // IS_GUEST
		}
		if id.Anonymous {
			flags |= 0x0002 // IS_NULL
		}
		c.srv.log.Info("smb session", "user", nameOf(id), "guest", id.Guest || id.Anonymous, "remote", remote(c.nc))
	}

	b := make([]byte, 8)
	set16(b, 0, 9)
	set16(b, 2, flags)
	set16(b, 4, 72) // SecurityBufferOffset = header(64)+fixed(8)
	set16(b, 6, uint16(len(out)))
	b = append(b, out...)
	if done {
		return statusSuccess, b
	}
	return statusMoreProcessingRequired, b
}

func (c *conn) doLogoff(h *header) (uint32, []byte) {
	c.mu.Lock()
	if s := c.sessions[h.SessionID]; s != nil {
		for _, t := range s.trees {
			for _, o := range t.opens {
				o.close()
			}
		}
		delete(c.sessions, h.SessionID)
	}
	c.mu.Unlock()
	return statusSuccess, []byte{4, 0, 0, 0}
}

func (c *conn) doTreeConnect(msg []byte, h *header) (uint32, []byte) {
	sess := c.requireSession(h)
	if sess == nil {
		return statusUserSessionDeleted, errorBody()
	}
	body := msg[headerSize:]
	if len(body) < 8 {
		return statusInvalidParameter, errorBody()
	}
	raw, ok := field(msg, int(le16(body[4:])), int(le16(body[6:])))
	if !ok {
		return statusInvalidParameter, errorBody()
	}
	name := shareFromUNC(utf16Decode(raw))
	sh := c.srv.shares[foldName(name)]
	if sh == nil {
		return statusBadNetworkName, errorBody()
	}

	c.mu.Lock()
	sess.nextTree++
	tid := sess.nextTree
	sess.trees[tid] = &tree{id: tid, share: sh, opens: make(map[uint64]*object)}
	c.mu.Unlock()
	h.TreeID = tid

	b := make([]byte, 16)
	set16(b, 0, 16)
	b[2] = 0x01                  // ShareType: DISK
	access := uint32(0x001f01ff) // FILE_ALL_ACCESS
	if sh.ReadOnly {
		access = 0x001200a9
	}
	set32(b, 12, access) // MaximalAccess
	return statusSuccess, b
}

func (c *conn) doTreeDisconnect(h *header) (uint32, []byte) {
	if sess := c.requireSession(h); sess != nil {
		c.mu.Lock()
		if t := sess.trees[h.TreeID]; t != nil {
			for _, o := range t.opens {
				o.close()
			}
			delete(sess.trees, h.TreeID)
		}
		c.mu.Unlock()
	}
	return statusSuccess, []byte{4, 0, 0, 0}
}

// requireSession returns the authenticated session for h, or nil.
func (c *conn) requireSession(h *header) *session {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[h.SessionID]
	if s == nil || !s.authed {
		return nil
	}
	return s
}

// treeFor resolves the session and tree for a request.
func (c *conn) treeFor(h *header) (*tree, uint32) {
	s := c.requireSession(h)
	if s == nil {
		return nil, statusUserSessionDeleted
	}
	c.mu.Lock()
	t := s.trees[h.TreeID]
	c.mu.Unlock()
	if t == nil {
		return nil, statusNetworkNameDeleted
	}
	return t, statusSuccess
}

func nameOf(id identity) string {
	switch {
	case id.Anonymous:
		return "(anonymous)"
	case id.User == "":
		return "(guest)"
	default:
		return id.User
	}
}

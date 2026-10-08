// Package server is a small, dependency-light SMB2/3 file server. It exports
// one or more local directories read/write over SMB2 (dialects 2.0.2 to
// 3.1.1), with NTLMv2 authentication, SMB2/3 signing, and guest access.
//
// It is an example, not a Samba replacement: no encryption, oplocks, leases,
// DFS, or alternate data streams. It is enough to mount a share from Windows
// Explorer, macOS Finder, or Linux (mount -t cifs / smbclient) and read, write,
// list, rename, and delete files.
package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
)

// Config configures a Server.
type Config struct {
	Addr       string            // listen address, e.g. "0.0.0.0:4455"
	ServerName string            // NetBIOS name advertised in NTLM; defaults to "GOSMB"
	Shares     []ShareConfig     // exported directories
	Users      map[string]string // user -> password for NTLMv2; empty with AllowGuest for open access
	AllowGuest bool              // map unknown/anonymous logins to a guest identity
	Logger     *slog.Logger
}

// ShareConfig describes one exported directory.
type ShareConfig struct {
	Name     string // share name as seen by clients
	Path     string // local directory to export
	ReadOnly bool
}

// Server is a configured SMB server. Create it with New, then call Serve.
type Server struct {
	cfg      Config
	log      *slog.Logger
	shares   map[string]*share
	name     string
	maxRead  uint32
	maxWrite uint32

	mu  sync.Mutex
	lis net.Listener
}

// New validates cfg and returns a Server.
func New(cfg Config) (*Server, error) {
	if len(cfg.Shares) == 0 {
		return nil, errors.New("smb: no shares configured")
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	name := cfg.ServerName
	if name == "" {
		name = "GOSMB"
	}
	s := &Server{
		cfg:      cfg,
		log:      log,
		shares:   make(map[string]*share),
		name:     name,
		maxRead:  1 << 20,
		maxWrite: 1 << 20,
	}
	for _, sc := range cfg.Shares {
		if sc.Name == "" || sc.Path == "" {
			return nil, errors.New("smb: share needs a name and path")
		}
		s.shares[foldName(sc.Name)] = &share{Name: sc.Name, Root: sc.Path, ReadOnly: sc.ReadOnly}
	}
	return s, nil
}

// Serve listens and serves until ctx is cancelled or Addr cannot be bound.
func (s *Server) Serve(ctx context.Context) error {
	var lc net.ListenConfig
	lis, err := lc.Listen(ctx, "tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("smb: listen %s: %w", s.cfg.Addr, err)
	}
	s.mu.Lock()
	s.lis = lis
	s.mu.Unlock()
	s.log.Info("smb serving", "addr", lis.Addr().String(), "shares", len(s.shares))

	go func() {
		<-ctx.Done()
		lis.Close()
	}()

	var wg sync.WaitGroup
	for {
		c, err := lis.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.serveConn(ctx, c)
		}()
	}
}

// Addr reports the bound address once Serve has started (useful with ":0").
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lis == nil {
		return nil
	}
	return s.lis.Addr()
}

func (s *Server) serveConn(ctx context.Context, nc net.Conn) {
	defer nc.Close()
	c := &conn{
		srv:      s,
		nc:       nc,
		sessions: make(map[uint64]*session),
	}
	if err := c.loop(ctx); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
		s.log.Debug("smb connection closed", "remote", nc.RemoteAddr().String(), "err", err)
	}
	c.cleanup()
}

// readFrame reads one NetBIOS-session/Direct-TCP framed message: a 4-byte big
// endian length (top byte zero) followed by the SMB payload.
func readFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint32(hdr[:]) & 0x00FFFFFF)
	if n == 0 || n > 16<<20 {
		return nil, fmt.Errorf("smb: bad frame length %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeFrame(w io.Writer, payload []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

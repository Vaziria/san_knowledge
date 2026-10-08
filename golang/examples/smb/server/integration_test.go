package server_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	smb2 "github.com/hirochachacha/go-smb2"
	"github.com/wargasipil/examples/smb/server"
)

// startServer boots a server on a random port exporting a temp dir and returns
// the address, the dir, and a cleanup func.
func startServer(t *testing.T, cfg server.Config) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cfg.Addr = "127.0.0.1:0"
	cfg.Shares = []server.ShareConfig{{Name: "shared", Path: dir}}
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errc := make(chan error, 1)
	// Bind synchronously so Addr() is ready before the client dials.
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ln.Close()
	go func() { errc <- srv.Serve(ctx) }()

	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == nil {
		if time.Now().After(deadline) {
			t.Fatal("server did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return srv.Addr().String(), dir
}

func dial(t *testing.T, addr string, d *smb2.Dialer) *smb2.Session {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	s, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		t.Fatalf("smb dial: %v", err)
	}
	t.Cleanup(func() { s.Logoff() })
	return s
}

func TestGuestReadWriteList(t *testing.T) {
	addr, dir := startServer(t, server.Config{AllowGuest: true})
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello smb"), 0o644)
	os.Mkdir(filepath.Join(dir, "sub"), 0o755)

	s := dial(t, addr, &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "guest"}})
	share, err := s.Mount("shared")
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	defer share.Umount()

	// Read an existing file.
	got, err := share.ReadFile("hello.txt")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello smb" {
		t.Fatalf("read = %q", got)
	}

	// Write a new file, then read it back.
	if err := share.WriteFile("note.txt", []byte("written over smb"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	back, err := os.ReadFile(filepath.Join(dir, "note.txt"))
	if err != nil || string(back) != "written over smb" {
		t.Fatalf("server-side read = %q, err %v", back, err)
	}

	// Directory listing.
	entries, err := share.ReadDir(".")
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	want := map[string]bool{"hello.txt": true, "note.txt": true, "sub": true}
	found := 0
	for _, n := range names {
		if want[n] {
			found++
		}
	}
	if found != 3 {
		t.Fatalf("listing = %v, want hello.txt note.txt sub", names)
	}
}

func TestNTLMAuth(t *testing.T) {
	addr, dir := startServer(t, server.Config{Users: map[string]string{"alice": "secret"}})
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("ok"), 0o644)

	// Correct password works.
	s := dial(t, addr, &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "alice", Password: "secret"}})
	share, err := s.Mount("shared")
	if err != nil {
		t.Fatalf("mount with good creds: %v", err)
	}
	if b, err := share.ReadFile("f.txt"); err != nil || string(b) != "ok" {
		t.Fatalf("read = %q err %v", b, err)
	}
	share.Umount()

	// Wrong password is rejected.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bad := &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "alice", Password: "wrong"}}
	if _, err := bad.Dial(conn); err == nil {
		t.Fatal("expected auth failure with wrong password")
	}
}

func TestMkdirRenameDelete(t *testing.T) {
	addr, dir := startServer(t, server.Config{AllowGuest: true})
	s := dial(t, addr, &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "guest"}})
	share, err := s.Mount("shared")
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	defer share.Umount()

	if err := share.Mkdir("newdir", 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "newdir")); err != nil || !fi.IsDir() {
		t.Fatalf("mkdir not reflected on disk: %v", err)
	}

	share.WriteFile("a.txt", []byte("x"), 0o644)
	if err := share.Rename("a.txt", "b.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatalf("rename target missing: %v", err)
	}

	if err := share.Remove("b.txt"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.txt")); !os.IsNotExist(err) {
		t.Fatalf("file still present after remove: %v", err)
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	addr, _ := startServer(t, server.Config{AllowGuest: true})
	s := dial(t, addr, &smb2.Dialer{Initiator: &smb2.NTLMInitiator{User: "guest"}})
	share, err := s.Mount("shared")
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	defer share.Umount()

	if _, err := share.ReadFile(`..\..\..\..\Windows\win.ini`); err == nil {
		t.Fatal("expected traversal read to fail")
	}
}

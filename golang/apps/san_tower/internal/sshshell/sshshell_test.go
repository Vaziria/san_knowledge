package sshshell

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wargasipil/san_tower/internal/sshshell/sshtest"
)

func login(s *sshtest.Server) Login {
	return Login{Host: s.Host, Port: s.Port, User: s.User, Password: s.Password}
}

// One shell runs one command after another: a cd and an exported variable
// hold for the next, Input is the command's standard input, and what it
// printed comes back without the marks.
func TestShell(t *testing.T) {
	s := sshtest.New(t)
	ctx := context.Background()
	sh, err := Open(ctx, login(s))
	if err != nil {
		t.Fatal(err)
	}
	defer sh.Close()
	if sh.HostKey != s.HostKey {
		t.Errorf("host key = %q, want %q", sh.HostKey, s.HostKey)
	}
	run := func(cmd, input string) Result {
		t.Helper()
		r, err := sh.Run(ctx, cmd, input, 10*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	run("cd /\nexport SAN_X=hello", "")
	if r := run(`echo "$PWD $SAN_X"; printf 'no newline'`, ""); r.Stdout != "/ hello\nno newline" || r.Code != 0 || r.Stderr != "" {
		t.Errorf("state: %+v", r)
	}
	if r := run("cat; cat", "a\r\nb"); r.Stdout != "a\nb" {
		t.Errorf("input: %+v", r)
	}
	if r := run("cat", ""); r.Stdout != "" || r.Code != 0 {
		t.Errorf("no input: %+v", r)
	}
	if r := run("echo out; echo oops >&2; (exit 3)", ""); r.Code != 3 || r.Stdout != "out" || r.Stderr != "oops" {
		t.Errorf("exit 3: %+v", r)
	}
	if s.Shells.Load() != 1 {
		t.Errorf("shells = %d", s.Shells.Load())
	}

	// A command still running at its timeout closes the shell.
	r, err := sh.Run(ctx, "sleep 3", "", 300*time.Millisecond)
	if err != nil || !r.TimedOut {
		t.Errorf("timeout: %+v, %v", r, err)
	}
	if _, err := sh.Run(ctx, "true", "", time.Second); err == nil {
		t.Error("ran in a closed shell")
	}
}

// exit ends the shell: its code is the command's, and the shell has ended.
func TestShellExit(t *testing.T) {
	s := sshtest.New(t)
	sh, err := Open(context.Background(), login(s))
	if err != nil {
		t.Fatal(err)
	}
	defer sh.Close()
	r, err := sh.Run(context.Background(), "echo bye; exit 4", "", 10*time.Second)
	if err != nil || !r.Ended || r.Code != 4 || r.Stdout != "bye" || !sh.Ended() {
		t.Errorf("exit: %+v, %v, ended %v", r, err, sh.Ended())
	}
}

// A private key logs in; a wrong password, a locked key without its
// passphrase and a host key other than the one kept are refused.
func TestDial(t *testing.T) {
	s := sshtest.New(t)
	ctx := context.Background()
	l := login(s)
	l.Password, l.PrivateKey = "", s.ClientKey
	c, key, err := Dial(ctx, l)
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
	if key != s.HostKey || !strings.HasPrefix(Fingerprint(key), "ED25519 SHA256:") {
		t.Errorf("key = %q (%s)", key, Fingerprint(key))
	}
	l.HostKey = key
	if c, _, err := Dial(ctx, l); err != nil {
		t.Errorf("kept key: %v", err)
	} else {
		c.Close()
	}

	bad := login(s)
	bad.Password = "wrong"
	if _, _, err := Dial(ctx, bad); err == nil || !strings.Contains(err.Error(), "unable to authenticate") {
		t.Errorf("wrong password: %v", err)
	}
	other := sshtest.New(t)
	moved := login(s)
	moved.HostKey = other.HostKey
	if _, _, err := Dial(ctx, moved); err == nil || !strings.Contains(err.Error(), "host key changed") {
		t.Errorf("other host key: %v", err)
	}
	if _, err := Signer("not a key", ""); err == nil {
		t.Error("read a key from nothing")
	}
}

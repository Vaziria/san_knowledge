package server

import (
	"context"
	"strings"
	"testing"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/sshshell/sshtest"
)

// Test on an SSH connection logs in and says what answered and its host
// key; a wrong password or another host key is not ok.
func TestSSHTest(t *testing.T) {
	srv := sshtest.New(t)
	ctx := context.Background()
	login := func() *v1.SSHSettings {
		return &v1.SSHSettings{Host: srv.Host, Port: uint32(srv.Port), User: srv.User, Password: srv.Password}
	}
	r := testSSH(ctx, login())
	if !r.Ok || !strings.HasPrefix(r.Message, "pi@"+srv.Addr()+" answers") || !strings.Contains(r.Message, "host key ED25519 SHA256:") || !strings.HasSuffix(r.Message, "kept the first time a step connects") {
		t.Errorf("password: %v", r)
	}
	key := login()
	key.Password, key.PrivateKey, key.HostKey = "", srv.ClientKey, srv.HostKey
	if r := testSSH(ctx, key); !r.Ok || !strings.HasSuffix(r.Message, "the one kept") {
		t.Errorf("key: %v", r)
	}
	wrong := login()
	wrong.Password = "wrong"
	if r := testSSH(ctx, wrong); r.Ok || !strings.Contains(r.Message, "unable to authenticate") {
		t.Errorf("wrong password: %v", r)
	}
	moved := login()
	moved.HostKey = sshtest.New(t).HostKey
	if r := testSSH(ctx, moved); r.Ok || !strings.Contains(r.Message, "host key changed") {
		t.Errorf("other host key: %v", r)
	}
	if r := testSSH(ctx, &v1.SSHSettings{Host: "raspi"}); r.Ok || !strings.Contains(r.Message, "user") {
		t.Errorf("no user: %v", r)
	}
}

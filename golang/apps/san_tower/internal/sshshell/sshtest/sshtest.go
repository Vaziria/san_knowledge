// Package sshtest is an SSH server for tests, on 127.0.0.1: it logs in its
// user by password or by its client key, and runs bash for a shell (Git
// Bash's on Windows) or for a command.
package sshtest

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Server is a running test server.
type Server struct {
	Host string
	Port int
	// HostKey is its key, as a known_hosts line has it without the host.
	HostKey        string
	User, Password string
	// ClientKey is a private key (OpenSSH's format) it takes for User.
	ClientKey string
	// Shells counts the shells it started.
	Shells atomic.Int32

	bash string
	ln   net.Listener
	mu   sync.Mutex
	cmds []*exec.Cmd
}

// Bash finds bash: SAN_TOWER_BASH when set, Git's on Windows, else the
// PATH's.
func Bash() (string, error) {
	if p := os.Getenv("SAN_TOWER_BASH"); p != "" {
		return p, nil
	}
	if runtime.GOOS == "windows" {
		var tries []string
		if git, err := exec.LookPath("git"); err == nil {
			// ...\Git\cmd\git.exe or ...\Git\mingw64\bin\git.exe
			for d := filepath.Dir(git); d != filepath.Dir(d); d = filepath.Dir(d) {
				tries = append(tries, filepath.Join(d, "bin", "bash.exe"))
			}
		}
		if d := os.Getenv("ProgramFiles"); d != "" {
			tries = append(tries, filepath.Join(d, "Git", "bin", "bash.exe"))
		}
		for _, p := range tries {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
		return "", errors.New("no Git Bash")
	}
	return exec.LookPath("bash")
}

// New starts a server with the user "pi", password "secret", until the
// test ends. It skips the test when there is no bash.
func New(t testing.TB) *Server {
	t.Helper()
	bash, err := Bash()
	if err != nil {
		t.Skip("bash for the test SSH server:", err)
	}
	_, hostPriv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatal(err)
	}
	clientPub, clientPriv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(clientPriv, "")
	if err != nil {
		t.Fatal(err)
	}
	sshClientPub, _ := ssh.NewPublicKey(clientPub)

	s := &Server{
		HostKey: strings.TrimSpace(string(ssh.MarshalAuthorizedKey(hostSigner.PublicKey()))),
		User:    "pi", Password: "secret", ClientKey: string(pem.EncodeToMemory(block)), bash: bash,
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(m ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if m.User() == s.User && string(p) == s.Password {
				return nil, nil
			}
			return nil, errors.New("wrong password")
		},
		PublicKeyCallback: func(m ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if m.User() == s.User && string(k.Marshal()) == string(sshClientPub.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("unknown key")
		},
	}
	cfg.AddHostKey(hostSigner)
	s.ln, err = net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := s.ln.Addr().(*net.TCPAddr)
	s.Host, s.Port = "127.0.0.1", addr.Port
	go func() {
		for {
			c, err := s.ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c, cfg)
		}
	}()
	t.Cleanup(func() {
		s.ln.Close()
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, c := range s.cmds {
			if c.Process != nil {
				c.Process.Kill()
			}
		}
	})
	return s
}

// Addr is host:port.
func (s *Server) Addr() string { return net.JoinHostPort(s.Host, strconv.Itoa(s.Port)) }

func (s *Server) serve(c net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "sessions only")
			continue
		}
		ch, reqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go s.session(ch, reqs)
	}
}

// session runs bash for a shell or an exec request, and ends the channel
// with its exit status. The channel closing kills it.
func (s *Server) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	var cmd *exec.Cmd
	for req := range reqs {
		switch req.Type {
		case "shell", "exec":
			if cmd != nil {
				req.Reply(false, nil)
				continue
			}
			args := []string{}
			if req.Type == "exec" {
				var p struct{ Command string }
				ssh.Unmarshal(req.Payload, &p)
				args = []string{"-c", p.Command}
			} else {
				s.Shells.Add(1)
			}
			cmd = exec.Command(s.bash, args...)
			in, _ := cmd.StdinPipe()
			cmd.Stdout, cmd.Stderr = ch, ch.Stderr()
			cmd.WaitDelay = time.Second
			if err := cmd.Start(); err != nil {
				req.Reply(false, nil)
				ch.Close()
				return
			}
			s.mu.Lock()
			s.cmds = append(s.cmds, cmd)
			s.mu.Unlock()
			req.Reply(true, nil)
			go func() {
				io.Copy(in, ch)
				in.Close()
			}()
			go func(cmd *exec.Cmd) {
				code := 0
				if err := cmd.Wait(); err != nil {
					code = 255
					if exit, ok := errors.AsType[*exec.ExitError](err); ok {
						code = exit.ExitCode()
					}
				}
				ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(code)}))
				ch.Close()
			}(cmd)
		default:
			req.Reply(req.Type == "pty-req" || req.Type == "env", nil)
		}
	}
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}

// Package sshshell is a shell on a server over SSH that runs one command
// after another, as a person types them, so what a command changes (its
// folder, an exported variable) holds for the next: what the tower's SSH
// Session and SSH Exec steps use, with golang.org/x/crypto/ssh.
package sshshell

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Login is an account on a server and how to log in to it.
type Login struct {
	Host string
	Port int // 22 when 0
	User string
	// The private key logs in when given, else the password.
	Password, PrivateKey, Passphrase string
	// HostKey is the server's key as kept, as a known_hosts line has it
	// without the host (ssh-ed25519 AAAA...). Empty trusts the key the
	// server gives, which Dial returns to keep.
	HostKey string
}

// Addr is host:port.
func (l Login) Addr() string {
	port := l.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(l.Host, strconv.Itoa(port))
}

// String is user@host:port, as the steps' logs name a server.
func (l Login) String() string { return l.User + "@" + l.Addr() }

// Signer reads a private key, locked with passphrase when it is not empty.
func Signer(privateKey, passphrase string) (ssh.Signer, error) {
	key := []byte(strings.TrimSpace(privateKey) + "\n")
	var s ssh.Signer
	var err error
	if passphrase != "" {
		s, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
	} else {
		s, err = ssh.ParsePrivateKey(key)
	}
	if _, ok := errors.AsType[*ssh.PassphraseMissingError](err); ok {
		return nil, errors.New("the private key is locked with a passphrase: give it")
	}
	if err != nil {
		return nil, fmt.Errorf("the private key cannot be read: %w", err)
	}
	return s, nil
}

// Fingerprint names a host key as ssh does: "ED25519 SHA256:...".
func Fingerprint(hostKey string) string {
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKey))
	if err != nil {
		return hostKey
	}
	return strings.ToUpper(strings.TrimPrefix(k.Type(), "ssh-")) + " " + ssh.FingerprintSHA256(k)
}

// handshakeTimeout bounds connecting and logging in.
const handshakeTimeout = 20 * time.Second

// Dial connects to the server and logs in. It returns the client and the
// server's host key (as Login.HostKey has it). A server whose key is not
// the one kept is refused.
func Dial(ctx context.Context, l Login) (*ssh.Client, string, error) {
	cfg := &ssh.ClientConfig{User: l.User, Timeout: handshakeTimeout}
	if strings.TrimSpace(l.PrivateKey) != "" {
		s, err := Signer(l.PrivateKey, l.Passphrase)
		if err != nil {
			return nil, "", err
		}
		cfg.Auth = []ssh.AuthMethod{ssh.PublicKeys(s)}
	} else {
		// Servers ask for a password either way.
		cfg.Auth = []ssh.AuthMethod{ssh.Password(l.Password), ssh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range answers {
				answers[i] = l.Password
			}
			return answers, nil
		})}
	}
	var kept ssh.PublicKey
	if strings.TrimSpace(l.HostKey) != "" {
		k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(l.HostKey))
		if err != nil {
			return nil, "", fmt.Errorf("the kept host key cannot be read: %w", err)
		}
		kept = k
	}
	var seen string
	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		seen = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
		if kept != nil && !bytes.Equal(kept.Marshal(), key.Marshal()) {
			return fmt.Errorf("the server's host key changed: it gives %s, and the connection kept %s; clear the connection's Host Key if that is expected", Fingerprint(seen), Fingerprint(l.HostKey))
		}
		return nil
	}

	d := net.Dialer{Timeout: handshakeTimeout}
	conn, err := d.DialContext(ctx, "tcp", l.Addr())
	if err != nil {
		return nil, "", err
	}
	// The handshake has no context: a deadline, and closing on ctx.
	conn.SetDeadline(time.Now().Add(handshakeTimeout))
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, l.Addr(), cfg)
	stop()
	if err != nil {
		conn.Close()
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		return nil, "", fmt.Errorf("%s: %w", l, err)
	}
	conn.SetDeadline(time.Time{})
	return ssh.NewClient(c, chans, reqs), seen, nil
}

// Probe logs in, as the Connections page's Test does, and asks the server
// what it is (uname -snrm), which changes nothing: it returns the server's
// host key and that answer, empty when the server has no uname.
func Probe(ctx context.Context, l Login) (hostKey, uname string, err error) {
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	client, key, err := Dial(ctx, l)
	if err != nil {
		return "", "", err
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { client.Close() })
	defer stop()
	if sess, err := client.NewSession(); err == nil {
		out, _ := sess.Output("uname -snrm")
		sess.Close()
		uname = strings.TrimSpace(string(out))
	}
	return key, uname, nil
}

// outputMax is what a command keeps of each of stdout and stderr: past it,
// the rest is counted and left out.
const outputMax = 1 << 20

// Shell is the user's login shell on a server, without a terminal, read
// from its standard input. One command runs at a time.
type Shell struct {
	// HostKey is the server's key, as Login.HostKey has it.
	HostKey string
	// Opened is when the shell started.
	Opened time.Time

	client *ssh.Client
	sess   *ssh.Session
	stdin  io.WriteCloser
	stdout chan string // lines, with their line break; closed at the end
	stderr chan string
	ended  chan struct{} // closed when the shell has ended
	code   int           // its exit status, once ended
	once   sync.Once
}

// Open logs in and starts a shell.
func Open(ctx context.Context, l Login) (*Shell, error) {
	client, key, err := Dial(ctx, l)
	if err != nil {
		return nil, err
	}
	s, err := start(client)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("%s: %w", l, err)
	}
	s.HostKey = key
	return s, nil
}

func start(client *ssh.Client) (*Shell, error) {
	sess, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	s := &Shell{client: client, sess: sess, Opened: time.Now(), stdout: make(chan string, 256), stderr: make(chan string, 256), ended: make(chan struct{})}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		return nil, err
	}
	out, err := sess.StdoutPipe()
	if err != nil {
		return nil, err
	}
	errOut, err := sess.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := sess.Shell(); err != nil {
		return nil, fmt.Errorf("the server did not start a shell: %w", err)
	}
	go lines(out, s.stdout)
	go lines(errOut, s.stderr)
	go func() {
		err := sess.Wait()
		var exit *ssh.ExitError
		switch {
		case errors.As(err, &exit):
			s.code = exit.ExitStatus()
		case err != nil:
			s.code = -1
		}
		close(s.ended)
	}()
	return s, nil
}

// lines sends what r gives, a line at a time, and closes ch at its end.
func lines(r io.Reader, ch chan<- string) {
	br := bufio.NewReaderSize(r, 64<<10)
	for {
		line, err := br.ReadString('\n')
		if line != "" {
			ch <- line
		}
		if err != nil {
			close(ch)
			return
		}
	}
}

// Ended says whether the shell has ended: a command exited it, or the
// server or the network closed it.
func (s *Shell) Ended() bool {
	select {
	case <-s.ended:
		return true
	default:
		return false
	}
}

// Close ends the shell and logs out.
func (s *Shell) Close() error {
	var err error
	s.once.Do(func() {
		s.sess.Close()
		err = s.client.Close()
	})
	return err
}

// Result is what a command gave.
type Result struct {
	Stdout, Stderr string
	Code           int
	Took           time.Duration
	// TimedOut: the command was still running after its timeout, and the
	// shell was closed, since without a terminal nothing else stops it.
	TimedOut bool
	// Ended: the shell ended during the command (it ran exit, or the
	// server closed it); Code is the shell's exit status.
	Ended bool
}

// Run runs command in the shell with input on its standard input (nothing
// when empty), and waits for it to end, timeout at most. What it printed
// is kept to 1 MB each, the last line break trimmed. The shell is closed
// when ctx ends or the timeout passes first.
func (s *Shell) Run(ctx context.Context, command, input string, timeout time.Duration) (Result, error) {
	if s.Ended() {
		return Result{}, errors.New("the shell has ended")
	}
	var b [8]byte
	rand.Read(b[:])
	mark := "__san_tower_" + hex.EncodeToString(b[:])

	// The command runs in a group with its own standard input, so it reads
	// Input, or nothing, never what comes after it. Then the exit code goes
	// to stdout after a mark, and a mark to stderr, each on a line of its
	// own: the command's output is what comes before them.
	var w strings.Builder
	w.WriteString("{\n")
	w.WriteString(strings.ReplaceAll(command, "\r\n", "\n"))
	if input == "" {
		w.WriteString("\n} </dev/null\n")
	} else {
		in := strings.ReplaceAll(input, "\r\n", "\n")
		if !strings.HasSuffix(in, "\n") {
			in += "\n"
		}
		fmt.Fprintf(&w, "\n} <<'%s_in'\n%s%s_in\n", mark, in, mark)
	}
	fmt.Fprintf(&w, "printf '\\n%%s %%d\\n' %s $?\nprintf '\\n%%s\\n' %s >&2\n", mark, mark)

	start := time.Now()
	if _, err := io.WriteString(s.stdin, w.String()); err != nil {
		s.Close()
		return Result{}, fmt.Errorf("the shell does not take the command: %w", err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var res Result
	var out, errOut capped
	outDone, errDone := false, false
	stdout, stderr := s.stdout, s.stderr
	for !outDone || !errDone {
		select {
		case line, ok := <-stdout:
			switch {
			case !ok:
				outDone, stdout = true, nil
				res.Ended = true
			case strings.HasPrefix(line, mark+" "):
				outDone, stdout = true, nil
				res.Code, _ = strconv.Atoi(strings.TrimSpace(line[len(mark)+1:]))
			default:
				out.write(line)
			}
		case line, ok := <-stderr:
			switch {
			case !ok:
				errDone, stderr = true, nil
				res.Ended = true
			case strings.TrimSpace(line) == mark:
				errDone, stderr = true, nil
			default:
				errOut.write(line)
			}
		case <-timer.C:
			s.Close()
			res.TimedOut = true
			outDone, errDone = true, true
		case <-ctx.Done():
			s.Close()
			return res, ctx.Err()
		}
	}
	if res.Ended {
		select {
		case <-s.ended:
		case <-time.After(5 * time.Second):
		}
		res.Code = s.code
		s.Close()
	}
	res.Took = time.Since(start).Round(time.Millisecond)
	res.Stdout, res.Stderr = out.String(), errOut.String()
	return res, nil
}

// capped keeps the first outputMax bytes written to it and counts the
// rest.
type capped struct {
	b    strings.Builder
	more int
}

func (c *capped) write(s string) {
	if room := outputMax - c.b.Len(); room >= len(s) {
		c.b.WriteString(s)
	} else {
		c.b.WriteString(s[:max(room, 0)])
		c.more += len(s) - max(room, 0)
	}
}

// String is what was kept, its last line breaks trimmed (one of them the
// mark's), as valid UTF-8, with a note of what was left out.
func (c *capped) String() string {
	s := strings.ToValidUTF8(strings.TrimRight(c.b.String(), "\r\n"), "�")
	if c.more > 0 {
		s += fmt.Sprintf("\n(%d more bytes left out)", c.more)
	}
	return s
}

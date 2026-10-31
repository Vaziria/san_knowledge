package engine

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/sshshell"
	"github.com/wargasipil/san_tower/internal/store"
)

// sshKinds are SSH Session and SSH Exec (the user, 2026-10-31: "ssh session
// node and ssh execution node"; a shell that goes on from run to run, which
// logs in with a password or a pasted private key).
var sshKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "ssh-session", Title: "SSH Session", Category: "Action", Group: "SSH",
			Description: "A shell on a server, through an SSH connection, that goes on from run to run: link Session to the Session of SSH Exec steps, and each runs its command in that same shell, so a cd or an exported variable holds for the next command, in this run or a later one. The shell is closed after Close After (15m) with no command, when the tower stops, or when a command ends it (exit); the next command opens a new one, in the login folder.",
			Outputs:     []*v1.Socket{sock("Session", Text)},
			Settings:    []*v1.Setting{{Name: "Connection", ConnectionType: store.SSH}, {Name: "Close After", Placeholder: "15m"}},
		},
		run: sshSession,
	},
	{
		StepKind: &v1.StepKind{
			Kind: "ssh-exec", Title: "SSH Exec", Category: "Action", Group: "SSH",
			Description: "Runs Command in an SSH Session's shell, with Input on its standard input; Stdout, Stderr and Exit Code are what it gave. An exit code other than 0 fails the step (Failed, with Error), unless Non-zero Exit is Goes On. A command still running after Timeout (5m) fails the step and closes the shell, since nothing else stops it.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Stdout", Text), sock("Stderr", Text), sock("Exit Code", Text)},
			Settings: []*v1.Setting{
				{Name: "Timeout", Placeholder: "5m"},
				{Name: "Non-zero Exit", Options: []string{"Fails", "Goes On"}},
			},
			Inputs: []*v1.Socket{sock("Event", Event), sock("Session", Text), field("Command"), field("Input")},
		},
		run: sshExec,
	},
}

// SSHShells are the shells SSH Session steps keep open from run to run, by
// session; the engine has one, and closes them when it stops.
type SSHShells struct {
	mu       sync.Mutex
	sessions map[string]sshSessionOf
	shells   map[string]*sshShell
}

// sshSessionOf is what an SSH Session step said its session is: its
// connection and how long its shell stays open with no command.
type sshSessionOf struct {
	connection uint32
	idle       time.Duration
}

// sshShell is a session's open shell: on connection, with login (its
// settings as the shell was opened, to see them change). mu is held while a
// command runs; idle closes it after its session's Close After; gone
// once it is closed and forgotten.
type sshShell struct {
	mu         sync.Mutex
	shell      *sshshell.Shell
	connection uint32
	login      *v1.SSHSettings
	idle       *time.Timer
	gone       atomic.Bool
}

func NewSSHShells() *SSHShells {
	return &SSHShells{sessions: make(map[string]sshSessionOf), shells: make(map[string]*sshShell)}
}

// Close closes every shell.
func (s *SSHShells) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, sh := range s.shells {
		sh.gone.Store(true)
		sh.idle.Stop()
		sh.shell.Close()
		delete(s.shells, key)
	}
}

// drop closes sh and forgets it, if it is still session key's shell.
func (s *SSHShells) drop(key string, sh *sshShell) {
	s.mu.Lock()
	if s.shells[key] == sh {
		delete(s.shells, key)
	}
	s.mu.Unlock()
	sh.gone.Store(true)
	sh.idle.Stop()
	sh.shell.Close()
}

// sshSpace is the namespace of the sessions SSH Session makes.
var sshSpace = [16]byte{0x2c, 0x5e, 0x91, 0x07, 0xd4, 0x3a, 0x4f, 0x6b, 0x88, 0x12, 0xe7, 0x3d, 0x50, 0xa9, 0x1c, 0x64}

// sshSessionKey is an SSH Session step's session: a UUID made from its
// workflow and its own ID (version 5), the same in every run, so that its
// shell goes on. Another step, or the same step in an imported copy of the
// workflow, has another.
func sshSessionKey(workflow uint32, step string) string {
	h := sha1.New()
	h.Write(sshSpace[:])
	fmt.Fprintf(h, "%d/%s", workflow, step)
	sum := h.Sum(nil)
	sum[6] = sum[6]&0x0f | 0x50 // version 5
	sum[8] = sum[8]&0x3f | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

func sshSession(c *call) error {
	if c.setting("Connection") == "" {
		return errors.New("Connection: pick an SSH connection (Connections page)")
	}
	id, err := c.connectionID("SSH")
	if err != nil {
		return err
	}
	idle := 15 * time.Minute
	if s := c.setting("Close After"); s != "" {
		if idle, err = time.ParseDuration(s); err != nil || idle <= 0 || idle > 24*time.Hour {
			return fmt.Errorf("Close After %q: give a duration such as 30s, 15m or 2h, up to 24h", s)
		}
	}
	if c.env.SSH == nil {
		return errors.New("this run has no SSH shells")
	}
	key := sshSessionKey(c.env.Workflow, c.step.GetId())
	c.env.SSH.mu.Lock()
	c.env.SSH.sessions[key] = sshSessionOf{connection: id, idle: idle}
	c.env.SSH.mu.Unlock()
	c.outputs["Session"] = stringValue(key)
	return nil
}

func sshExec(c *call) error {
	key := c.text("Session")
	if key == "" {
		return errors.New("Session: link an SSH Session's Session")
	}
	command := c.text("Command")
	if command == "" {
		return fmt.Errorf("Command %w", errEmpty)
	}
	timeout, err := c.timeout()
	if err != nil {
		return err
	}
	shells := c.env.SSH
	if shells == nil {
		return errors.New("this run has no SSH shells")
	}
	shells.mu.Lock()
	of, ok := shells.sessions[key]
	shells.mu.Unlock()
	if !ok {
		return fmt.Errorf("Session %q is not an SSH Session's: link one", key)
	}
	if c.env.Connection == nil {
		return errors.New("this run has no connections")
	}
	conn, err := c.env.Connection(of.connection)
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("connection %d is gone: pick another", of.connection)
	}
	if err != nil {
		return err
	}
	if conn.Type != store.SSH {
		return fmt.Errorf("connection %s is not an SSH connection", conn.Name)
	}
	sh, err := c.sshShell(shells, key, of, conn)
	if err != nil {
		return err
	}
	defer sh.mu.Unlock()

	res, err := sh.shell.Run(c.ctx, command, c.text("Input"), timeout)
	switch {
	case err != nil:
		shells.drop(key, sh)
		return err
	case res.TimedOut:
		shells.drop(key, sh)
		return fmt.Errorf("stopped after the Timeout, %s: the shell is closed, and the next command opens a new one%s", timeout, tail(res.Stderr))
	case res.Ended:
		shells.drop(key, sh)
		c.logs = append(c.logs, fmt.Sprintf("the shell ended (exit %d): the next command opens a new one", res.Code))
	default:
		sh.idle.Reset(of.idle)
	}
	c.outputs["Stdout"] = stringValue(res.Stdout)
	c.outputs["Stderr"] = stringValue(res.Stderr)
	c.outputs["Exit Code"] = stringValue(strconv.Itoa(res.Code))
	c.logs = append(c.logs, fmt.Sprintf("exit %d after %s, on %s", res.Code, res.Took, sshLogin(sh.login)))
	if res.Code != 0 && c.setting("Non-zero Exit") != "Goes On" {
		return fmt.Errorf("exit code %d%s", res.Code, tail(res.Stderr))
	}
	c.fire("Done")
	return nil
}

// sshShell is session key's shell, locked: the one open, unless its
// connection or its settings changed since (then it is closed), else a
// new one, whose server's host key the connection keeps when it has none
// yet.
func (c *call) sshShell(shells *SSHShells, key string, of sshSessionOf, conn store.Connection) (*sshShell, error) {
	login := conn.Settings.GetSsh()
	shells.mu.Lock()
	sh := shells.shells[key]
	shells.mu.Unlock()
	if sh != nil {
		sh.mu.Lock()
		// The host key is kept after the shell opened: it is no change.
		was := proto.Clone(sh.login).(*v1.SSHSettings)
		was.HostKey = login.GetHostKey()
		if sh.connection == of.connection && proto.Equal(was, login) && !sh.gone.Load() && !sh.shell.Ended() {
			return sh, nil
		}
		sh.mu.Unlock()
		shells.drop(key, sh)
	}

	shell, err := sshshell.Open(c.ctx, sshshell.Login{
		Host: login.GetHost(), Port: int(login.GetPort()), User: login.GetUser(),
		Password: login.GetPassword(), PrivateKey: login.GetPrivateKey(), Passphrase: login.GetPassphrase(),
		HostKey: login.GetHostKey(),
	})
	if err != nil {
		return nil, err
	}
	if login.GetHostKey() == "" {
		if c.env.KeepHostKey != nil {
			if err := c.env.KeepHostKey(of.connection, shell.HostKey); err != nil {
				c.logf(v1.LogLevel_LOG_LEVEL_WARNING, "the host key %s was not kept: %v", sshshell.Fingerprint(shell.HostKey), err)
			}
		}
		c.logs = append(c.logs, fmt.Sprintf("kept %s's host key, %s: a server that gives another is refused from now on", sshLogin(login), sshshell.Fingerprint(shell.HostKey)))
	}
	c.logs = append(c.logs, fmt.Sprintf("opened a shell on %s", sshLogin(login)))
	sh = &sshShell{shell: shell, connection: of.connection, login: proto.Clone(login).(*v1.SSHSettings)}
	sh.mu.Lock()
	sh.idle = time.AfterFunc(of.idle, func() {
		// A command running keeps it open: its end sets the timer again.
		if sh.mu.TryLock() {
			shells.drop(key, sh)
			sh.mu.Unlock()
		}
	})
	shells.mu.Lock()
	shells.shells[key] = sh
	shells.mu.Unlock()
	return sh, nil
}

// sshLogin names a server as the logs do: user@host:port.
func sshLogin(s *v1.SSHSettings) string {
	return sshshell.Login{Host: s.GetHost(), Port: int(s.GetPort()), User: s.GetUser()}.String()
}

package engine

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/internal/sshshell/sshtest"
	"github.com/wargasipil/san_tower/internal/store"
)

// sshEnv is an SSH connection (1) to a test server, logging in by password,
// with the host key it keeps; connection 2 logs in with the server's
// client key.
func sshEnv(t *testing.T) (*sshtest.Server, Env, func() string) {
	t.Helper()
	srv := sshtest.New(t)
	var mu sync.Mutex
	kept := ""
	shells := NewSSHShells()
	t.Cleanup(shells.Close)
	return srv, Env{
		Workflow: 7,
		SSH:      shells,
		Connection: func(id uint32) (store.Connection, error) {
			mu.Lock()
			defer mu.Unlock()
			s := &v1.SSHSettings{Host: srv.Host, Port: uint32(srv.Port), User: srv.User, HostKey: kept}
			switch id {
			case 1:
				s.Password = srv.Password
			case 2:
				s.PrivateKey = srv.ClientKey
			default:
				return store.Connection{}, store.ErrNotFound
			}
			return store.Connection{ID: id, Name: "Pi", Type: store.SSH, Settings: &v1.ConnectionSettings{Type: &v1.ConnectionSettings_Ssh{Ssh: s}}}, nil
		},
		KeepHostKey: func(_ uint32, key string) error {
			mu.Lock()
			defer mu.Unlock()
			if kept == "" {
				kept = key
			}
			return nil
		},
	}, func() string { mu.Lock(); defer mu.Unlock(); return kept }
}

// sshRun runs SSH Exec with values after a topic event, its Session linked
// to an SSH Session with session's values.
func sshRun(env Env, session []string, values ...string) ([]*v1.StepRun, error) {
	g := &v1.Graph{
		Steps: []*v1.Step{step("ev", TopicEvent), step("sess", "ssh-session", session...), step("x", "ssh-exec", values...)},
		Links: []*v1.Link{link("ev", "Event", "x", "Event"), link("sess", "Session", "x", "Session")},
	}
	return Execute(context.Background(), g, "ev", nil, env)
}

// The shell goes on from run to run: a cd and an exported variable hold
// for the next run's command, in the same shell; the host key is kept on
// the first connect.
func TestSSHSession(t *testing.T) {
	srv, env, kept := sshEnv(t)
	pi := []string{"Connection", "1"}
	steps, err := sshRun(env, pi, "Command", "cd /\nexport SAN_X=hello\necho started")
	if err != nil {
		t.Fatal(err)
	}
	if out := textOut(t, steps[len(steps)-1], "Stdout"); out != "started" {
		t.Errorf("Stdout = %q", out)
	}
	logs := logsOf(steps[len(steps)-1])
	if kept() != srv.HostKey || !strings.Contains(logs, "kept pi@127.0.0.1:") || !strings.Contains(logs, "opened a shell on pi@127.0.0.1:") {
		t.Errorf("kept %q, logs %q", kept(), logs)
	}
	steps, err = sshRun(env, pi, "Command", `echo "$PWD $SAN_X"; cat`, "Input", "in")
	if err != nil {
		t.Fatal(err)
	}
	last := steps[len(steps)-1]
	if out := textOut(t, last, "Stdout"); out != "/ hello\nin" {
		t.Errorf("next run's Stdout = %q", out)
	}
	if strings.Contains(logsOf(last), "opened") || srv.Shells.Load() != 1 {
		t.Errorf("a new shell: %q, %d shells", logsOf(last), srv.Shells.Load())
	}

	// Another connection on the same step is another login: a new shell.
	steps, err = sshRun(env, []string{"Connection", "2"}, "Command", `echo "[$SAN_X]"`)
	if err != nil {
		t.Fatal(err)
	}
	if out := textOut(t, steps[len(steps)-1], "Stdout"); out != "[]" || srv.Shells.Load() != 2 {
		t.Errorf("key login: %q, %d shells", out, srv.Shells.Load())
	}
}

// A non-zero exit fails the step unless Non-zero Exit is Goes On; exit
// ends the shell, a timeout closes it, and Close After closes it idle: the
// next command opens a new one each time.
func TestSSHExecEnds(t *testing.T) {
	srv, env, _ := sshEnv(t)
	pi := []string{"Connection", "1", "Close After", "500ms"}
	if _, err := sshRun(env, pi, "Command", "echo oops >&2; false"); err == nil || !strings.Contains(err.Error(), "exit code 1: oops") {
		t.Errorf("exit 1: %v", err)
	}
	steps, err := sshRun(env, pi, "Command", "export SAN_X=1; (exit 2)", "Non-zero Exit", "Goes On")
	if err != nil {
		t.Fatal(err)
	}
	if code := textOut(t, steps[len(steps)-1], "Exit Code"); code != "2" || srv.Shells.Load() != 1 {
		t.Errorf("goes on: exit %q, %d shells", code, srv.Shells.Load())
	}

	steps, err = sshRun(env, pi, "Command", "exit 0")
	if err != nil {
		t.Fatal(err)
	}
	if l := logsOf(steps[len(steps)-1]); !strings.Contains(l, "the shell ended (exit 0)") {
		t.Errorf("exit logs = %q", l)
	}
	if steps, err = sshRun(env, pi, "Command", `echo "[$SAN_X]"`); err != nil || textOut(t, steps[len(steps)-1], "Stdout") != "[]" || srv.Shells.Load() != 2 {
		t.Errorf("after exit: %v, %d shells", err, srv.Shells.Load())
	}

	if _, err := sshRun(env, pi, "Command", "sleep 2", "Timeout", "300ms"); err == nil || !strings.Contains(err.Error(), "stopped after the Timeout, 300ms: the shell is closed") {
		t.Errorf("timeout: %v", err)
	}
	if _, err := sshRun(env, pi, "Command", "true"); err != nil || srv.Shells.Load() != 3 {
		t.Errorf("after the timeout: %v, %d shells", err, srv.Shells.Load())
	}
	time.Sleep(time.Second)
	if _, err := sshRun(env, pi, "Command", "true"); err != nil || srv.Shells.Load() != 4 {
		t.Errorf("after Close After: %v, %d shells", err, srv.Shells.Load())
	}
}

// What is wrong is said before a shell opens.
func TestSSHExecFails(t *testing.T) {
	_, env, _ := sshEnv(t)
	for _, c := range []struct {
		session, values []string
		want            string
	}{
		{nil, []string{"Command", "true"}, "Connection: pick an SSH connection"},
		{[]string{"Connection", "9"}, []string{"Command", "true"}, "connection 9 is gone"},
		{[]string{"Connection", "1", "Close After", "soon"}, []string{"Command", "true"}, `Close After "soon"`},
		{[]string{"Connection", "1"}, []string{"Command", ""}, "Command is empty"},
	} {
		if _, err := sshRun(env, c.session, c.values...); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v %v: %v, want %q", c.session, c.values, err, c.want)
		}
	}
	if _, err := run(t, env, "ssh-exec", "Command", "true"); err == nil || !strings.Contains(err.Error(), "link an SSH Session") {
		t.Errorf("no session: %v", err)
	}
}

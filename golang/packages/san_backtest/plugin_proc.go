package san_backtest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wargasipil/san_backtest/internal/pipe"
	"github.com/wargasipil/san_backtest/plugin/pb"
	"google.golang.org/protobuf/encoding/protodelim"
)

// Approach plugins (the user, 2026-11-16: "I think, backtest and live
// library can load 2 mode, first approach is from code, and approach from
// dll, like approach: Load approach(dllfile)"). A plugin is a separate
// program that serves approaches over its stdin and stdout (the user,
// 2026-11-16: "Stdin and out seem better"), not a DLL: Go cannot load or
// unload Go DLLs on Windows, and a crash in a separate process cannot take
// the host down. Package plugin is the plugin side; this file is the host's.

// PluginProtocol is the version of the plugin wire protocol
// (plugin/pb/plugin.proto); PluginAPI the version of what a plugin's
// Context carries. Host and plugin must agree on both.
const (
	PluginProtocol = 1
	PluginAPI      = 1
)

// DefaultPluginTimeout is how long the host waits for one answer from a
// plugin before it stops the plugin and fails the run.
const DefaultPluginTimeout = 10 * time.Second

// PluginOptions are how a host runs a plugin's process.
type PluginOptions struct {
	// Stderr gets the plugin's log (its stderr); nil = this process's
	// stderr.
	Stderr io.Writer
	// Timeout is the longest wait for one answer; 0 = DefaultPluginTimeout.
	Timeout time.Duration
	// Env are extra environment entries ("KEY=value") for the process,
	// after this process's own.
	Env []string
	// CleanEnv starts the process with only the system's basic variables
	// (PATH, TEMP, SystemRoot, ...) plus Env, so nothing the host holds in
	// its environment (an API key) reaches the plugin.
	CleanEnv bool
}

// cleanEnvKeys are the variables CleanEnv keeps (case-insensitive).
var cleanEnvKeys = map[string]bool{"PATH": true, "PATHEXT": true, "SYSTEMROOT": true, "SYSTEMDRIVE": true, "WINDIR": true, "COMSPEC": true,
	"TEMP": true, "TMP": true, "TMPDIR": true, "HOME": true, "USERPROFILE": true, "LOCALAPPDATA": true, "APPDATA": true, "PROGRAMDATA": true,
	"NUMBER_OF_PROCESSORS": true, "PROCESSOR_ARCHITECTURE": true, "OS": true, "LANG": true, "TZ": true, "USER": true, "USERNAME": true}

func environment(o PluginOptions) []string {
	if !o.CleanEnv {
		if len(o.Env) == 0 {
			return nil
		}
		return append(os.Environ(), o.Env...)
	}
	out := []string{} // not nil: a nil Env would pass on everything
	for _, kv := range os.Environ() {
		if k, _, ok := strings.Cut(kv, "="); ok && cleanEnvKeys[strings.ToUpper(k)] {
			out = append(out, kv)
		}
	}
	return append(out, o.Env...)
}

// PluginApproach is one approach a plugin file serves.
type PluginApproach struct {
	Name string // served under
	// StrategyName is the approach's own Name() (usually Name).
	StrategyName string
	File         string
	Timeframes   []string
	Warmup       int
	Params       map[string]float64 // nil when the approach is not Tunable
	MaxOpen      int                // 0 when it has no MaxOpen of its own
	HasOnStart   bool
	HasGrid      bool
	// ModTime is the file's modification time when its process started.
	ModTime time.Time
}

// pluginProc is one running plugin process. Calls are one at a time.
type pluginProc struct {
	file    string
	timeout time.Duration
	cmd     *exec.Cmd
	in      io.WriteCloser
	inBuf   *bufio.Writer
	out     *bufio.Reader
	kill    func() // stops the process and anything it started
	exited  chan struct{}
	waitErr error
	tail    *tailBuffer
	modTime time.Time

	mu         sync.Mutex
	dead       error // set once the process is unusable
	approaches []PluginApproach
}

// tailBuffer keeps the last bytes of the plugin's stderr for error texts,
// passing everything on to w.
type tailBuffer struct {
	mu  sync.Mutex
	w   io.Writer
	buf []byte
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > 4096 {
		t.buf = append([]byte{}, t.buf[len(t.buf)-4096:]...)
	}
	t.mu.Unlock()
	if t.w != nil {
		t.w.Write(p)
	}
	return len(p), nil
}

// last is the last non-empty line of stderr, "" when none.
func (t *tailBuffer) last() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := bytes.Split(bytes.TrimRight(t.buf, "\r\n\t "), []byte("\n"))
	return string(bytes.TrimSpace(lines[len(lines)-1]))
}

// startPlugin starts file, checks the versions and reads its approach
// list.
func startPlugin(file string, o PluginOptions) (*pluginProc, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("san_backtest: plugin %s: %w", file, err)
	}
	// The process runs a copy, so the file can be rebuilt while it runs
	// (Windows cannot overwrite a running .exe).
	run, err := shadowCopy(abs)
	if err != nil {
		return nil, fmt.Errorf("san_backtest: plugin %s: %w", file, err)
	}
	p := &pluginProc{file: abs, timeout: o.Timeout, exited: make(chan struct{}), modTime: fi.ModTime()}
	if p.timeout <= 0 {
		p.timeout = DefaultPluginTimeout
	}
	stderr := o.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	p.tail = &tailBuffer{w: stderr}
	cmd := exec.Command(run)
	cmd.Dir = filepath.Dir(abs)
	cmd.Env = environment(o)
	cmd.Stderr = p.tail
	hideWindow(cmd)
	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		os.Remove(run)
		// The error names the temporary copy; say the file instead (e.g.
		// "This version of %1 is not compatible ..." for a file that is no
		// program).
		var pe *os.PathError
		if errors.As(err, &pe) {
			err = pe.Err
		}
		name := filepath.Base(abs)
		return nil, fmt.Errorf("san_backtest: plugin %s cannot start: %s", name, strings.ReplaceAll(err.Error(), "%1", name))
	}
	p.cmd, p.in, p.inBuf, p.out = cmd, in, bufio.NewWriterSize(pipe.Writer(in), 1<<16), bufio.NewReaderSize(pipe.Reader(out), 1<<16)
	p.kill = killTree(cmd)
	go func() {
		p.waitErr = cmd.Wait()
		for range 20 { // the image is released a moment after the exit
			if e := os.Remove(run); e == nil || errors.Is(e, os.ErrNotExist) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		close(p.exited)
	}()
	r, err := p.call(&pb.Request{Call: &pb.Request_Hello{Hello: &pb.HelloRequest{Protocol: PluginProtocol, Api: PluginAPI}}})
	if err != nil {
		p.stop()
		return nil, err
	}
	h := r.GetHello()
	if h.GetProtocol() != PluginProtocol || h.GetApi() != PluginAPI {
		p.stop()
		return nil, fmt.Errorf("san_backtest: plugin %s built with protocol %d (API %d), host speaks %d (API %d): rebuild the plugin",
			filepath.Base(abs), h.GetProtocol(), h.GetApi(), PluginProtocol, PluginAPI)
	}
	r, err = p.call(&pb.Request{Call: &pb.Request_List{List: &pb.ListRequest{}}})
	if err != nil {
		p.stop()
		return nil, err
	}
	for _, a := range r.GetList().GetApproaches() {
		pa := PluginApproach{Name: a.GetName(), StrategyName: a.GetStrategyName(), File: abs, Timeframes: a.GetTimeframes(), Warmup: int(a.GetWarmup()), HasOnStart: a.GetHasOnStart(),
			HasGrid: a.GetHasGrid(), ModTime: p.modTime}
		if a.GetHasParams() {
			pa.Params = map[string]float64{}
			for k, v := range a.GetParams() {
				pa.Params[k] = v
			}
		}
		if a.GetHasMaxOpen() {
			pa.MaxOpen = int(a.GetMaxOpen())
		}
		p.approaches = append(p.approaches, pa)
	}
	return p, nil
}

var shadowN atomic.Int64

// sweepShadows removes copies left behind by hosts that were killed
// before they could remove theirs. A copy still running is locked by
// Windows and stays; an hour old keeps clear of copies being started.
var sweepShadows sync.Once

// shadowCopy copies the plugin file to the temp folder under a name of its
// own; the copy is removed when its process ends.
func shadowCopy(file string) (string, error) {
	dir := filepath.Join(os.TempDir(), "san_backtest_plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	sweepShadows.Do(func() {
		es, _ := os.ReadDir(dir)
		for _, e := range es {
			if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > time.Hour {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	})
	ext := filepath.Ext(file)
	name := fmt.Sprintf("%s-%d-%d%s", strings.TrimSuffix(filepath.Base(file), ext), os.Getpid(), shadowN.Add(1), ext)
	dst := filepath.Join(dir, name)
	src, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer src.Close()
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(dst)
		return "", err
	}
	return dst, f.Close()
}

// call sends one request and waits for its answer, at most the timeout;
// a timeout or a broken pipe stops the process for good.
func (p *pluginProc) call(req *pb.Request) (*pb.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.dead != nil {
		return nil, p.dead
	}
	name := filepath.Base(p.file)
	timedOut := false
	var tmu sync.Mutex
	timer := time.AfterFunc(p.timeout, func() {
		tmu.Lock()
		timedOut = true
		tmu.Unlock()
		p.kill()
	})
	res := &pb.Response{}
	_, err := protodelim.MarshalTo(p.inBuf, req)
	if err == nil {
		err = p.inBuf.Flush()
	}
	if err == nil {
		err = protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(p.out, res)
	}
	timer.Stop()
	tmu.Lock()
	to := timedOut
	tmu.Unlock()
	if to {
		p.dead = fmt.Errorf("san_backtest: plugin %s gave no answer in %s: stopped it", name, p.timeout)
		return nil, p.dead
	}
	if err != nil {
		p.dead = p.exitError(err)
		p.kill()
		return nil, p.dead
	}
	if e := res.GetError(); e != "" {
		return nil, fmt.Errorf("san_backtest: plugin %s: %s", name, e)
	}
	return res, nil
}

// exitError explains a broken pipe: the process's exit and its last log
// line when it ended.
func (p *pluginProc) exitError(err error) error {
	name := filepath.Base(p.file)
	select {
	case <-p.exited:
	case <-time.After(2 * time.Second):
		return fmt.Errorf("san_backtest: plugin %s: %v", name, err)
	}
	msg := fmt.Sprintf("san_backtest: plugin %s exited", name)
	if p.waitErr != nil {
		msg += " (" + p.waitErr.Error() + ")"
	}
	if l := p.tail.last(); l != "" {
		msg += ": " + l
	}
	return errors.New(msg)
}

// stop ends the process: Shutdown when no call is going on, then a kill
// after a second. A call that is going on fails at once.
func (p *pluginProc) stop() {
	if p.mu.TryLock() {
		if p.dead == nil {
			p.dead = fmt.Errorf("san_backtest: plugin %s is closed", filepath.Base(p.file))
			go func() {
				protodelim.MarshalTo(p.in, &pb.Request{Call: &pb.Request_Shutdown{Shutdown: &pb.Empty{}}})
				p.in.Close()
			}()
			select {
			case <-p.exited:
			case <-time.After(time.Second):
			}
		}
		p.mu.Unlock()
	}
	p.kill()
	<-p.exited
}

// newInstance makes a fresh approach in the plugin.
func (p *pluginProc) newInstance(name string) (uint64, error) {
	r, err := p.call(&pb.Request{Call: &pb.Request_NewInstance{NewInstance: &pb.NewInstanceRequest{Name: name}}})
	if err != nil {
		return 0, err
	}
	return r.GetNewInstance().GetId(), nil
}

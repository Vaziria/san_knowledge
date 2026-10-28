package engine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// bashKinds is Bash (the user, 2026-10-27: "Add node bash for executing
// shell and it can connect to storage as working directory").
var bashKinds = []*kind{
	{
		StepKind: &v1.StepKind{
			Kind: "bash", Title: "Bash", Category: "Action",
			Description: "Runs Script with bash in a storage's folder when one is picked, else in the tower's data folder, with Input on its standard input; Stdout, Stderr and Exit Code are what it gave. An exit code other than 0 fails the step (Failed, with Error), unless Non-zero Exit is Goes On. It is stopped, with what it started, after Timeout (5m). On Windows it is Git Bash's bash.",
			Outputs:     []*v1.Socket{sock("Done", Event), sock("Stdout", Text), sock("Stderr", Text), sock("Exit Code", Text)},
			Settings: []*v1.Setting{
				{Name: "Storage", Storage: true, Placeholder: "No storage: the data folder"},
				{Name: "Timeout", Placeholder: "5m"},
				{Name: "Non-zero Exit", Options: []string{"Fails", "Goes On"}},
			},
			Inputs: []*v1.Socket{sock("Event", Event), field("Script"), field("Input")},
		},
		run: bashStep,
	},
}

// What a Bash step keeps of a script's output, each of stdout and stderr:
// past it, the rest is counted and left out.
const bashOutputMax = 1 << 20

func bashStep(c *call) error {
	script := c.text("Script")
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("Script %w", errEmpty)
	}
	dir, err := c.workDir()
	if err != nil {
		return err
	}
	timeout, err := c.timeout()
	if err != nil {
		return err
	}
	res, err := runScript(c.ctx, script, c.text("Input"), dir, timeout, nil)
	if err != nil {
		return err
	}
	if res.timedOut {
		return fmt.Errorf("stopped after the Timeout, %s%s", timeout, tail(res.stderr))
	}
	c.outputs["Stdout"] = stringValue(trimNewline(res.stdout))
	c.outputs["Stderr"] = stringValue(trimNewline(res.stderr))
	c.outputs["Exit Code"] = stringValue(strconv.Itoa(res.code))
	c.logs = append(c.logs, fmt.Sprintf("exit %d after %s, in %s", res.code, res.took, filepath.ToSlash(dir)))
	if res.code != 0 && c.setting("Non-zero Exit") != "Goes On" {
		return fmt.Errorf("exit code %d%s", res.code, tail(res.stderr))
	}
	c.fire("Done")
	return nil
}

// timeout is the step's Timeout setting, 5m when not set.
func (c *call) timeout() (time.Duration, error) {
	s := c.setting("Timeout")
	if s == "" {
		return 5 * time.Minute, nil
	}
	timeout, err := time.ParseDuration(s)
	if err != nil || timeout <= 0 || timeout > 24*time.Hour {
		return 0, fmt.Errorf("Timeout %q: give a duration such as 30s, 5m or 2h, up to 24h", s)
	}
	return timeout, nil
}

// scriptResult is what a script gave: its output, its exit code, and how
// long it took; timedOut when it was stopped after its timeout.
type scriptResult struct {
	stdout, stderr string
	code           int
	took           time.Duration
	timedOut       bool
}

// runScript runs script with bash in dir, with input on its standard
// input, and stops it, with what it started, after timeout. An exit code
// other than 0 is not an error; ctx ending is. When onLine is set, it gets
// each line of stdout and stderr as it comes (from two goroutines).
func runScript(ctx context.Context, script, input, dir string, timeout time.Duration, onLine func(string)) (scriptResult, error) {
	bash, err := bashPath()
	if err != nil {
		return scriptResult{}, err
	}
	// The script goes through a file, not -c: on Windows a command line
	// with quotes and newlines does not come out whole on the other side.
	f, err := os.CreateTemp("", "san_tower-*.sh")
	if err != nil {
		return scriptResult{}, err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(strings.ReplaceAll(script, "\r\n", "\n"))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return scriptResult{}, err
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(tctx, bash, filepath.ToSlash(f.Name()))
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr cappedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if onLine != nil {
		outLines, errLines := &lineWriter{line: onLine}, &lineWriter{line: onLine}
		cmd.Stdout, cmd.Stderr = io.MultiWriter(&stdout, outLines), io.MultiWriter(&stderr, errLines)
		defer outLines.flush()
		defer errLines.flush()
	}
	// A program the script left running may keep the pipes open: Wait
	// gives up on them a moment after the script itself has ended.
	cmd.WaitDelay = 5 * time.Second
	start := time.Now()
	err = runTree(cmd)
	res := scriptResult{took: time.Since(start).Round(time.Millisecond)}

	var exit *exec.ExitError
	switch {
	case tctx.Err() == context.DeadlineExceeded && ctx.Err() == nil:
		res.timedOut = true
	case ctx.Err() != nil:
		return res, ctx.Err()
	case errors.As(err, &exit):
		res.code = exit.ExitCode()
	case errors.Is(err, exec.ErrWaitDelay):
		// The script ended; something it started still held the output.
	case err != nil:
		return res, err
	}
	// Output that is not UTF-8 (a Windows program in its console code page)
	// has U+FFFD for its bytes: proto strings must be UTF-8 (2026-10-28).
	res.stdout, res.stderr = strings.ToValidUTF8(stdout.String(), "�"), strings.ToValidUTF8(stderr.String(), "�")
	return res, nil
}

// workDir is the folder of the step's storage, or the tower's data folder
// when none is picked.
func (c *call) workDir() (string, error) {
	if c.setting("Storage") == "" {
		if c.env.DataDir == "" {
			return "", errors.New("this run has no data folder: pick a storage")
		}
		return filepath.FromSlash(c.env.DataDir), nil
	}
	st, err := c.storage()
	if err != nil {
		return "", err
	}
	dir := filepath.FromSlash(st.Abs)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("storage %s: its folder %s is not there", st.Name, st.Abs)
	}
	return dir, nil
}

// bashPath finds bash: SAN_TOWER_BASH when set. On Windows it is Git
// Bash's, beside git or where Git for Windows installs it, never the
// bash.exe of System32 or WindowsApps, which starts WSL (2026-10-27: a
// cmd.exe's PATH finds that one first).
func bashPath() (string, error) {
	if p := os.Getenv("SAN_TOWER_BASH"); p != "" {
		return p, nil
	}
	if runtime.GOOS != "windows" {
		p, err := exec.LookPath("bash")
		if err != nil {
			return "", errors.New("bash is not on this computer")
		}
		return p, nil
	}
	var tries []string
	if git, err := exec.LookPath("git"); err == nil {
		root := filepath.Dir(filepath.Dir(git)) // ...\Git\cmd\git.exe
		tries = append(tries, filepath.Join(root, "bin", "bash.exe"), filepath.Join(root, "usr", "bin", "bash.exe"))
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if d := os.Getenv(env); d != "" {
			tries = append(tries, filepath.Join(d, "Git", "bin", "bash.exe"), filepath.Join(d, "Programs", "Git", "bin", "bash.exe"))
		}
	}
	if p, err := exec.LookPath("bash"); err == nil {
		low := strings.ToLower(p)
		if !strings.Contains(low, `\system32\`) && !strings.Contains(low, `\windowsapps\`) {
			tries = append(tries, p)
		}
	}
	for _, p := range tries {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("Git Bash is not on this computer: install Git for Windows, or set SAN_TOWER_BASH to a bash.exe")
}

// cappedBuffer keeps the first bashOutputMax bytes written to it, and says
// how many more there were.
type cappedBuffer struct {
	mu   sync.Mutex
	b    []byte
	more int
}

func (w *cappedBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	room := bashOutputMax - len(w.b)
	if room >= len(p) {
		w.b = append(w.b, p...)
	} else {
		w.b = append(w.b, p[:max(room, 0)]...)
		w.more += len(p) - max(room, 0)
	}
	return len(p), nil
}

func (w *cappedBuffer) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.more > 0 {
		return string(w.b) + fmt.Sprintf("\n(%d more bytes left out)", w.more)
	}
	return string(w.b)
}

// lineWriter passes each line written to it on to line, without its line
// break, as it comes. A \r ends a line too, so a progress bar redrawn in
// place (ffmpeg, npm) shows each time it moves; empty lines are left out.
// It is locked: a program the script left running may still write as the
// script's end flushes it.
type lineWriter struct {
	line func(string)
	mu   sync.Mutex
	buf  []byte
}

// lineMax is the longest line a lineWriter waits for: past it, the line is
// passed on in pieces.
const lineMax = 4096

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexAny(w.buf, "\r\n")
		if i < 0 {
			break
		}
		if i > 0 {
			w.line(string(w.buf[:i]))
		}
		w.buf = w.buf[i+1:]
	}
	if len(w.buf) >= lineMax {
		w.line(string(w.buf))
		w.buf = w.buf[:0]
	}
	return len(p), nil
}

// flush passes on the last line, when it had no line break.
func (w *lineWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.line(string(w.buf))
		w.buf = nil
	}
}

func trimNewline(s string) string { return strings.TrimRight(s, "\r\n") }

// tail is the last lines of stderr, for an error: ": <them>", or "".
func tail(stderr string) string {
	s := strings.TrimSpace(stderr)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	s = strings.Join(lines, "\n")
	if r := []rune(s); len(r) > 1000 {
		s = "…" + string(r[len(r)-1000:])
	}
	return ": " + s
}

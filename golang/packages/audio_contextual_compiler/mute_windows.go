package audio_contextual_compiler

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// mute_windows.ps1 mutes Sonic Pi's audio engine in Windows' volume mixer: the
// audio sessions of the "Sonic Pi - SuperSonic" processes started since a
// time, so a Sonic Pi the user has open is left alone. It mutes them as they
// come until its standard input ends, then unmutes them.
//
//go:embed mute_windows.ps1
var muteScript string

// engineProcess is the name of Sonic Pi 5's audio engine process.
const engineProcess = "Sonic Pi - SuperSonic"

// muteEngine silences what a render plays (the user, 2026-10-29: "Silent
// rendering"): Sonic Pi's recording is taken inside its engine, before the
// sound device, so muting the engine's audio session in Windows' volume
// mixer keeps the speakers quiet and the take as it is. unmute ends it and
// waits; call it while the engine still runs, since Windows remembers an
// app's mute for the next time it starts.
func muteEngine(since time.Time, log io.Writer) (unmute func(), err error) {
	dir, err := os.MkdirTemp("", "ctxcompiler-mute-")
	if err != nil {
		return nil, err
	}
	script := filepath.Join(dir, "mute.ps1")
	if err := os.WriteFile(script, []byte(muteScript), 0o644); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-File", script, "-Name", engineProcess, "-SinceUnixMs", strconv.FormatInt(since.UnixMilli(), 10))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("start PowerShell to mute Sonic Pi: %w", err)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			stdin.Close()
			done := make(chan struct{})
			go func() { cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(20 * time.Second):
				cmd.Process.Kill()
				fmt.Fprintf(log, "unmuting Sonic Pi took too long: if it stays silent, unmute %q in the volume mixer\n", engineProcess)
			}
			os.RemoveAll(dir)
		})
	}, nil
}

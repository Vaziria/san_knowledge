package audio_contextual_compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// SonicPi is what the compiler knows about an installed Sonic Pi: the names
// of its synths, FX and samples, which pass 2 gets so that it does not invent
// any, and its Ruby, for a syntax check of the code.
type SonicPi struct {
	Dir     string   `json:"-"`
	Version string   `json:"version"`
	Synths  []string `json:"synths"`
	FX      []string `json:"fx"`
	Samples []string `json:"samples"`
	Ruby    string   `json:"-"` // "" skips the syntax check
}

// defaultSonicPiDirs are where Sonic Pi installs itself.
var defaultSonicPiDirs = []string{
	`C:\Program Files\Sonic Pi`,
	"/Applications/Sonic Pi.app/Contents/Resources",
	"/opt/sonic-pi",
	"/usr/lib/sonic-pi",
}

// FindSonicPi loads the Sonic Pi in dir or, when dir is empty, in
// $SONIC_PI_HOME or the first usual install folder that has one. It returns
// nil and no error when there is none.
func FindSonicPi(dir string) (*SonicPi, error) {
	if dir == "" {
		dir = os.Getenv("SONIC_PI_HOME")
	}
	if dir != "" {
		return LoadSonicPi(dir)
	}
	for _, d := range defaultSonicPiDirs {
		if _, err := os.Stat(synthInfoPath(d)); err == nil {
			return LoadSonicPi(d)
		}
	}
	return nil, nil
}

func synthInfoPath(dir string) string {
	return filepath.Join(dir, "app", "server", "ruby", "lib", "sonicpi", "synths", "synthinfo.rb")
}

var sampleExts = []string{".flac", ".wav", ".aif", ".aiff", ".ogg"}

// LoadSonicPi reads the Sonic Pi installed in dir.
func LoadSonicPi(dir string) (*SonicPi, error) {
	info, err := os.ReadFile(synthInfoPath(dir))
	if err != nil {
		return nil, fmt.Errorf("%s is not a Sonic Pi folder: %w", dir, err)
	}
	sp := &SonicPi{Dir: dir}
	sp.Synths, sp.FX = synthNames(string(info))
	if len(sp.Synths) == 0 {
		return nil, fmt.Errorf("found no synth names in %s", synthInfoPath(dir))
	}
	if v, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		sp.Version = strings.TrimSpace(string(v))
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "etc", "samples"))
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if !e.IsDir() && slices.Contains(sampleExts, strings.ToLower(ext)) {
			sp.Samples = append(sp.Samples, strings.TrimSuffix(e.Name(), ext))
		}
	}
	slices.Sort(sp.Samples)
	sp.Samples = slices.Compact(sp.Samples)

	ruby := filepath.Join(dir, "app", "server", "native", "ruby", "bin", "ruby")
	if runtime.GOOS == "windows" {
		ruby += ".exe"
	}
	if _, err := os.Stat(ruby); err == nil {
		sp.Ruby = ruby
	} else if path, err := exec.LookPath("ruby"); err == nil {
		sp.Ruby = path
	}
	return sp, nil
}

var synthEntry = regexp.MustCompile(`(?m)^\s*:(\w+)\s*=>`)

// synthNames reads the synth and FX names from Sonic Pi's synthinfo.rb: the
// keys of its @@synth_infos table, without its internal players and mixers.
// FX keys start with "fx_", which Sonic Pi code leaves out.
func synthNames(src string) (synths, fx []string) {
	i := strings.Index(src, "@@synth_infos =")
	if i < 0 {
		return nil, nil
	}
	block := src[i:]
	if j := strings.Index(block, "}"); j >= 0 {
		block = block[:j]
	}
	for _, m := range synthEntry.FindAllStringSubmatch(block, -1) {
		name := m[1]
		switch {
		case strings.HasPrefix(name, "fx_replace_"):
		case strings.HasPrefix(name, "fx_"):
			fx = append(fx, strings.TrimPrefix(name, "fx_"))
		case strings.HasSuffix(name, "_player"), strings.HasSuffix(name, "_mixer"):
		default:
			synths = append(synths, name)
		}
	}
	slices.Sort(synths)
	slices.Sort(fx)
	return synths, fx
}

var (
	// stringOrComment is Sonic Pi's own preparser pattern.
	stringOrComment = regexp.MustCompile(`'(?:\\.|[^'\\])*'|"(?:\\.|[^"\\])*"|#[^\n]*`)
	liveLoop        = regexp.MustCompile(`\blive_loop\b`)
	synthUse        = regexp.MustCompile(`\b(?:synth|use_synth|with_synth)\b[\s(]*:(\w+)`)
	fxUse           = regexp.MustCompile(`\bwith_fx\b[\s(]*:(\w+)`)
	sampleUse       = regexp.MustCompile(`\b(?:sample|sample_duration|use_sample_bpm)\b[\s(]*:(\w+)`)
)

// vecFns are the functions Sonic Pi 5's preparser rewrites (those returning a
// ring, vector or ramp). Sonic Pi refuses them as variable names.
var vecFns = []string{"bools", "chord", "chord_degree", "chord_invert", "doubles", "halves", "knit", "line",
	"midi_notes", "note_range", "octs", "ramp", "range", "ring", "sample_names", "sample_paths", "scale",
	"spread", "stretch", "vector"}

var vecFnAssign = regexp.MustCompile(`\b(` + strings.Join(vecFns, "|") + `)\s*=[\s\w]`)

// mask blanks string literals and comments, keeping newlines, so that the
// checks only look at code.
func mask(code string) string {
	return stringOrComment.ReplaceAllStringFunc(code, func(m string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, m)
	})
}

// CheckCode returns the problems of pass 2's code for graph g, nothing when
// it is fine. sp adds the checks that need an installed Sonic Pi; it may be
// nil. The messages go back to Claude, so they say how to fix the code.
func CheckCode(ctx context.Context, code string, g *Graph, sp *SonicPi) []string {
	if strings.TrimSpace(code) == "" {
		return []string{"the code is empty"}
	}
	var problems []string
	masked := mask(code)
	if liveLoop.MatchString(masked) {
		problems = append(problems, "live_loop never ends, and the piece must: use in_thread with counted loops")
	}
	var missing []string
	for _, n := range g.Nodes {
		if n.Type == TypeSong {
			continue
		}
		marker := regexp.MustCompile(`(?m)^[ \t]*#[ \t]*node:[ \t]*` + regexp.QuoteMeta(n.ID) + `[ \t]*\r?$`)
		if !marker.MatchString(code) {
			missing = append(missing, n.ID)
		}
	}
	if len(missing) > 0 {
		problems = append(problems, `there is no "# node: <id>" comment for `+strings.Join(missing, ", "))
	}
	var vars []string
	for _, m := range vecFnAssign.FindAllStringSubmatch(masked, -1) {
		if !slices.Contains(vars, m[1]) {
			vars = append(vars, m[1])
			problems = append(problems, fmt.Sprintf("%s is a Sonic Pi function and cannot be a variable name", m[1]))
		}
	}
	if sp != nil {
		problems = append(problems, sp.unknownNames(masked)...)
		if p := sp.syntax(ctx, code); p != "" {
			problems = append(problems, p)
		}
	}
	return problems
}

// unknownNames lists the synths, FX and samples in code that this Sonic Pi
// does not have.
func (sp *SonicPi) unknownNames(code string) []string {
	var problems []string
	for _, k := range []struct {
		what, field string
		use         *regexp.Regexp
		names       []string
	}{
		{"synth", "synths", synthUse, sp.Synths},
		{"FX", "fx", fxUse, sp.FX},
		{"sample", "samples", sampleUse, sp.Samples},
	} {
		var unknown []string
		for _, m := range k.use.FindAllStringSubmatch(code, -1) {
			if !slices.Contains(k.names, m[1]) && !slices.Contains(unknown, m[1]) {
				unknown = append(unknown, m[1])
			}
		}
		for _, name := range unknown {
			problems = append(problems, fmt.Sprintf("Sonic Pi %s has no %s :%s; pick one from sonic_pi.%s", sp.Version, k.what, name, k.field))
		}
	}
	return problems
}

// Record plays the Sonic Pi code in codePath without Sonic Pi's GUI and
// records its first d to a WAV file. Recording is real time. Unless audible,
// nothing is heard: Sonic Pi's engine is muted in the volume mixer while it
// records (muteEngine), which does not change the take. Sonic Pi's output
// goes to log.
//
// The code runs in the headless recorder Sonic Pi 5 ships, which keeps the
// mixer on, but the recording is made by Sonic Pi's own recording_start and
// recording_save: on Windows the recorder's record tap writes nothing (Sonic
// Pi 5.0.0), and recording_save needs a moment after recording_stop, or the
// file is still in use.
func (sp *SonicPi) Record(ctx context.Context, codePath, wavPath string, d time.Duration, audible bool, log io.Writer) error {
	recorder := filepath.Join(sp.Dir, "app", "server", "ruby", "bin", "headless-record.rb")
	if _, err := os.Stat(recorder); err != nil {
		return fmt.Errorf("Sonic Pi %s has no headless recorder (Sonic Pi 5 has app/server/ruby/bin/headless-record.rb)", sp.Version)
	}
	if sp.Ruby == "" {
		return fmt.Errorf("no Ruby to run Sonic Pi's headless recorder with")
	}
	if d <= 0 {
		return fmt.Errorf("the duration to record must be more than 0")
	}
	code, err := os.ReadFile(codePath)
	if err != nil {
		return err
	}
	wavPath, err = filepath.Abs(wavPath)
	if err != nil {
		return err
	}
	// Sonic Pi saves to a name of its own, so a failed take leaves an old
	// file at wavPath as it was.
	take := strings.TrimSuffix(wavPath, filepath.Ext(wavPath)) + ".recording.wav"
	os.Remove(take)

	tmp, err := os.MkdirTemp("", "ctxcompiler-render-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	script := filepath.Join(tmp, "take.rb")
	if err := os.WriteFile(script, []byte(recordingScript(string(code), take, d, !audible)), 0o644); err != nil {
		return err
	}
	if !audible {
		unmute, err := muteEngine(time.Now(), log)
		if err != nil {
			return fmt.Errorf("silent rendering: %w (render with --audible to hear it as it records)", err)
		}
		defer unmute()
		// Unmute once the take is saved, while the engine still runs (the
		// script has turned Sonic Pi's own volume down by then): Windows
		// remembers an app's mute for the next time it starts.
		saved := make(chan struct{})
		defer close(saved)
		go func() {
			for {
				select {
				case <-saved:
					return
				case <-time.After(250 * time.Millisecond):
				}
				if info, err := os.Stat(take); err == nil && info.Size() > 0 {
					time.Sleep(time.Second)
					unmute()
					return
				}
			}
		}()
	}
	// The recorder stops everything after its -d: leave time to save.
	run := d + 10*time.Second
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, sp.Ruby, recorder,
		"-o", filepath.Join(tmp, "tap.wav"), "-d", strconv.FormatFloat(run.Seconds(), 'f', -1, 64), "-f", script)
	cmd.Stdout = io.MultiWriter(log, &out)
	cmd.Stderr = cmd.Stdout
	// Its exit status is about its own record tap, not the take.
	if err := cmd.Run(); err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	if info, err := os.Stat(take); err != nil || info.Size() == 0 {
		return fmt.Errorf("Sonic Pi saved no recording (see its output above)")
	}
	if err := os.Rename(take, wavPath); err != nil {
		return err
	}
	if strings.Contains(out.String(), "ERRORS:") {
		return fmt.Errorf("%s was written, but the code had errors while it played (see above)", wavPath)
	}
	return nil
}

// recordingScript puts a recording of the first d around code, saved to wav.
// Its thread keeps its own tempo, so its sleep is in seconds whatever tempo
// the code uses. A silent take then turns Sonic Pi's volume down, so the rest
// of the code plays unheard once its engine is unmuted.
func recordingScript(code, wav string, d time.Duration, silent bool) string {
	quiet := ""
	if silent {
		quiet = "  set_volume! 0\n"
	}
	return fmt.Sprintf(`recording_start
in_thread do
  use_bpm 60
  sleep %s
  recording_stop
  sleep 3
  recording_save %q
%send

`, strconv.FormatFloat(d.Seconds(), 'f', -1, 64), filepath.ToSlash(wav), quiet) + code
}

// syntax runs Ruby's syntax check on code. Sonic Pi code is Ruby, so a
// missing "end" shows up here. It reports nothing when Ruby cannot be run.
func (sp *SonicPi) syntax(ctx context.Context, code string) string {
	if sp.Ruby == "" {
		return ""
	}
	f, err := os.CreateTemp("", "ctxcompiler-*.rb")
	if err != nil {
		return ""
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(code)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, sp.Ruby, "-c", f.Name()).CombinedOutput()
	var exit *exec.ExitError
	if err == nil || !errors.As(err, &exit) || ctx.Err() != nil {
		return ""
	}
	msg := strings.NewReplacer(
		sp.Ruby+": ", "",
		f.Name(), "code.rb",
		filepath.ToSlash(f.Name()), "code.rb",
		"\r\n", "\n",
	).Replace(strings.TrimSpace(string(out)))
	return "Ruby's syntax check fails:\n" + msg
}

package audio_contextual_compiler

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeSonicPi writes the files LoadSonicPi reads into a temp folder.
func fakeSonicPi(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	info := synthInfoPath(dir)
	files := map[string]string{
		info: `module SonicPi
  class SynthInfo
      @@synth_infos =
        {
        :dull_bell => DullBell.new,
        :pluck => Pluck.new,
        :mono_player => MonoPlayer.new,
        :main_mixer => MainMixer.new,
        :fx_reverb => FXReverb.new,
        :fx_replace_reverb => FXReverb.new,
        :fx_echo => FXEcho.new,
      }
      def self.other
        { :not_a_synth => 1 }
      end
  end
end
`,
		filepath.Join(dir, "VERSION"):                          "5.0.0\n",
		filepath.Join(dir, "etc", "samples", "bd_haus.flac"):   "",
		filepath.Join(dir, "etc", "samples", "ambi_drone.WAV"): "",
		filepath.Join(dir, "etc", "samples", "README.md"):      "",
	}
	for path, text := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadSonicPi(t *testing.T) {
	sp, err := LoadSonicPi(fakeSonicPi(t))
	if err != nil {
		t.Fatal(err)
	}
	if sp.Version != "5.0.0" {
		t.Errorf("version %q", sp.Version)
	}
	if want := []string{"dull_bell", "pluck"}; !reflect.DeepEqual(sp.Synths, want) {
		t.Errorf("synths %v, want %v", sp.Synths, want)
	}
	if want := []string{"echo", "reverb"}; !reflect.DeepEqual(sp.FX, want) {
		t.Errorf("fx %v, want %v", sp.FX, want)
	}
	if want := []string{"ambi_drone", "bd_haus"}; !reflect.DeepEqual(sp.Samples, want) {
		t.Errorf("samples %v, want %v", sp.Samples, want)
	}
	if _, err := LoadSonicPi(t.TempDir()); err == nil {
		t.Error("an empty folder loaded as Sonic Pi")
	}
}

// folkCode is pass 2 code for folkGraph that passes every check.
const folkCode = `use_bpm 70
use_random_seed 1

# node: song.reverb
with_fx :reverb, room: 0.8 do
  # node: intro
  define :intro do
    # node: intro.guitar
    in_thread do
      notes = ring(60, 62.4, 64.8) # pelog-ish
      8.times do
        synth :pluck, note: notes.tick
        sleep 0.5
      end
    end
    sleep 4
  end

  # node: climax
  define :climax do
    # node: climax.kendang
    in_thread do
      8.times do
        sample :bd_haus
        sleep 0.5
      end
      cue :speed_up
    end
    # node: climax.bonang
    in_thread do
      sync :speed_up
      synth :dull_bell, note: 72
    end
    sleep 4
  end

  intro
  climax
end
`

func TestCheckCode(t *testing.T) {
	sp, err := LoadSonicPi(fakeSonicPi(t))
	if err != nil {
		t.Fatal(err)
	}
	sp.Ruby = "" // syntax: TestRubySyntax
	g := folkGraph(t)
	ctx := context.Background()
	if p := CheckCode(ctx, folkCode, g, sp); len(p) > 0 {
		t.Fatalf("problems in good code: %q", p)
	}
	for _, tc := range []struct {
		name, old, new string
		want           []string // nil: no problem
	}{
		{"empty", folkCode, " \n", []string{"the code is empty"}},
		{"live_loop", "define :intro do", "live_loop :intro do", []string{"live_loop never ends"}},
		{"live_loop in a comment", "# pelog-ish", "# not a live_loop", nil},
		{"live_loop in a string", "sample :bd_haus", `puts "live_loop"`, nil},
		{"missing node comments", "    # node: intro.guitar\n", "", []string{`no "# node: <id>" comment for intro.guitar`}},
		{"node comment with another id", "# node: climax.bonang", "# node: climax.bonang2", []string{"comment for climax.bonang"}},
		{"function as variable", "notes = ring", "scale = ring", []string{"scale is a Sonic Pi function"}},
		{"longer variable name", "notes = ring", "my_scale = ring", nil},
		{"comparison", "notes = ring(60, 62.4, 64.8)", "notes = ring(60, 62.4, 64.8) if scale == 1", nil},
		{"unknown synth", "synth :pluck", "synth :gamelan", []string{"Sonic Pi 5.0.0 has no synth :gamelan; pick one from sonic_pi.synths"}},
		{"unknown fx", "with_fx :reverb", "with_fx(:hall", []string{"has no FX :hall"}},
		{"unknown sample", "sample :bd_haus", "sample :gong", []string{"has no sample :gong"}},
		{"sample from a file", "sample :bd_haus", `sample "C:/samples/gong.wav"`, nil},
		{"use_synth", "synth :dull_bell, note: 72", "use_synth :bonang", []string{"has no synth :bonang"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code := strings.Replace(folkCode, tc.old, tc.new, 1)
			if code == folkCode && tc.old != tc.new {
				t.Fatalf("%q is not in the code", tc.old)
			}
			problems := CheckCode(ctx, code, g, sp)
			if tc.want == nil && len(problems) > 0 {
				t.Errorf("want no problems, got %q", problems)
			}
			for _, w := range tc.want {
				if !strings.Contains(strings.Join(problems, "\n"), w) {
					t.Errorf("want a problem with %q, got %q", w, problems)
				}
			}
		})
	}
	if p := CheckCode(ctx, strings.Replace(folkCode, "synth :pluck", "synth :gamelan", 1), g, nil); len(p) > 0 {
		t.Errorf("names are only checked with a Sonic Pi: %q", p)
	}
}

// TestRubySyntax uses the Ruby of the installed Sonic Pi, when there is one.
func TestRubySyntax(t *testing.T) {
	sp, err := FindSonicPi("")
	if err != nil || sp == nil || sp.Ruby == "" {
		t.Skip("no Sonic Pi with Ruby installed")
	}
	ctx := context.Background()
	if p := sp.syntax(ctx, folkCode); p != "" {
		t.Errorf("good code fails: %s", p)
	}
	p := sp.syntax(ctx, strings.Replace(folkCode, "    sleep 4\n  end\n", "    sleep 4\n", 1))
	if !strings.Contains(p, "Ruby's syntax check fails:\ncode.rb:") || strings.Contains(p, "ctxcompiler-") || strings.Contains(p, "\r") {
		t.Errorf("a missing end: got %q", p)
	}
}

func TestRecordNeedsTheRecorder(t *testing.T) {
	sp, err := LoadSonicPi(fakeSonicPi(t))
	if err != nil {
		t.Fatal(err)
	}
	err = sp.Record(context.Background(), "x.rb", "x.wav", time.Second, true, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "Sonic Pi 5.0.0 has no headless recorder") {
		t.Errorf("got %v", err)
	}
}

func TestRecordingScript(t *testing.T) {
	got := recordingScript("use_bpm 90\nplay 60\n", `D:\music\song.recording.wav`, 90*time.Second, false)
	want := `recording_start
in_thread do
  use_bpm 60
  sleep 90
  recording_stop
  sleep 3
  recording_save "D:/music/song.recording.wav"
end

use_bpm 90
play 60
`
	if got != want {
		t.Errorf("got\n%s", got)
	}
	// A silent take turns Sonic Pi down once it is saved, before its engine
	// is unmuted.
	silent := recordingScript("play 60\n", `D:\music\song.recording.wav`, 90*time.Second, true)
	if !strings.Contains(silent, "  recording_save \"D:/music/song.recording.wav\"\n  set_volume! 0\nend\n") {
		t.Errorf("silent:\n%s", silent)
	}
}

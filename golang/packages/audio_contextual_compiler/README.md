# audio_contextual_compiler

An LLM contextual compiler for music. You describe a piece in a **prompt spec
context**, a markdown file with OOP-like pseudo code; Claude turns it into a
**jsongraph context**, and then into Sonic Pi code.

```
prompt spec context ──Claude──▶ jsongraph context ──Claude──▶ Sonic Pi code
   spec.md            pass 1      spec.graph.json    pass 2      spec.rb
                                  (checked, saved)
```

A plain prompt ("create me a folk song, slow and relaxing") leaves Claude to
decide everything, and every run comes out different. The spec's classes are
contexts rather than code: each one says what is true in one part of the
music (the song, a section, an instrument), so a vague word like "warm" only
applies where it is written, and every part has a name that stays the same
between runs. Claude still makes the musical choices inside each context.

Module `github.com/wargasipil/audio_contextual_compiler`, Go 1.25.

## Requirements

- **Go 1.25**, to build.
- **Claude Code** (`claude` on the PATH, signed in), for `compile` and
  `graph`. They call it in headless mode with your Claude Code login, so no
  API key is needed.
- **Sonic Pi 5**, for `render`, and optional for `compile` and `check`: with
  it the code is checked against its real synth, FX and sample names and its
  bundled Ruby (`ruby -c`); without it those checks are skipped with a note.
  5.0.0 is installed here, at `C:\Program Files\Sonic Pi`.
- **Windows PowerShell**, for silent rendering (Windows only; see Rendering).

## The tool

```powershell
pwsh build.ps1                                             # vet, test, build bin/ctxcompiler.exe and bin/ctxcompiler (bash build.sh on Linux)
bin\ctxcompiler.exe compile audiospec.md -o sonicpicode.rb # both passes
bin\ctxcompiler.exe graph audiospec.md                     # pass 1 only: audiospec.graph.json
bin\ctxcompiler.exe compile audiospec.graph.json           # pass 2 only, from a graph you checked or edited
bin\ctxcompiler.exe check audiospec.graph.json audiospec.rb   # the checks, without Claude (the code is optional)
bin\ctxcompiler.exe render audiospec.rb -d 5m              # record the first 5 minutes to audiospec.wav, silently (Sonic Pi 5; --audible to hear it)
bin\ctxcompiler.exe --help                                 # every command and flag
```

- Output files default to the input's name: `<spec>.graph.json`, `<spec>.rb`,
  `<code>.wav`. `-o` chooses another (the code for `compile`, the graph for
  `graph`, the WAV for `render`).
- `compile` saves the graph next to the spec (`--graph` to choose the file).
  When the spec has not changed since, the saved graph is used and only pass 2
  runs. When it has, pass 1 gets the old graph too and keeps its ids for what
  is still there. `--fresh` builds the graph again from scratch.
- `--model` (default `opus`), `--timeout` per Claude call (default 10m),
  `--retries` (default 2): how many more times a pass asks when its answer
  fails the checks.
- `--sonic-pi <folder>` or `$SONIC_PI_HOME` points at a Sonic Pi install; by
  default the usual install folder is used if there is one (on Windows
  `C:\Program Files\Sonic Pi`; also `/Applications/Sonic Pi.app/Contents/Resources`,
  `/opt/sonic-pi`, `/usr/lib/sonic-pi`). `--no-sonic-pi` checks without it.
- `render` needs `-d`, how much to record (`90s`, `5m`).
- Progress goes to stderr, the files written to stdout. A failed check exits
  with status 1 and lists the problems.

The written code starts with `# Compiled by ctxcompiler from <spec> (graph:
<graph>).`

[examples/folk_song.md](examples/folk_song.md) is a spec, and
`folk_song.graph.json` and `folk_song.rb` next to it are what it compiled to.

## The spec

Any markdown file. The pseudo code is usually in a code block; prose around it
is part of the spec too. Nothing parses the pseudo code: Claude reads it. What
it understands: classes with properties, `flow = [Intro(), Climax()]` for the
order of the sections, arguments like `Gong(role = "...")`, `extends` between
your own classes, and interactions like `on Kendang.signal -> all.speedUp`.

- `extends` is resolved in pass 1: a node gets the properties of the class it
  extends, and its own values win. A base class such as `Section` only tells
  the node's type.
- Prose about the whole piece goes into the song's `notes` prop, prose about
  one class into that node's `notes`.
- Pass 1 keeps the spec's words and adds no property, part or section the spec
  does not have; the musical choices are left to pass 2.

## The jsongraph context

```json
{
  "spec_sha256": "…",
  "nodes": [
    {"id": "song", "type": "song", "name": "FolkSong", "props": {"scale": "pelog", "duration": "5min"}},
    {"id": "intro", "type": "section", "name": "Intro", "props": {"length": "1min"}},
    {"id": "intro.guitar", "type": "instrument", "name": "Guitar", "props": {"role": "soft fingerpicked melody"}}
  ],
  "edges": [
    {"from": "song", "rel": "contains", "to": "intro", "order": 1},
    {"from": "intro", "rel": "contains", "to": "intro.guitar"}
  ]
}
```

- **Nodes:** `song` (exactly one, id `song`), `section`, `instrument` (anything
  that sounds) and `effect`. Props are the node's own properties in the spec's
  words; pass 1 adds nothing the spec does not say.
- **Ids** are paths: a section is its name in snake_case, a part is
  `<section>.<name>` (`intro.guitar`), a part of the whole song is
  `song.<name>`. The same instrument in two sections is two nodes.
- **Edges:** `contains` (song → section, with `order` in the flow; song or
  section → instrument or effect), `follows` (instrument → the instrument it
  locks to), `signals` (instrument or section → what reacts; `detail` names the
  event), `applies` (effect → what it is applied to).

Pass 2 does not get the flat graph but its nested contexts: the song, its
sections in flow order, the parts in each, and each node's relations. An inner
context's props override the ones around it, so the climax's "medium" tempo
wins over the song's "slow" there and nowhere else.

## The Sonic Pi code

Pass 2 turns the descriptions into concrete choices (tempo, scale, notes,
rhythm, synth, envelope, filter, volume) that fit every context a part is in.
It is told to write the code this way:

- **The piece ends.** Each section is a `define` that plays for the section's
  length and returns: its parts play together in `in_thread` with counted
  loops while the section's own thread sleeps for its length. The code ends by
  calling the sections in flow order, and the total matches the song's
  duration. No `live_loop`.
- **It plays the same every time:** it starts with `use_bpm` and
  `use_random_seed`.
- **Every part can be found:** a `# node: <id>` comment above the code of
  every section and part, with the graph's ids.
- **Relations:** `follows` locks a part's timing or notes to the other part's;
  `signals` is a `cue` in the part that signals and a `sync` in what reacts;
  `applies` wraps the target in `with_fx`.
- **Only Sonic Pi's own sounds.** An instrument it has no sound for (a gamelan
  gong, bonang, kendang) is approximated with built-in synths or samples, with
  a comment saying so. A scale it does not have (pelog, slendro) is computed as
  fractional MIDI note numbers.
- Nested calls take parentheses (`ring(60, 62)`), and Sonic Pi functions
  (`scale`, `chord`, `ring`, `note`) are not used as variable names.

## Checks

An answer that fails goes back to Claude with its problems, up to `--retries`
times; a graph is saved and code is written only once it passes.

- **The graph:** exactly one song, unique ids, known types and relations,
  edges between existing nodes of the right types (no edge listed twice or
  pointing at its own node), one parent for every node, sections with distinct
  orders of 1 or more, no circle of `follows`. Pass 2 checks the graph again
  first, so a hand-edited graph that fails is not sent to Claude.
- **The code:** not empty, no `live_loop` (the piece has to end), a
  `# node: <id>` comment above the code of every section and part, no Sonic Pi
  function (`scale`, `ring`, ...) used as a variable name. With Sonic Pi
  installed, also: every `synth`, `with_fx` and `sample` name is one this
  Sonic Pi has (pass 2 gets the lists, read from its `synthinfo.rb` and
  `etc/samples`), and Sonic Pi's bundled Ruby passes `ruby -c` on the code.

## How Claude is called

One `claude -p` call per answer, with the request as JSON on stdin and the
answer as structured output (`--json-schema`). The calls run with no tools
(`--tools ""`), no MCP servers (`--strict-mcp-config`), no saved session
(`--no-session-persistence`) and the temp folder as working directory, so a
project's `CLAUDE.md` and hooks stay out. A retry sends the rejected answer and
its problems along with the request.

## Rendering

`render` plays the code in Sonic Pi 5 without its GUI and records the first
`-d` of it (stereo, 48 kHz, 24-bit). It is real time, so a 5-minute song takes
a little over 5 minutes.

It is **silent** (2026-10-29): Sonic Pi records inside its audio engine,
before the sound device, so `render` mutes the engine's audio session in
Windows' volume mixer (`mute_windows.ps1`, through PowerShell) and the WAV is
the same. Only the engine process the render starts ("Sonic Pi -
SuperSonic", started since the render began) is muted, not a Sonic Pi you
have open; it is muted as soon as it opens the sound device, before the code
plays. Windows remembers an app's mute, so once the take is saved the code
turns Sonic Pi's own volume down (`set_volume! 0`) and the engine is unmuted
while it still runs. `--audible` lets you hear it instead. Elsewhere than
Windows, silent rendering is not done yet: use `--audible`. If a render is
killed while muted and Sonic Pi stays silent afterwards, unmute "Sonic Pi -
SuperSonic" in the volume mixer.

It uses `app/server/ruby/bin/headless-record.rb`, which Sonic Pi 5 ships, to
run the code. That script's own record tap writes nothing on Windows (Sonic
Pi 5.0.0; it prints `FAILED — no output written`, which `render` ignores), so
the recording is made with Sonic Pi's `recording_start` / `recording_save`
around the code instead. `recording_save` right after `recording_stop` fails
with "Permission denied" because the file is still in use, so it waits 3
seconds first. The take is saved as `<name>.recording.wav` and renamed when it
is complete, so a failed take leaves an older WAV as it was. If the code had
errors while it played, the WAV is still written and `render` says so with
status 1; Sonic Pi's output is shown above it.

Checked on the example: 20 seconds of `folk_song.rb` gave a 19.75 s WAV with
real signal (peak 0.23, RMS 0.034). Silent, 8 seconds of it gave a 7.79 s
WAV with real signal (mean -38.7 dB, peak -20.3 dB); the engine was muted
before Sonic Pi was ready and unmuted, with its one session, before it
stopped.

The whole 5 minutes of the example have not been rendered yet.

## Code

| File | What it holds |
| --- | --- |
| `graph.go` | the jsongraph context: `Graph`, `Node`, `Edge`, `Check`, and `Contexts`, the nested form pass 2 reads |
| `compile.go` | `Compiler`: pass 1 (`Graph`) and pass 2 (`Code`), their prompts and answer schemas, the retry loop |
| `claude.go` | `Claude`, the interface both passes ask through, and `ClaudeCLI`, the `claude -p` one |
| `sonicpi.go` | finding and reading a Sonic Pi install, `CheckCode`, and `Record` for `render` |
| `mute_windows.go`, `mute_windows.ps1` | muting Sonic Pi's engine in Windows' volume mixer while it records |
| `mute_other.go` | the same elsewhere: not done, so `render` there needs `--audible` |
| `cmd/ctxcompiler` | the command line (urfave/cli v3); `run` is its test entry point |

Tests (`go test ./...`) use a fake `Claude` and a small fake Sonic Pi install,
so they need neither Claude Code nor Sonic Pi; only the Ruby syntax test uses
an installed Sonic Pi, and it is skipped without one. Nothing records audio in
the tests. The build scripts run `go vet` and the tests before building.

## Not done yet

- Pass 2 writes the whole song in one call. Writing each part with only its
  own contexts, and rewriting only the parts whose nodes changed, is the next
  step for big specs.
- Claude cannot hear the result. With `render` there is audio now, so a loop
  that measures it (length, loudness, silence) or shows Claude a spectrogram
  image could send problems back like the code checks do.
- Silent rendering on macOS and Linux (`render --audible` works there).

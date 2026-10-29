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

Module `github.com/wargasipil/audio_contextual_compiler`, Go 1.25. It calls
Claude through the Claude Code CLI (`claude -p`) with your Claude Code login,
so no API key is needed.

## The tool

```powershell
pwsh build.ps1                                             # vet, test, build bin/ctxcompiler.exe and bin/ctxcompiler
bin\ctxcompiler.exe compile audiospec.md -o sonicpicode.rb # both passes
bin\ctxcompiler.exe graph audiospec.md                     # pass 1 only: audiospec.graph.json
bin\ctxcompiler.exe compile audiospec.graph.json           # pass 2 only, from a graph you checked or edited
bin\ctxcompiler.exe check audiospec.graph.json audiospec.rb   # the checks, without Claude
bin\ctxcompiler.exe render audiospec.rb -d 5m              # record the first 5 minutes to audiospec.wav (Sonic Pi 5)
```

- `compile` saves the graph next to the spec (`--graph` to choose the file).
  When the spec has not changed since, the saved graph is used and only pass 2
  runs. When it has, pass 1 gets the old graph too and keeps its ids for what
  is still there. `--fresh` builds the graph again from scratch.
- `--model` (default `opus`), `--timeout` per Claude call (default 10m),
  `--retries` (default 2): how many more times a pass asks when its answer
  fails the checks.
- `--sonic-pi <folder>` or `$SONIC_PI_HOME` points at a Sonic Pi install; by
  default the usual install folder is used if there is one (on Windows
  `C:\Program Files\Sonic Pi`). `--no-sonic-pi` checks without it.

[examples/folk_song.md](examples/folk_song.md) is a spec, and
`folk_song.graph.json` and `folk_song.rb` next to it are what it compiled to.

## The spec

Any markdown file. The pseudo code is usually in a code block; prose around it
is part of the spec too. Nothing parses the pseudo code: Claude reads it. What
it understands: classes with properties, `flow = [Intro(), Climax()]` for the
order of the sections, arguments like `Gong(role = "...")`, `extends` between
your own classes, and interactions like `on Kendang.signal -> all.speedUp`.

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

## Checks

An answer that fails goes back to Claude with its problems, up to `--retries`
times; a graph is saved and code is written only once it passes.

- **The graph:** unique ids, known types and relations, edges between existing
  nodes of the right types, one parent for every node, sections with distinct
  orders, no circle of `follows`.
- **The code:** no `live_loop` (the piece has to end), a `# node: <id>` comment
  above the code of every section and part, no Sonic Pi function (`scale`,
  `ring`, ...) used as a variable name. With Sonic Pi installed, also: every
  `synth`, `with_fx` and `sample` name is one this Sonic Pi has (pass 2 gets
  the lists, read from its `synthinfo.rb` and `etc/samples`), and Sonic Pi's
  bundled Ruby passes `ruby -c` on the code.

## Rendering

`render` plays the code in Sonic Pi 5 without its GUI and records the first
`-d` of it (stereo, 48 kHz, 24-bit). It is real time, so a 5-minute song takes
a little over 5 minutes, and **the music plays out loud** while it records.

It uses `app/server/ruby/bin/headless-record.rb`, which Sonic Pi 5 ships, to
run the code. That script's own record tap writes nothing on Windows (Sonic
Pi 5.0.0; it prints `FAILED — no output written`, which `render` ignores), so
the recording is made with Sonic Pi's `recording_start` / `recording_save`
around the code instead. `recording_save` right after `recording_stop` fails
with "Permission denied" because the file is still in use, so it waits 3
seconds first. The take is saved as `<name>.recording.wav` and renamed when it
is complete, so a failed take leaves an older WAV as it was.

Checked on the example: 20 seconds of `folk_song.rb` gave a 19.75 s WAV with
real signal (peak 0.23, RMS 0.034).

## Not done yet

- Pass 2 writes the whole song in one call. Writing each part with only its
  own contexts, and rewriting only the parts whose nodes changed, is the next
  step for big specs.
- Claude cannot hear the result. With `render` there is audio now, so a loop
  that measures it (length, loudness, silence) or shows Claude a spectrogram
  image could send problems back like the code checks do.

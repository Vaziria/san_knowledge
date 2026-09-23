# Sonic Pi

Sonic Pi is a musical instrument you play by writing code, live: Ruby-flavoured
code that sounds as soon as you press Run and can be edited while it plays. It
was created by Sam Aaron with support from the Raspberry Pi Foundation to teach
programming in schools, and is now also used by live-coding performers and DJs.
Tagline: "Code. Music. Live."

This is a study of someone else's repository, not one of ours. Source:
<https://github.com/sonic-pi-net/sonic-pi>, saved as
[the GitHub page](../external_sources/web/github.com/sonic-pi-net-sonic-pi.md).
Figures below come from the GitHub API for `main` on 2026-09-23.

## General.
1. about 12k stars, still active (last push 2026-09-20)
2. current version is v5.1-beta1 (2026-09-14)
3. mostly C++ (GUI and API) and Ruby (the language runtime), plus Clojure for
   synth design and some SuperCollider
4. about 27,500 files (~215 MB), most of which is bundled audio, docs and
   third-party code, not the project's own code
5. one git submodule: `app/external/supersonic` (`samaaron/supersonic`), the
   audio engine
6. built with CMake and vcpkg; ships its own Ruby
7. the app and tutorial are translated into about 20 languages via Weblate

## Language ideas.

```ruby
live_loop :drums do
  sample :bd_haus
  sleep 0.5
end

live_loop :melody do
  use_synth :prophet
  play scale(:e3, :minor_pentatonic).choose, release: 0.3
  sleep 0.25
end
```

- **Virtual time.** `sleep` doesn't pause the thread; it moves a logical clock
  forward. Every sound is scheduled ahead with a timestamp, so timing stays
  sample-accurate even though Ruby itself runs unevenly.
- **Live loops.** Each `live_loop` runs in its own thread. Pressing Run again
  swaps in the new code at the start of the loop's next pass, so the music
  never stops.
- **`cue` / `sync`.** Threads coordinate through a shared, time-stamped event
  store. MIDI, OSC and Ableton Link events come in through the same store.
- **Determinism.** "Random" functions use a seeded generator, so the same code
  always gives the same music, which matters in classrooms.

## Architecture.

```
GUI (C++ / Qt, QScintilla editor)
  └─ C++ API layer (app/api, sonicpi_api.h)
       └─ Boot daemon (Ruby): launches and watches the processes,
          assigns ports, auth token, kill switch
            ├─ Spider (Ruby runtime): runs user code, virtual time,
            │  threads, cue/sync; sends timestamped OSC bundles over TCP
            └─ SuperSonic audio engine = scsynth (SuperCollider synthesis)
               on clockwork (audio device IO, MIDI, OSC, gamepad,
               clock, Ableton Link)
Spider reads the engine's clock from shared memory.
```

1. **GUI** (`app/gui`): Qt desktop app with the editor, themes, the built-in
   tutorial, an audio visualiser and translations.
2. **API** (`app/api`): C++ library between the GUI and the backend. It boots
   the daemon and handles OSC, so other front-ends could reuse the backend.
3. **Daemon** (`app/server/ruby/bin/daemon.rb`): process supervisor. Starts the
   engine and Spider, finds free ports, reads `v5-audio-settings.toml`,
   forwards engine events to the GUI, and shuts everything down if the GUI
   dies.
4. **Spider** (`app/server/ruby/lib/sonicpi/`): the language runtime.
5. **SuperSonic**: Sam Aaron's rewrite of SuperCollider's scsynth, running on
   his clockwork runtime. The same engine can also run in a browser as an
   AudioWorklet, or inside Erlang's BEAM as a NIF.
6. **Synth designs** (`etc/synthdefs/designs/overtone/`): synths and FX are
   written in Clojure with Overtone and compiled to SuperCollider synthdefs.

## Recent changes (2026).
- **New audio engine.** The daemon used to launch a stock scsynth; it now
  launches SuperSonic ("Clockwork Ready!", 2026-09-11). No Elixir/Erlang
  "Tau" server code remains in the tree.
- **TCP instead of UDP** for engine commands, after a 2026-08-05 investigation
  found silently lost UDP packets were making notes late.
- **Clock from shared memory.** Spider reads tempo and beat position from
  clockwork's shared memory (`clockwork_arena.rb`) instead of asking the engine
  over the network on every `sleep`.
- **Tracks panel with audio plugins** being added; new track functions are
  marked for v6.
- **Option validation rules as data**, shared by the desktop app and a web
  runtime. With SuperSonic also running in the browser, this points to a web
  version of Sonic Pi.

## Repository layout.

| Part | Files | Size | What it is |
|---|---|---|---|
| `etc/` | 9,619 | 116 MB | Content: samples, wavetables, docs, synthdefs |
| `app/server` | 15,248 | 35 MB | Ruby runtime; ~99% of files are bundled gems |
| `app/gui` | 1,095 | 48 MB | Qt app; half is bundled QScintilla, translations, images |
| `app/api`, `app/api-tests` | ~850 | 10 MB | C++ API layer; mostly bundled libraries |
| `app/external` | 536 | 4 MB | Native dependencies and the SuperSonic submodule |
| root, `bin/`, `install/`, `packaging/` | ~60 | small | Docs, launchers, installers |

### Root and launchers.
- Build guides: `BUILD-WINDOWS.md`, `BUILD-MAC.md`, `BUILD-LINUX.md`,
  `BUILD-RASPBERRY-PI.md`, `PACKAGING.md`
- Contributor guides: `CONTRIBUTING.md`, `TESTING.md`, `SYNTH_DESIGN.md`,
  `TRANSLATION*.md`
- `bin/sonic-pi` starts the app; `sonic-pi-repl.sh` / `.bat` give a terminal
  REPL with no GUI

### Build scripts.
Each platform has its own chain in `app/` with the same stages:
`*-config` → `*-pre-vcpkg` → `*-prebuild` → `*-build-gui` → `*-build-all`,
for `win-*.bat`, `mac-*.sh`, `linux-*.sh` and `pi-*.sh`. macOS release runs 8
numbered steps (stage, prune, bundle dylibs, Info.plist, codesign, DMG,
notarize, verify, compat audit). Linux has AppImage and `.deb` scripts.

### Ruby runtime (app/server/ruby).
| Path | Role |
|---|---|
| `bin/daemon.rb` | Boot daemon |
| `bin/spider-server.rb` | Spider entry point |
| `bin/repl.rb`, `headless-run.rb`, `headless-record.rb` | Running without the GUI |
| `lib/sonicpi/lang/sound.rb` (248 KB) | `play`, `sample`, `synth`, `with_fx`, `control` |
| `lib/sonicpi/lang/core.rb` (210 KB) | `live_loop`, `sleep`, `cue`/`sync`, `in_thread`, `define`, `tick`, rings |
| `lib/sonicpi/lang/midi.rb` (76 KB) | MIDI functions |
| `lib/sonicpi/lang/western_theory.rb` | `chord`, `scale`, note names |
| `lib/sonicpi/synths/synthinfo.rb` (320 KB) | Parameters, defaults, docs and validation for every synth and FX |
| `lib/sonicpi/runtime.rb` | Runs code: jobs, threads, the virtual-time scheduler |
| `lib/sonicpi/preparser.rb` | Rewrites source before it runs |
| `lib/sonicpi/studio.rb` | Audio graph: synth, FX, bus and buffer allocation |
| `supersonic_*_comms.rb`, `osc/` | TCP/OSC clients to the engine, one per subsystem |
| `clockwork_*.rb` | Reads the engine clock from shared memory |
| `midi_api.rb`, `osc_api.rb`, `link_api.rb`, `gamepad_api.rb` | External I/O |
| `cueevent.rb`, `event_history.rb`, `incomingevents.rb` | Event store behind `cue`/`sync` |
| `gitsave.rb` | Auto-commits the user's buffers to a local git repo |
| `test/` | About 72 Ruby test files |
| `vendor/` | 15k files of bundled gems, mostly `rugged` (libgit2 binding, ~12k files) |

`lang/minecraftpi.rb` is empty: the old Minecraft Pi integration is gone.

### GUI (app/gui).
- `mainwindow.cpp` (370 KB) is the core of the UI.
- `widgets/` (~65 files): editor (`sonicpieditor`, `sonicpilexer`,
  `completionpopup`), panels (`logpanel`, `metricspanel`, `trackspanel`,
  `tutorialpane`, `nodetreegraph`), audio and Link (`devicelistwidget`,
  `linkaudiostreamswidget`, `bpmscrubwidget`).
- `platform/`: screen recording, Syphon (macOS) and Spout (Windows) video
  sharing, Windows accessibility, MP4 remuxing.
- `lang/` (14 MB translations), `images/`, `fonts/`, and bundled
  `QScintilla_src-2.14.1/` (18 MB).

### C++ API and externals.
- `app/api/include/api/sonicpi_api.h` (816 lines) is the API the GUI calls.
- `app/api/src/`: `sonicpi_api.cpp`, `osc/` (UDP and TCP senders and servers),
  `audio/audio_processor.cpp` (scope analysis), `*.test.cpp` next to the code.
- `app/api/vendor/`: reproc, kissnet, kissfft, TLSF, PlatformFolders,
  ghc_filesystem. `app/api-tests/` only holds bundled Catch2.
- `app/external/`: `supersonic/` (submodule), `aubio-0.4.9/` (onset
  detection), `Spout2/`, `Syphon-Framework/`, `piano/`.

### Content (etc).
| Path | Size | Contents |
|---|---|---|
| `wavetables/AKWF` | 46 MB | Adventure Kid single-cycle waveforms |
| `samples/` | 35 MB | Built-in samples (`:bd_haus`, `:loop_amen`, ...) |
| `doc/` | 23 MB | Tutorial (86 chapters), translations, cheatsheets |
| `synthdefs/designs/overtone/` | small | Synth source in Clojure/Overtone |
| `synthdefs/compiled/` | 164 files | Compiled `.scsyndef` files |
| `synthdefs/graphviz/` | 6 MB | Diagrams of each synth's signal graph |
| `examples/` | 34 files | Pieces by level: apprentice to algomancer |

## Where to start reading.
1. `lang/core.rb` and `lang/sound.rb`: what the language offers
2. `runtime.rb`: how code runs and how `sleep` / virtual time work
3. `studio.rb` and `supersonic_comms.rb`: how `play` becomes an OSC bundle
4. `bin/daemon.rb`: how the processes start and connect
5. `app/gui/mainwindow.cpp`: the UI
6. `etc/synthdefs/designs/overtone/.../basic.clj`: how a synth is built

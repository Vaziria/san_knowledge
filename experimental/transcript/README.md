# transcript

A scratch web app for trying streaming speech-to-text with Vosk. Speak into the
browser and watch partial results stream in, then firm up when Vosk decides an
utterance ended.

Not part of the `san_knowledge` module - it is its own module because it needs
cgo and the Vosk native library.

## Run

```powershell
pwsh experimental/transcript/setup.ps1   # once: downloads libvosk + the English model (~70 MB)
cd experimental/transcript; go run .     # opens http://127.0.0.1:7575
```

Flags: `-addr`, `-models`, `-open=false`.

No `CGO_*` variables are needed. The Go binding's cgo directives look for the
header and library in `<module>/../src`, which normally lands in the read-only
module cache. `setup.ps1` vendors the module so that path becomes
`vendor/github.com/alphacep/vosk-api/src` and drops `vosk_api.h` there, while
`libvosk.dll` goes beside these sources - [cgoflags.go](cgoflags.go) links it
with `-L${SRCDIR}`, and Windows finds it at run time through the working
directory. That last part is why `go run .` wants to be run from this directory.

## Indonesian

AlphaCephei publishes no Indonesian model - their list is ar br ca cn cs de el
en eo es fa fr gu hi it ja ka ko ky kz nl pl pt ru sv te tg tl tr uk uz vn, with
no `id`. But [bookbot-kids/speech-recognizer-bahasa-indonesian][bookbot] ships a
complete one under Apache-2.0: 56 MB, a 28,161 word vocabulary, and the standard
`am/ conf/ graph/ ivector/` layout, so it loads unmodified. `setup.ps1` fetches
it as `models/id`.

[bookbot]: https://github.com/bookbot-kids/speech-recognizer-bahasa-indonesian

It is better than expected on ordinary Indonesian and predictably bad on
everything else. Transcribing a spoken Indonesian Wikipedia article:

```
reference    Atta Halilintar merupakan anak sulung dari kesebelasan Gen
             Halilintar. Dia merupakan putra pertama dari pasangan perantau
             Minang ...
recognized   atol halilintar merupakan anak sulung dari kesebelasan gen
             halilintar dia merupakan putra pertama dari pasangan perantau
             minat tabal
```

Function words and common vocabulary come through nearly intact. What fails is
consistent: proper nouns (`Atta` -> `atol`), numbers (`20 November 1994` ->
`tua pun masai bor seribu sma ratus belam bu umpat`), and above all English
loanwords - `YouTuber` -> `jitu bar`, `internet` -> `tebar`, `selebriti` ->
`sel beri dis`.

That last row is the one that matters here. **The Indonesian model cannot take
English.** Running id and en-us side by side does not fix it either, because
neither can handle a switch inside one utterance - Vosk decodes against a single
lexicon. Genuine id+en mixing still needs Whisper.

Where it does work well: ordinary Indonesian dictation feeding an LLM, which
repairs mangled proper nouns from context, and command-style input with a
grammar.

## How it works

```
browser                                    server
  getUserMedia, AudioContext @ 16 kHz
  worklet.js: f32 -> s16, 100 ms chunks
  websocket, binary frames          --->  vosk.AcceptWaveform
                                           0 -> PartialResult  {"type":"partial"}
                                           1 -> Result         {"type":"final"}
  "eof" on stop                     --->  FinalResult, flush tail
```

The page asks for a 16 kHz `AudioContext` so there is no resampling to do, and
Vosk does its own endpointing - a "final" is a boundary it chose, not a timer.

Models are cached in-process across connections (`hub`), since loading one
costs a second or two and a few hundred MB.

## The grammar field

Vosk accepts a phrase list per recognizer, which constrains decoding to that
vocabulary. It is the biggest accuracy lever available and the difference is
easy to see. Feeding the same synthesized sentence through both paths:

```
free      ... and start the sink now
grammar   ... and start the sync now
```

Anything outside the list comes back as the nearest-sounding in-list phrase, so
it suits command-style input, not open dictation. Pointing a grammar at
free-flowing speech collapses it into fragments of the listed phrases:

```
grammar      atta halilintar merupakan anak sulung dari kesebelasan gen halilintar
recognized   kesebelasan gen gen halilintar kesebelasan halilintar
```

Leave it empty for free recognition.

## Later, on the Pi

The Go side cross-compiles, but cgo means you need the matching native library:
`vosk-linux-aarch64-0.3.45.zip` for 64-bit Pi OS, `vosk-linux-armv7l-0.3.45.zip`
for 32-bit. Drop `libvosk.so` where `libvosk.dll` sits now and set
`LD_LIBRARY_PATH=.`, since Linux does not search the working directory. Build on
the Pi itself, or with a cross toolchain plus those headers.

Note a Vosk model wants far more memory than its download size suggests. With
`en-us` and `id` both loaded this process sits at ~464 MB private, so budget
roughly 230-300 MB per model. On a 1 GB Pi 3B+ that means one model at a time,
not two side by side.

## If Start does nothing

The button is a state machine (`idle -> starting -> running -> stopping`) and
disables itself mid-transition, so clicking twice cannot start two sessions.
Failures show up in a red banner rather than only the footer.

Two bugs used to produce a dead button, both fixed:

- `start()` awaited `onopen` alone, so a refused upgrade never settled and the
  button stayed on "Start" forever. It now rejects on error, close or a 10s
  timeout, and tears the microphone back down on every failure path.
- The server died on a bad model name. `vosk.NewModel` returns a nil error even
  when `vosk_model_new` returns NULL, and that NULL crashes the process inside
  `vosk_recognizer_new`. Model directories are now validated, and only names
  from a directory scan are accepted.

## Behind a tunnel or port forward

The server speaks plain HTTP, but it is often reached through something that
terminates TLS - a VS Code tunnel or forwarded port, a dev tunnel, a reverse
proxy. A page loaded over HTTPS may not open a `ws://` socket:

```
Failed to construct 'WebSocket': An insecure WebSocket connection may not be
initiated from a page loaded over HTTPS.
```

So the socket URL follows `location.protocol` and becomes `wss://` there. The
TLS front-end terminates it and forwards a plain upgrade to this server, which
needs no certificate of its own. Verified against a TLS-terminating reverse
proxy, audio and all.

Reaching it over a LAN address on plain HTTP will not work: that is not a
secure context, so the browser exposes no microphone at all. Loopback and
HTTPS are both fine.

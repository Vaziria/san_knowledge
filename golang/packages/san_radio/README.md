# san_radio

Stream music around the clock to YouTube from a Raspberry Pi. It plays the
`.wav` files in a folder over a looping background video of a terminal
window with three panes, and keeps them live:

- **the chat pane:** each new track is typed as `play <file>`, and each live
  chat comment as `greet @name "comment"`, which prints `Hello, @name!`;
- **neofetch:** Tracks, Now (the track playing) and Uptime;
- **`eza --tree`:** the music folder, the playing track marked;
- **the tab bar:** the clock.

```
radio@getresolved ~
❯ play auntum.wav
radio@getresolved ~
❯ greet @budi "lagunya enak banget"
Hello, @budi!
[radio ▸ ~ ▸] █
```

Module `github.com/wargasipil/san_radio`, Go 1.26. It reads the chat with
[san_youtube](../san_youtube) (no API key) and needs `ffmpeg` on the machine.

```
music/*.wav ──► one ffmpeg per track ──► PCM ─────────────┐
loop.mp4 (looped) ────────────────────────────────────────┼──► ffmpeg ──► rtmp://…/live2/<key>
tracks + live chat + clock ──► ten text files ────────────┘   (draws the text every frame)
```

- **Music:** every `.wav` in the folder, in name order, again and again.
  Number the files (`01 …`, `02 …`) to set the order. The folder is read
  again before each track, so new files play without a restart. Hidden files
  (`.name.wav`) are skipped, so a file can be copied in under a hidden name
  and then renamed. Any sample rate, bit depth or channel count plays. With no
  files, the stream carries on in silence.
- **Video:** one file, looped: everything still on the screen (the night
  scene, the window, the pixel art, the static neofetch rows, the prompt
  bars). The built-in one is [background/terminal.html](background/terminal.html),
  rendered to `assets/loop.mp4` by `pwsh background/render.ps1` (headless
  Edge). It leaves room for the live text at the places in
  [layout.go](layout.go); a Remotion version must keep those places free:
  H.264, 16:9, its last frame running into its first.
- **The live text:** DejaVu Sans Mono 16px at 720p (built in,
  [fonts/LICENSE](fonts/LICENSE)), so the static text in the background and
  the live text match. The chat pane holds 125×12 characters. Commands type
  at about 25 characters a second (faster when others wait) on the input
  line after the prompt bar, then move up into the history; between
  commands the cursor blinks. Each colour is one text file and one
  drawtext, the other colours' characters as spaces, so in the monospace
  font they line up.
- **Comments:** comments and Super Chats. Emoji and links are left out: the
  font has no colour emoji. A comment deleted by a moderator leaves the
  screen at once, and so does everything by a banned author. With
  `-blocked words.txt`, a comment with any of those words is never typed.
  At most 10 wait; in a busier chat the oldest waiting one is dropped.
- **Around the clock:** when ffmpeg stops (the network drops, YouTube closes
  the connection), it starts again after 5 s, then 10 s, up to a minute. The
  music and the terminal carry on where they were. When the stream is a new
  video after a restart, the chat is opened again.

## Usage

```
san_radio -config assets/channels.yaml -video assets/loop.mp4 -music assets
san_radio -video assets/loop.mp4 -music assets -out test.flv      # to a file, to try it
```

The stream settings come from a YAML file (`assets/channels.yaml`, which git
ignores, like the music):

```yaml
stream_id: xxxx-xxxx-xxxx-xxxx-xxxx   # the stream key, from YouTube Studio
stream_url: rtmp://a.rtmp.youtube.com/live2
youtube_channel: "@yourchannel"       # whose live chat is greeted
```

| Flag | Default | |
|---|---|---|
| `-config` | | the YAML above; `-url`, `-chat` and `-key-file` given as well win |
| `-video` | (required) | background video, looped |
| `-music` | `music` | folder of `.wav` files |
| `-chat` | none | channel whose live chat is greeted, e.g. `@yourchannel` |
| `-url` | `rtmp://a.rtmp.youtube.com/live2` | RTMP server; the key is added to it |
| `-key-file` | `-config`'s key, else `$SAN_RADIO_KEY` | file holding the stream key |
| `-out` | | a URL or file instead of `-url` and the key |
| `-size` | `1280x720` | video size |
| `-fps` | `30` | frames per second |
| `-bitrate` | `2500k` | video bitrate |
| `-encoder` | `auto` | the Pi's hardware encoder (`h264_v4l2m2m`) when it works, else `libx264` |
| `-decoder` | `auto` | the Pi's hardware decoder for the video when it works, else software |
| `-preset` | `veryfast`, `ultrafast` on ARM | libx264 preset |
| `-blocked` | | file of words, one per line, `#` for comments |
| `-poll` | YouTube's pace (~10 s) | read the chat at least this often, e.g. `3s` |
| `-text-dir` | `/dev/shm`, else the temp folder | where the live text's files are written |

The shell's `user@host` is `radio@` and the chat's channel, lowercased
(`radio@getresolved`); the background's static text says the same, so
change both together.

The key is never logged; it shows as `****`.

The log says which encoder and decoder it uses, a status line a minute after
starting and then every 10 minutes (`live 1:02:03, 30.0 fps, speed 1.00x`),
each comment, each track, and **"falling behind"** when the machine makes
less than 0.95 s of stream per second, measured over 30 s. If it falls
behind, lower `-size`, `-fps` or `-bitrate`.

In YouTube Studio, set the stream's latency to **Ultra low**, so viewers see
their greeting within seconds of writing. YouTube warns that the bitrate is
below 2500 kbit/s while the picture is mostly still: a still picture needs few
bits. Viewers see no difference.

## Raspberry Pi

`-encoder auto` and `-decoder auto` use the Pi's hardware H.264 encoder and
decoder (the Pi 3 and 4 have them, the Pi 5 doesn't). On a Pi 3B+ at 720p,
held at 600 MHz by a weak power supply (2026-10-12):

- the three panes (ten drawtexts) fell behind at 30 fps (0.92x) and hold
  at **24 fps** (1.00x, ffmpeg about 1.6 of 4 cores, 52 °C), so `run.sh`
  passes `-fps 24`;
- decoding a detailed 720p video in software takes about three cores, so the
  hardware decoder matters for a busy Remotion background;
- a weak power supply makes the Pi slow itself down (`vcgencmd get_throttled`
  not `0x0`, "Undervoltage detected" in `dmesg`): use a 5.1 V 2.5 A supply.

## Build and deploy

```powershell
pwsh build.ps1        # vet, test, then bin/san_radio.exe, bin/san_radio, bin/san_radio-linux-arm64, bin/san_radio-linux-arm
pwsh deploy.ps1       # to the SSH host "raspi"
```

`deploy.ps1` (`-Target` for another SSH host) needs no sudo on the Pi:

1. picks the build for the Pi (`uname -m`: `aarch64` → arm64, `armv6l`/`armv7l` → arm);
2. without an installed `ffmpeg`, fetches Debian's packages with
   `apt-get download` and unpacks them into `~/san_radio/ffmpeg`, run by
   [ffmpeg.sh](ffmpeg.sh) (about 120 MB, once);
3. copies the program, [run.sh](run.sh), `assets/loop.mp4` (`-Video`), the
   `.wav` files in `assets/` the Pi doesn't have yet (`-Music`) and
   `assets/channels.yaml` (`-Config`, readable only by the Pi's user), each
   under a temporary name first;
4. restarts the stream (a few seconds off the air), and adds `run.sh` to cron
   at boot. `run.sh` starts san_radio again whenever it exits, and only one
   copy runs: one stream key carries one stream;
5. turns on lingering for the Pi's user (`loginctl enable-linger`). Without
   it, systemd-logind empties `/dev/shm`, where the live text is, when that
   user's last login ends, so an SSH logout would stop the stream. Should it
   happen anyway, san_radio writes the missing files again before ffmpeg
   starts again (a few seconds off the air).

Everything on the Pi is in `~/san_radio`; the log is
`~/san_radio/san_radio.log`. To stop it: `pkill -f '^/bin/sh /home/pi/san_radio/run.sh'; pkill -x san_radio`.

## Limits

- The chat is read the way youtube.com's chat window reads it, not through
  the official API, so a change on YouTube's side can break it until
  san_youtube is updated. The stream itself does not depend on it.
- A greeting takes YouTube's chat pace (up to ~10 s) plus the stream's
  latency to appear. `-poll 3s` is faster; YouTube answers too many requests
  with a 429, after which the chat keeps to its own pace for 10 minutes.
- Characters the font lacks show as boxes, and wide ones (Chinese, Japanese)
  put that line's colours out of step.

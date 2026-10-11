# Article guideline

How articles for Medium are written and stored in this repo. Articles are drafts; the user publishes them to Medium by hand, from a Medium draft that `san_medium` can make (see "Publishing to Medium").

## Layout

```
docs/articles/
  guideline.md                       this file
  foreign_terms_wordlist.md          which terms stay English, which stay Indonesian
  dialectical_writing.md             how an article argues: claim, objection, resolution
  <topic>/
    <title-slug>.md                  one file per article
    assets/
      <name>.svg                     diagram source
      <name>.png                     the render uploaded to Medium
```

- **Topic folder:** kebab-case, named by the topic, not by an article (`create-own-vpn`).
- **Article file:** the slug of its title. Lowercase, words joined by `-`, brackets and punctuation dropped: "[Seri Membuat VPN Sendiri Bagian 1] Apa itu VPN?" becomes `seri-membuat-vpn-sendiri-bagian-1-apa-itu-vpn.md`. Renaming the title renames the file.
- **Assets:** everything an article shows lives in its topic's `assets/`, linked relatively (`assets/<name>.png`).
- **Online drafts:** none. Articles live only as files here, never as claude.ai Artifacts or Claude Docs, unless the user asks (CLAUDE.md, "Online artifacts").

## Educational only

Articles teach concepts. They may explain an architecture we built, but they never point back to it.

- **No references to our work:**
  - no names of our packages, products or repos
  - no links to our GitHub organisations or releases
  - no file names, function names or types from our code
  - no commands or flags of our tools
- **No internals.** No hostnames, tunnel ids, URLs, keys, paths, account names, or defaults that identify our setup, such as our overlay range.
- **Code:** examples are written fresh for the article, as small as the point needs. Never paste or adapt code from our repos.
  - Build the code in a scratch module first, and run it, including its tests and `-race`, before it goes into the article.
  - After writing, extract the article's code blocks and check them against the tested files, so what readers copy is what ran. A block that shows a whole file must equal it. An excerpt must appear in the file exactly as written, and the article says what the rest of the file is ("sama persis dengan Bagian 2").
- **How to describe our design:** in general terms, as "a VPN that carries WireGuard over WebSockets", or as something the reader and the writer build together ("yang akan kita bangun").
- **Check before handing over:** search the article for package names, `github.com`, internal paths and our default values.

## Language and tone

- **Language:** Bahasa Indonesia, including titles, captions and the text inside diagrams.
- **Reader:** addressed as "kamu"; the writer is "saya". Use "kita" for building something together ("yang akan kita bangun").
- **Foreign terms stay foreign.** Technical terms keep their original form, usually English, untranslated and with no Indonesian equivalent next to them. Write "tunnel", not "terowongan" or "terowongan (tunnel)".
- **Everyday words stay Indonesian:** paket, jaringan, alamat, kunci, and Indonesian spellings of borrowed words such as enkripsi and autentikasi.
- **The wordlist decides.** [foreign_terms_wordlist.md](foreign_terms_wordlist.md) lists every term decided so far: what to write, what not to write, and the forms with suffixes. Check every line of an article against it on each revision, not only the lines that changed. Add each new term there; when a term could go either way, ask the user.
- **Code, commands and output:** stay as they are, in code blocks.

## Series

- **Title:** `[Seri <Nama Seri> Bagian <N>] <Judul>`, e.g. "[Seri Membuat VPN Sendiri Bagian 1] Apa itu VPN?".
- **Opening:** each part starts by saying what the series builds and what this part covers.
- **Closing:** each part ends with `## Berikutnya dalam seri ini`, a short teaser for the next part. Update the teaser when the plan changes. The last part of a series ends with `## Akhir seri <Nama Seri>` instead.

Current series:

- **Membuat VPN Sendiri** (`create-own-vpn/`). It is based on our own VPN work, which the articles never name (see "Educational only").
  - Bagian 1, "Apa itu VPN (Virtual Private Network) ?": written, in Indonesian. It ends with the language (Go, and why) and the components needed, and promises that Bagian 2 starts writing code with the delivery path, WireGuard over WebSocket through a relay.
  - Bagian 2, "Membawa WireGuard lewat WebSocket dan relay": written, in Indonesian. It covers why WebSocket and its costs, the anatomy of the delivery path, the message format, a relay and a WebSocket Bind for wireguard-go, and a test of two members over netstack.
    - The code was written fresh for the article, in a scratch module called `vpnku`, and tested before it went in: `go vet`, `go test` 20 times, and `-race` in Docker.
    - The article's code blocks were extracted and diffed against the tested files.
    - The teaser promises Bagian 3: a real virtual network card (TUN and Wintun), addresses and routes, two real machines, then proof of key ownership at the relay.
  - Bagian 3, "Menjalankan VPN dengan virtual network card sungguhan": written, in Indonesian.
    - It covers netstack against a real TUN, the OS anatomy (TUN device, prefix, route, MTU, the Windows firewall), `genkey`, the member program with `os_linux.go` and `os_windows.go`, the relay on a VPS behind Caddy, a two-machine try-out, and the limits.
    - Verified on Linux in Docker: a relay, Caddy as reverse proxy, and two members on real `/dev/net/tun`. Ping both ways worked, as did HTTP over the VPN; the route and MTU 1420 were set. A relay restart stopped ping, as the limits section says.
    - The ping and `ip` output quoted in the article comes from that run, and says so.
    - Windows is **not yet run**: `os_windows.go` (Wintun, netsh, firewall rule) passes `go vet` and builds, but needs an elevated terminal. Run it once before publishing.
    - The teaser promises Bagian 4: proof of private-key ownership at the relay, and a member list the relay hands out.
  - Bagian 4, "Membuktikan kepemilikan kunci dan membagikan daftar client": written, in Indonesian.
    - It covers why the relay needs proof, not a claim, and why the proof runs both ways: a fake relay could slip an attacker into the peer list.
    - It compares three options (a shared password, a token per member, proof from the WireGuard key) and explains the anatomy: shared secret by X25519, nonce, HMAC, label.
    - It covers the five-step mutual proof, member list and addresses from the relay as a JSON text message (text for control, binary for data), the code (`proof`, `relay`, `cmd/relay` with `relay.json`, the changed Bind, `cmd/client`), the tests, and the limits.
    - The code lives in the scratch module `vpnku4`, a copy of `vpnku`, so the Part 2 and 3 code stays as published.
    - Tested: four tests (registered members, stranger, intruder with A's key, fake relay), `-count=10`, and `-race` in Docker.
    - On real TUN in Docker: relay, Caddy, two members and a stranger. A and B got their addresses and one peer each from the relay and pinged both ways. The stranger was refused with "bukan anggota" (now "bukan client terdaftar", see the rename below).
    - The article's code blocks were checked against the tested files: full files are equal, excerpts are substrings.
    - Windows is still not run (same `os_windows.go` as Bagian 3).
    - The teaser promises Bagian 5: reconnect, live member-list updates, and a queue per member at the relay.
  - The series is complete at nine parts (plan agreed 2026-10-11). Bagian 5 to 9 were written on 2026-10-11, each from a scratch module `vpnku5` … `vpnku9`, a copy of the one before.
    - Scratch modules live in the session scratchpad and are gone afterwards. The article code blocks are the record: `vpnku4` was rebuilt from Bagian 2 to 4 and its blocks matched again.
    - Every part's code blocks were filled from the tested files by script and checked: full files equal, excerpts substrings.
    - Tests ran `-count=20` on Windows and with `-race` (in WSL Ubuntu for Bagian 5 and 6, then on Windows with the msys64 gcc).
    - Docker Desktop failed on 2026-10-11, so the real-TUN runs for Bagian 5 to 7 used WSL Ubuntu as root with network namespaces and a bridge (`192.0.2.0/24`). The user then asked not to use WSL for article work, so Bagian 8 and 9 have `go test` only.
  - Bagian 5, "Membuat VPN yang tahan banting": written.
    - It covers why the connection to the relay breaks, and why TCP keepalive and WireGuard's own retries are not enough.
    - It builds a Bind that reconnects inside its receive function: a WebSocket ping every 15 s, 10 s per attempt, gaps from 1 s doubling to 30 s.
    - The relay re-reads `relay.json` every 2 s and sends the full member list; members add and remove peers without resetting kept sessions; the relay keeps a queue of 256 messages per member, with a 10 s write limit.
    - Tested: 7 tests. TestClientLambat was also run against the Bagian 4 relay and fails there, as it should.
    - Real TUN (WSL): relay restart, a silent 40 s cut (found after 17 s), a member added and one removed. The quoted log lines and ping counts come from that run.
  - Bagian 6, "Menambah client VPN dengan undangan sekali pakai": written.
    - It compares three ways in (admin makes the key, approve on first connect, one-time invite) and picks the invite: relay URL, relay public key and a 32-byte token, base64 with a `vpnku-` prefix. The relay keeps only the token's SHA-256, valid 24 hours.
    - New: `relay init|run|undang|keluarkan|daftar`, `client gabung`, `POST /gabung`, member names, the next free address (skipping network, broadcast and the address before broadcast), and safe updates of `relay.json` with a `.lock` file and rename.
    - Tested: 5 tests, including 20 concurrent writers. Real TUN (WSL): init, two joins, a reused invite refused, `keluarkan` cut the member within 2 s.
  - Bagian 7, "Menjangkau client VPN lewat nama": written.
    - Domain `vpn.internal` (ICANN reserved `.internal` on 2024-07-29; `.local` is mDNS, RFC 6762). Each member answers DNS itself, inside the tunnel, at the address before broadcast (`10.10.0.254`), by wrapping the TUN device. No port 53 is opened.
    - OS split DNS: `resolvectl` on Linux (`~vpn.internal`, `default-route false`), an NRPT rule on Windows. `nslookup` ignores NRPT (Microsoft Networking Blog, 2026-01-08).
    - Tested: 3 `names` tests and a netstack test by name. Real TUN (WSL): `dig` answers, `resolvectl status` and query, `example.com` unaffected.
    - Windows (NRPT) is **not run**.
  - Bagian 8, "Menjalankan relay VPN tanpa server sendiri": written.
    - It compares VPS + Caddy, a tunnel service (Cloudflare quick tunnel, Microsoft dev tunnels) and a container service (Cloud Run), with sourced limits and prices. The Cloud Run figure, about US$44 a month, is computed from the listed us-central1 prices.
    - Code: the relay's `-renew` asks members to open a new connection before the front cuts the old one; the old one gets 10 s of grace; `PORT` is honoured. Storage outside the container is described, not built.
    - Tested: TestPerbaruiKoneksi with a proxy that cuts every connection after 1 s; it fails without renewal. No real tunnel or Cloud Run run.
  - Bagian 9, "Merawat VPN dan menutup seri": written, the last part.
    - It covers starting on boot (systemd unit, `schtasks /sc onstart /ru System`), the member waiting for the relay unless rejected, `wintun.dll` embedded with `go:embed`, builds for Raspberry Pi (`arm64`, `arm` with `GOARM=6`), and self-update from HTTPS with `SHA256SUMS` and rename.
    - It closes with the series recap, the costs of the whole design, and when to choose plain WireGuard, Tailscale, Headscale or ZeroTier.
    - Tested: TestMulaiSebelumRelay and 4 `update` tests. All four release files were cross-built, `-version` printed `v1.0.0`, and replacing a running `.exe` was checked on Windows.
    - **Not run:** the systemd unit, the scheduled task and the embedded Wintun need administrator rights.
  - Wording revised on 2026-10-11, at the user's request.
    - A design's cost is "kelemahan" or "hal yang dikorbankan", not "harga"; the closing sections are "Kelemahan VPN versi …".
    - "anggota" became "client" everywhere: prose, titles and file names of Bagian 4, 6 and 7, diagrams, and code (`cmd/client`, `client.json`, "vpnku client", "bukan client terdaftar", test names). The relay stays "relay", introduced as the server its clients connect to. Go identifiers were already English (`Member`) and were kept.
    - The modules `vpnku2` … `vpnku9` were renamed the same way and passed `go vet` (Windows and Linux), builds for linux amd64, arm64 and arm, `go test -count=20` and `-race` on Windows. The `go test -v` output in the articles was rerun.
    - The real-TUN transcripts in Bagian 4 to 6 (Docker and WSL) were edited for the rename only, not rerun: the binary name, the "client A" labels and the refusal message.

## Structure

- **Argument:** articles are written dialectically, as in [dialectical_writing.md](dialectical_writing.md). Each idea is stated, tested against the strongest objection or alternative, and resolved with its conditions and cost. Run its checklist before handing an article over.
- **Title:** `# ` heading, the only H1.
- **Subtitle:** one italic line right under the title; Medium shows it as the subtitle.
- **Headings:** only `##` and `###`. Medium has two heading sizes.
- **Sections:** the first sentence of each one carries its point.
- **Paragraphs:** at most three sentences. Keep sentences short and words plain.
- **Lists:**
  - Bullets for parallel items, numbers for steps.
  - No nested lists and no tables: Medium supports neither. Turn a table into a list, or into a diagram.
- **Code:** fenced blocks with a language tag. Medium shows them without colours.

## Facts

- **Sources:** facts come from the project docs (pull them with `knowledge_explain`), from the code itself, or from a source that was actually opened. Never invent a number, a date or a result. What comes from our docs or code is told as general knowledge, never cited (see "Educational only").
- **Statistics:** figures from BPS or APJII quoted through news coverage are flagged as such.
- **State of the work:** what is unverified or not done yet is stated plainly, as it stands in the design docs. Check it again before the user publishes.
- **Example addresses:**
  - private ones: generic, such as `10.10.0.x`, never our defaults
  - public ones from the documentation ranges, never real hosts: `192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`

## Diagrams

Medium takes images, not Markdown diagrams. Draw each one as SVG, render it to PNG, and keep both in `assets/`.

- **Purpose:** a diagram earns its place when the content has a shape: a packet layout, an architecture, a flow. A list of facts stays text.
- **Title:** the first line states the point ("Paket VPN adalah paket di dalam paket lain"), not the topic. A quiet subtitle explains how to read it.
- **Frame:** `viewBox="0 0 760 H"` with a 24-unit margin, drawn at 2× (`width="1520"`) so the PNG stays sharp on Medium.
- **Font:** `-apple-system, 'Segoe UI', Helvetica, Arial, sans-serif`. Title 15 at weight 600, labels 12 to 13, notes 11.5.
- **Line breaks:** SVG does not wrap text, so break long lines yourself and check that each one fits.
- **Colours** (fixed, on white):

  | Role | Colour |
  |---|---|
  | text | `#1f1f1f` |
  | quiet text | `#5f5f5f` |
  | borders and arrows | `#a8a69c` |
  | box tint | `#f1f0eb` |
  | accent | `#2f76d6` (fills at 0.07–0.14 opacity) |
  | background | `#ffffff` |

  Use one accent per diagram, on the thing the title is about.
- **Drawing order:** filled containers first, connectors after, so arrows stay visible.
- **In the article:** alt text describes what the picture shows, in at most 500 characters (Medium keeps no more, and `san_medium` refuses a longer one). An italic caption follows on the next line:

  ```markdown
  ![Dua baris: ...apa yang digambar...](assets/vpn-packet-inside-a-packet.png)

  *Apa yang dikirim aplikasi, dan apa yang dilihat jaringan.*
  ```

Render with headless Edge, using a profile folder of its own in the scratchpad. Then look at the PNG before using it, and check that no `msedge.exe` is left running.

```sh
"/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" --headless=new --disable-gpu \
  --hide-scrollbars --user-data-dir="<scratchpad>\edge-shot" --window-size=1520,<2×H> \
  --screenshot='D:\wargasipil\san_knowledge\docs\articles\<topic>\assets\<name>.png' \
  "file:///D:/wargasipil/san_knowledge/docs/articles/<topic>/assets/<name>.svg"
```

## Publishing to Medium

Medium no longer issues API tokens. `golang/packages/san_medium` makes the draft instead by driving a logged-in Chrome through Medium's own editor. Publishing stays with the user.

1. Once: `bin\san_medium.exe login` (from `golang/packages/san_medium`), sign in to Medium in the window that opens, then close it.
2. `bin\san_medium.exe draft -dry-run <article.md>` shows how the article will be cut. Then `bin\san_medium.exe draft <article.md>` writes the new draft, uploads each image from `assets/` with its alt text and caption, and prints the draft's URL.
3. The user opens the draft, adds tags, and publishes.

Write drafts one at a time. Cloudflare in front of Medium blocks the Medium sign-in session when chromedp launches Chrome itself (2026-10-11), so the tool starts Chrome as a plain process and attaches to it. If `draft` ever stops with "Cloudflare blocks this browser", run `login` on a new profile (`-profile <dir>`).

By hand, as before: open the article in VS Code, press `Ctrl+Shift+V` for the preview, then select all of the preview, copy it, and paste it into a new Medium story. Medium does not convert pasted Markdown, but it keeps the formatting of pasted HTML. Then upload each PNG from `assets/` where its image appears; pasted images do not come through.

## Knowledge graph

`docs/articles/` is tracked. Ask the user before syncing after article edits (CLAUDE.md, "Research workflow").

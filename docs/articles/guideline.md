# Article guideline

How articles for Medium are written and stored in this repo. Articles are drafts; the user publishes them to Medium by hand.

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
  - After writing, extract the article's code blocks and diff them against the tested files, so what readers copy is what ran.
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
- **Closing:** each part ends with `## Berikutnya dalam seri ini`, a short teaser for the next part. Update the teaser when the plan changes.

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
- **In the article:** alt text describes what the picture shows, and an italic caption follows on the next line:

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

The user does this by hand:

1. Open the article in VS Code and press `Ctrl+Shift+V` for the preview.
2. Select all of the preview, copy it, and paste it into a new Medium story. Medium does not convert pasted Markdown, but it keeps the formatting of pasted HTML.
3. Upload each PNG from `assets/` where its image appears; pasted images do not come through.

Medium no longer issues API tokens, so there is no tool that posts for us.

## Knowledge graph

`docs/articles/` is tracked. Ask the user before syncing after article edits (CLAUDE.md, "Research workflow").

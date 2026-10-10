# San VPN

San VPN joins machines at home, at the office and in other regions into one
private network. Each machine runs WireGuard, embedded in the binary, but the
WireGuard packets travel over a WebSocket to a relay instead of over UDP. That
makes the network work wherever outbound HTTPS works. It runs behind
carrier-grade NAT and office firewalls, and through a Microsoft dev tunnel (the
"GitHub tunnel" behind `code tunnel`), so no machine needs a public IP.

The operator guide is the package
[README](../../golang/packages/san_vpn/README.md).

## General.
1. it is written in Go
2. package is `github.com/wargasipil/san_vpn`
3. folder is `./golang/packages/san_vpn` as git submodule, tracking
   `https://github.com/wargasipil/san_vpn.git` on `main`
4. WireGuard is embedded: `golang.zx2c4.com/wireguard` (wireguard-go), with
   Wintun on Windows and the kernel TUN device on Linux
5. the transport is WebSocket only (`coder/websocket`), through one relay.
   There is no UDP path and no ngrok.
6. the relay sits behind a Microsoft dev tunnel by default. Any HTTPS front
   works, because members only need a URL.
7. topology: full mesh. Every member is a WireGuard peer of every other.
8. reachable: only the machines running san_vpn, on an overlay range
   (`10.77.0.0/24`), not their LANs
9. platforms: Windows and Linux, amd64
10. entry point is `./golang/packages/san_vpn/cmd/san_vpn`. One binary, two
    roles: `san_vpn relay ...` on the relay machine, and `san_vpn join` /
    `san_vpn up` on each member.
11. the cli is `urfave/cli` v3
12. installing as a background service (Windows service, systemd) is left for
    later; `up` runs in the foreground
13. `san_vpn setup init` sets up the relay and its dev tunnel in one command,
    and `san_vpn setup check` checks a machine end to end (asked for
    2026-10-07)
14. a member can keep memberships of several networks as profiles, such as
    the dev tunnel relay's and the Cloud Run relay's, and switch between them
    (asked for 2026-10-08; section "Profiles.")
15. members find each other by name, `office.vpn`, answered by each member
    inside its own tunnel (asked for 2026-10-10; section "Names.")

These were decided with the user on 2026-10-07. The steps were: embedded
WireGuard, full mesh, overlay only, Windows and Linux, then WebSocket instead
of UDP, then no ngrok (its free plan has a 1 GB/month cap and no UDP), then
dev tunnels as the front, then WebSocket-only with no direct UDP, and service
install later.

## Layout.

```
golang/packages/san_vpn/
  cmd/san_vpn/main.go         urfave/cli v3 tree: relay init/run/invite/list/remove, join, up, status
  cmd/san_vpn/setup.go        setup init / setup check
  cmd/san_vpn/cloudrun.go     cloudrun setup / deploy, and which source deploy builds
  cmd/san_vpn/profile.go      profile list/use/rename, and which profile a member command works on
  cmd/san_vpn/update.go       update: the latest or a named release, in place of this binary
  cmd/san_vpn/main_test.go    the admin flow through the real command tree
  internal/wire/              the protocol both sides speak
               keys.go        X25519 keys (one type for both halves), base64/hex forms
               wire.go        frames, control messages, join MAC, handshake proofs,
                              the DNS address and the domain
               ws.go          keepalive, control message read/write
  internal/names/             members' names (office.vpn)
               names.go       the table from a member list, DNS answers
               intercept.go   catches DNS queries to the DNS address in the tunnel
               hosts.go       the /etc/hosts block
  internal/invite/            the sanvpn1_ invite blob
  internal/state/             JSON files: default dirs, lock, atomic save, Windows ACL
                   store.go   the relay's file as a Store: local File, or a GCS object
                   profile.go a member's profiles: folders, listing, the current one, rename
  internal/gcs/               Cloud Storage JSON API, three calls, generation preconditions
               gcstest/       in-memory Cloud Storage for tests
  internal/relay/  state.go   members, invites, address allocation
                   server.go  join endpoint, WebSocket sessions, forwarding, reload
  internal/node/   config.go  node.json, Join
                   bind.go    WireGuard's conn.Bind, carried by the relay
                   uapi.go    netmap -> WireGuard peer config diff, stats parsing
                   node.go    the device, relay session, reconnects, status
                   e2e_test.go relay + members + WireGuard on gVisor netstack
  internal/devtunnel/         the devtunnel CLI: find, install, JSON calls, supervised `host`
  internal/setup/  setup.go   Init: sign in, relay, tunnel, anonymous access, port, URL
                   cloudrun.go CloudRunSetup: APIs, bucket, service account, key, image repo, build account;
                               CloudRunDeploy: Cloud Build from source, deploy, URL, check
                   gcloud.go  the gcloud CLI, as devtunnel.CLI is the devtunnel one
                   gcloud_windows.go  gcloud.cmd through cmd /s, quoting each argument
                   check.go   Check: relay + tunnel + member, each failure with its fix
  internal/update/            release links, SHA256SUMS, replacing a running binary
                   source.go  a tag's source tree, from GitHub's archive of it
  internal/osnet/             the only admin-only code: TUN, address, firewall, names
                   osnet_windows.go  Wintun (embedded dll), winipcfg, netsh rule, NRPT rule
                   osnet_linux.go    TUN, netlink address, resolvectl or /etc/hosts
                   wintun/           fetched by the build scripts, git-ignored
  build.ps1 / build.sh        fetch Wintun (checksum pinned), vet, test, build Windows, Linux and Linux ARM
  .github/workflows/release.yml  on a v* tag: build.sh, the ghcr.io image (for Docker), then a GitHub release
  Dockerfile                  the relay as an image: built from source, on distroless static
```

## Releases.

Pushing a version tag (`git tag -a v0.2.0 -m "san_vpn v0.2.0"`, then
`git push origin v0.2.0`) runs `.github/workflows/release.yml` on GitHub
Actions. It runs `build.sh` with `SAN_VPN_VERSION` set to the tag, so
`--version` prints the tag; local builds keep the time stamp. The release gets
`san_vpn-windows-amd64.exe`, `san_vpn-linux-amd64`, `san_vpn-linux-arm64`,
`san_vpn-linux-arm`, `wintun-LICENSE.txt` and `SHA256SUMS`, with the README's
install notes and GitHub's generated changelog. Asset names carry no version, so
`releases/latest/download/san_vpn-linux-amd64` always fetches the newest. A tag
with a `-` (`v0.2.0-rc1`) becomes a prerelease, which "latest" skips.

The ARM files are for the Raspberry Pi (asked for 2026-10-08), so a Pi can
`san_vpn update` too. `san_vpn-linux-arm64` is for a 64-bit Raspberry Pi OS
(Pi 3, 4, 5, Zero 2 W; `uname -m` says `aarch64`). `san_vpn-linux-arm` is for
a 32-bit one (`armv6l`, `armv7l`). It is built with `GOARM=6`, so it runs on
every Pi, the Pi 1 and Zero included; ARMv7 would leave those out. Both are
static (`CGO_ENABLED=0`), so the arm64 file also runs on a 32-bit userland
with a 64-bit kernel. Releases before v0.4.1 have no ARM files, and older
binaries do not know these names, so a Pi's first binary is a download.

- **v0.1.0** (2026-10-07): the first release. CI built it in 1m32s. The
  downloaded files matched `SHA256SUMS`, and the downloaded exe printed
  `san_vpn version v0.1.0`.
- **v0.2.0** (2026-10-07): `san_vpn update` (next section) and urfave/cli
  v3.14.0. CI built it in 1m5s. Windows and Linux builds stamped `v0.0.9`
  updated themselves to it, and the results matched its `SHA256SUMS`.
- **v0.3.0** (2026-10-08): the Cloud Run relay (section "Cloud Run.") and
  `setup init` without winget. CI built it in 1m59s, the first build to also
  push the container image, `ghcr.io/wargasipil/san_vpn:v0.3.0` and
  `:latest`, sha256:5645ff16…. The downloaded files matched `SHA256SUMS`, and
  the exe printed `san_vpn version v0.3.0`. The image's package started
  **private**: an anonymous pull got 403.
- **v0.4.0** (2026-10-08): profiles (section "Profiles."). CI built it in
  2m26s and pushed `ghcr.io/wargasipil/san_vpn:v0.4.0` and `:latest`,
  sha256:ab4a9634…. The downloaded files matched `SHA256SUMS`, the exe
  printed `san_vpn version v0.4.0`, and a v0.3.0 exe updated itself to it
  with `san_vpn update`. The package is **still private**: an anonymous pull
  of either tag got 403.
- **v0.4.1** (2026-10-08): the Raspberry Pi files `san_vpn-linux-arm64` and
  `san_vpn-linux-arm`, and the relay waiting for a new dev tunnels sign-in
  (section "Setup.", under "Hosting."). CI built it in 2m33s and
  pushed `ghcr.io/wargasipil/san_vpn:v0.4.1` and `:latest`,
  sha256:d6af92c0…. The downloaded files matched `SHA256SUMS`. In Docker,
  each file ran `update --force` and replaced itself with its own release
  file: `linux-arm` on emulated ARMv6, `linux-arm64` on emulated arm64. v0.4.0
  binaries for Linux and Windows updated themselves to it. The package is
  **still private**: an anonymous token request got 401.
- **v0.4.2** (2026-10-09): `setup cloudrun` on Windows when gcloud sits under
  a path with a space (section "Cloud Run.", "gcloud on Windows"). CI built it
  in 2m11s and pushed `ghcr.io/wargasipil/san_vpn:v0.4.2` and `:latest`,
  sha256:85f2b446…. The downloaded files matched `SHA256SUMS`, and the exe
  printed `san_vpn version v0.4.2`. The package is **still private**: an
  anonymous token request got 401.
- **v0.4.3** (2026-10-09): `cloudrun setup` and `cloudrun deploy`, which
  builds the relay from source with Cloud Build instead of pulling the GHCR
  image (section "Cloud Run."). CI built it in 2m42s and pushed
  `ghcr.io/wargasipil/san_vpn:v0.4.3` and `:latest`, sha256:10972041….
  The downloaded files matched `SHA256SUMS`; the exe printed `san_vpn
  version v0.4.3`, listed both commands, and `setup cloudrun` named them.
  GitHub served the tag's source archive (115,689 bytes).
- **v0.5.0** (2026-10-10): names, `office.vpn` (section "Names.").
  - CI built it in 2m40s and pushed `ghcr.io/wargasipil/san_vpn:v0.5.0` and
    `:latest`, sha256:b048b53b….
  - The downloaded files matched `SHA256SUMS`. The Windows and Linux files
    printed `san_vpn version v0.5.0`.
  - A v0.4.3 exe updated itself to it with `san_vpn update`, and then had
    `up --no-dns`.
  - The package is **still private**: an anonymous token request got 401.

## Updates.

`san_vpn update` replaces the binary with the latest release, or with
`update <tag>` (asked for 2026-10-07). `--check` only reports.

- **Web links, not the API.** GitHub's API allows 60 unauthenticated calls an
  hour per IP, and behind an Indonesian ISP's carrier-grade NAT one IP is
  shared by many customers. `releases/latest` answers with a redirect to
  `releases/tag/<tag>` (drafts and prereleases skipped), and asset names carry
  no version, so `releases/download/<tag>/<asset>` needs nothing else.
- **Checked twice before it replaces anything.**
  - The download must match the release's `SHA256SUMS`.
  - The new binary's `--version` must print `san_vpn version <tag>`, which
    also proves it runs on this machine.

  Both files come from the same release, so this catches a broken or
  truncated download, not a compromised repository.
- **Replacing a running binary.**
  - Linux: one rename over the old file, keeping its mode. Running processes
    keep the old file open.
  - Windows refuses to overwrite or delete a running exe but lets it be
    renamed. The old one moves to `san_vpn.exe.old`, and every later start
    deletes it once nothing runs from it. If an un-restarted `up` still holds
    an older `.old`, the next one gets a name of its own.
- **Which versions.** Tags compare as `vMAJOR.MINOR.PATCH[-pre]`. A local
  build is stamped with its build time and is not compared: replacing it takes
  `--force`. A named tag is installed even when it is older.
- **Not writable** (`/usr/local/bin`, `Program Files`): the error says to use
  `sudo` or an administrator terminal, and nothing is left behind.
- **Verified** (2026-10-07):
  - tests against a fake GitHub that redirects like the real one
  - a Windows build stamped `v0.0.9` updated itself from the real v0.1.0
    release while running; the result matched `SHA256SUMS`, and the next
    start removed `san_vpn.exe.old`
  - Docker (alpine): as `nobody`, refused with the hint; as root,
    `/usr/local/bin/san_vpn` became v0.1.0 with mode 755

  v0.2.0 is the first release with `update`. A machine on v0.1.0 needs one
  manual download of it, and updates itself from then on.

## Design decisions.

**1. WireGuard is unchanged; only its socket is replaced.** wireguard-go takes
a `conn.Bind`, the thing that sends and receives datagrams. Our bind
(`node/bind.go`) hands each packet to the relay, addressed by the peer's
public key, instead of sending a UDP datagram to an IP and port. The handshake,
the keys, the timers, rekeying and replay protection are all WireGuard's own.
We wrote no cryptography for the data path.

**2. An endpoint is a public key.** WireGuard's "where is this peer" becomes
"which key should the relay deliver to". So the UAPI `endpoint=` of every peer
is its own key in hex, and roaming has nothing to follow. Each key also gets a
stable, private `fd..` IPv6 address as its `DstIP`. Without that, WireGuard's
handshake rate limiter, which groups by source IP, would put all peers in one
bucket.

**3. Full mesh in keys, star in transport.** Every member is a WireGuard peer
of every other, so the relay forwards only ciphertext. It never holds a key that
decrypts traffic. Neither does the dev tunnel, whose TLS ends at Microsoft:
behind that TLS is still WireGuard. A relay-as-hub design, where the relay is
the one WireGuard peer and routes packets, would have let it read everything.

**4. The relay authenticates nodes by their WireGuard key.** X25519 keys
cannot sign, but they can agree on a secret. On connect:
- The relay sends a fresh nonce.
- The node answers with its public key, its own nonce, and
  `HMAC(X25519(node, relay), "node" ‖ nonces)`.
- The relay checks that proof and answers with the same MAC under the label
  `"relay"`.

Only the two key holders can compute that secret. So each side proves
possession, a proof cannot be replayed on a later connection, and the labels
stop a proof from being reflected back. The node checks the relay's proof
against the relay key in its invite. A hijacked URL or a hostile front cannot
act as the relay (`TestImpostorRelayRefused`). There is one key per member,
used both for the relay and for WireGuard; there is no second credential to
leak or rotate.

**5. The relay sets the source.** A data frame is a binary WebSocket message:
32 bytes of key, then the WireGuard packet. From node to relay, the key is the
destination. Before delivering, the relay overwrites it, in place, with the
sender's authenticated key. A node therefore cannot pose as another node
(WireGuard would reject it anyway). Control messages are JSON text messages, so
a packet can never be read as a command.

**6. Joining spends an invite, and the secret never travels.** `relay invite`
records an id, a 32-byte secret and the member's name. The `sanvpn1_` blob
carries the relay URL, the relay key, the id and the secret.
- `join` generates the private key on the machine itself.
- It posts the public key with `HMAC(secret, id ‖ public key)`.
- The MAC binds the invite to that key. Someone copying the request off an
  unencrypted path cannot use it for a key of their own, and the invite is
  spent by the time they could replay it.
- Unknown, used, expired and wrong-MAC invites all fail with the same 403, so
  a guesser learns nothing.

**7. No admin port.** `relay invite` and `relay remove` edit `relay.json`
directly, under a lock file (`O_EXCL`; stale after 30s). The running relay
polls the file's modification time every second and reloads. A reload
disconnects removed members with a reason and pushes the new member list to
everyone else. Nothing listens except the relay's one HTTP port, so the dev
tunnel exposes nothing administrative.

**8. Peer changes are diffs, never `replace_peers`.** Rewriting all peers on
each member-list change would throw away every session. Then a member joining
would stall traffic between all the others until they handshake again.
`peerUpdate` removes departed peers, adds new ones and leaves the rest alone.

**9. Reconnects reset sessions on purpose.** The netmap tags each online peer
with `conn`, a random id for its current relay connection. A peer whose `conn`
changed may have restarted and lost its WireGuard sessions. In that case the
session is dead on one side only, and plain WireGuard notices only after 15
seconds of unanswered packets. So:
- A node resets its session with a peer as soon as that peer's `conn` changes.
- After its own reconnect, a node resets its session with every peer, because
  each of them will be resetting theirs with it.

The reset is a remove plus add in one UAPI write: the session is dropped but
the route stays. The next packet starts a fresh handshake, and recovery takes
about one round trip, not 15 seconds (`TestReconnectAfterNodeRestart`,
`TestRelayRestart`).

**10. Drop, never block.** The bind's queues and the relay's queue to each
member are bounded (1024 and 512 packets). When full, packets are dropped, the
way a UDP socket's buffer would drop them. WireGuard and the TCP inside it
recover from loss. If we blocked instead, one stalled member would hold up
WireGuard's own goroutines, or every sender at the relay. A write that makes no
progress for 15 seconds closes that connection.

**11. Both ends ping.** WebSocket pings every 25s (15s timeout) from the node
and from the relay. Each side notices a vanished peer, such as a laptop that
slept or a dropped dev tunnel, within about 40s, and fronts that close idle
connections keep seeing traffic. This is the same approach as san_tunnels'
`wsconn`.

**12. Shutdown tells the members.** `http.Server.Shutdown` stops tracking a
connection once it becomes a WebSocket. So `relay.Server.Shutdown` kicks each
session with "relay is shutting down", and members redial at once instead of
waiting out a keepalive. Reconnect backoff is 1s doubling to 30s, with jitter.
It resets after a connection that lasted a minute.

**13. One binary to copy.** The Windows build embeds `wintun.dll`. Its license
allows shipping it unmodified alongside software that uses its API. `up` writes
it next to the exe, which is where the Wintun loader looks. The DLL is not
checked in: `build.ps1` and `build.sh` download Wintun 0.14.1 and check its
SHA-256 (`07c25618...`).

## Setup.

`setup init` replaces the manual dev tunnel steps (install the CLI, sign in,
create, allow anonymous, add the port, copy the URL into `relay init`).
`relay run` then hosts the tunnel itself.

- **The CLI is the interface.** No Go SDK can host a dev tunnel, so
  `internal/devtunnel` runs the `devtunnel` CLI, using only its machine
  side:
  - every call takes `--json --nologo`
  - `show` answers `{"tunnel": {tunnelId, hostConnections, ports: [{portNumber, portUri}]}}`
  - `user show` answers `{status: "Logged in" | "Not logged in" | "Login token expired", provider, username}`,
    with exit 0 in all three cases
  - exit code 1 means "already exists", 2 means "not found", and an
    unauthenticated call is exit 3 with "Login required."

  This is the surface .NET Aspire's dev tunnels integration
  (`Aspire.Hosting.DevTunnels`) relies on. The version, `user show` and
  unauthenticated outputs were confirmed against CLI 1.0.2094.
- **Every step looks before it acts**, so a rerun changes nothing on a working
  setup and repairs only what broke:
  - sign-in: `user show`, then `user login -g` attached to the terminal
    (`-d` device code on Linux without a display)
  - relay key: created once, never replaced
  - tunnel: the recorded one; if the service deleted it (30 days unhosted), a
    new one with a warning
  - anonymous access and the port: checked first, added only if missing
- **Tunnel names are random:** `san-vpn-` plus six characters. A fixed name
  such as `san-vpn` can belong to someone else. A taken generated name is
  retried twice; a name given with `--tunnel` is used as given.
  The id is stored as the service reports it, with its region
  (`san-vpn-k3x9qa.asse`; `asse` is Southeast Asia, nearest for
  Indonesia), together with the port, in `relay.json` under `tunnel`.
- **The URL** is the port's `portUri` from `show`. If that is missing, it is
  built from the documented shape `https://<name>-<port>.<region>.devtunnels.ms`.
- **Installing the CLI without winget.** Windows Server, LTSC and older
  Windows 10 have no winget, and an old winget can fail on its sources. So on
  Windows `Install` tries winget when it is on PATH. If winget is missing or
  fails, it downloads Microsoft's direct link
  `https://aka.ms/TunnelsCliDownload/win-x64` to
  `%LocalAppData%\san_vpn\devtunnel.exe`.
  - The link served a 24 MB `devtunnel.exe` on 2026-10-07. It was
    Authenticode-signed by Microsoft Corporation and was the same 1.0.2094 that
    winget installs.
  - There is no `win-arm64` link: it redirects to a Bing page with a 200. A
    retired link would do the same, so the download is kept only if it starts
    with `MZ`, and it replaces the file only once it is complete.
  - The publisher's signature is not checked in code. `x/sys/windows` has no
    `CryptMsgGetParam`, so reading the signer would mean loading crypt32 by
    hand. Trust rests on HTTPS to Microsoft's hosts, as with the Linux
    `curl | bash`.
  - Checked on this machine with winget hidden, and with a fake winget that
    exits 1. Both fell back, downloaded, and ran `--version`.
  - Go's floor is Windows 10 / Server 2016, so san_vpn cannot run on Windows
    7 or 8.1 anyway.
- **Finding the CLI right after installing it.** Installers update PATH for
  new terminals only. winget (1.29) makes no shim in `WinGet\Links`: it adds
  its package folder to the user PATH in the registry. So `Find` looks in this
  order:
  1. our PATH
  2. the user and machine PATH read fresh from the registry
  3. winget's `Links` and `Packages\Microsoft.devtunnel_*` folders
  4. the downloaded `%LocalAppData%\san_vpn\devtunnel.exe`
  5. `~/bin` and `/usr/local/bin` on Linux
- **Hosting.**
  - `relay run` starts `devtunnel host <id> --nologo` once it is listening
    on `127.0.0.1:<port>`. It logs the host's output and restarts it with
    backoff (5s doubling to 1 min) when it exits.
  - An expired sign-in is not a drop. The CLI's sign-in lasts "several days"
    (Microsoft's CLI reference), and nothing renews it. On 2026-10-08 the
    host printed "Login token expired." and exited with 3, and the relay
    restarted it every minute with only a warning. Since v0.4.1, after every
    exit the relay asks `user show`. If the sign-in is not valid it logs one
    error naming the fix (`devtunnel user login -g`, or `setup init`), checks
    every 15s, and hosts at once after a new sign-in, with no relay restart. A
    failing `user show` falls back to the plain restart.
  - Members ride it out on their own: `up` redials with backoff (1s doubling
    to 30s, jittered to its second half) and never gives up, so they are back
    within about 30s of the tunnel.
  - The child cannot outlive the relay, even if the relay is killed: on Windows
    it sits in a job object set to kill on close, and on Linux it gets a
    parent-death signal. Otherwise a killed relay would leave a tunnel served
    to nothing.
  - `--no-tunnel` opts out.
- **`setup check` covers both roles.**
  - For the relay: the CLI, the sign-in, the tunnel, anonymous access, the port
    against the recorded URL, `hostConnections`, an HTTP GET locally and
    through the URL, and a WebSocket that must receive the relay's challenge.
    Some proxies carry HTTP but not WebSockets.
  - For a member: `node.json`, a probe handshake, and `status.json`.
  - Every failure carries the command that fixes it, and the exit code is 1.
- **Probe handshakes.** The member check authenticates with the member's own
  key while that member's `up` may be connected. A normal second connection
  would replace the first (decision 9). So the auth message carries
  `probe: true`: the relay verifies and welcomes, then hangs up without
  attaching (`TestProbeLeavesTheRunningNodeAlone`). Connections that close
  before sending auth, such as health checks and the relay check's challenge
  test, are logged at debug, not as refusals.

## Network.

- **Range.** The overlay is `10.77.0.0/24` by default (`relay init --network`).
  - Home and office routers rarely use 10.77.x, unlike 192.168.x and 10.0.x.
  - It also stays clear of `100.64.0.0/10`, the range Indonesian ISPs use for
    carrier-grade NAT on the WAN side. That is what Tailscale uses, and it can
    collide here.
- **Addresses.** They are given out lowest-free-first from `.1`, and a removed
  member's address is reused. Peers trust keys, not addresses. The last host
  address (`.254`) is kept for names (section "Names.").
- **The interface address carries the network's prefix length**
  (`10.77.0.2/24`), so the route to the whole overlay comes with it.
  - On Linux, the kernel adds that route itself (netlink `AddrReplace`).
  - On Windows, `Configure` follows wireguard-windows: set routes, then
    addresses (clearing a stale copy held by a dead adapter), then the
    interface row (no router discovery, no DAD delay, the MTU).
- **MTU.** The default is 1420, WireGuard's own. The outer path is TCP, so
  nothing fragments, and a larger MTU would work and cut per-packet overhead
  (`up --mtu`).
- **Adapter GUID (Windows).** It is derived from the node's key, so it is the
  same on every start. Windows keys network profiles by GUID, and a fresh GUID
  would create a new "Network N" profile each time, each one Public.
- **Firewall (Windows).** A new adapter is Public, which blocks inbound traffic,
  even ping. `up` adds one rule, `san_vpn`: inbound, any protocol, from the
  overlay range to our own overlay address only. On Linux the firewall is left
  alone (`ufw allow in on sanvpn0`).

## Files.

- **The relay** (`relay.json`): its private key, URL, network, members and
  invites. It lives in the user config dir (`%AppData%\san_vpn`,
  `~/.config/san_vpn`) because the relay needs no privileges.
- **A member** (`node.json`, `status.json`): it lives in a system dir
  (`C:\ProgramData\san_vpn`, `/var/lib/san_vpn`). Creating the interface needs
  an administrator anyway, and a future boot service will not be any
  particular user.
  - On Windows, ProgramData lets every user read what is created in it. So when
    the process is elevated, the directory gets an explicit DACL: SYSTEM and
    Administrators, full control, inherited.
  - An unelevated `join --state <dir>`, for trying things out, keeps the
    directory owner-only instead, so the user can read back what they wrote.
  - Other profiles keep the same two files in `profiles/<name>/`, and
    `profile.json` names the current one (section "Profiles.").
- **Writes.** They go to a temp file that is renamed over the original, with
  retries, because Windows refuses the rename while a reader has the file open.
- **Status.** `up` rewrites `status.json` every 2s and deletes it on exit.
  `status` treats a file older than 10s as "not running", which covers a crash.
  This avoids an IPC channel that would need its own access control.

## Verification.

- **Unit tests:**
  - wire: key clamping, low-order rejection, proof properties
  - invite: round trip and junk
  - state: 100 concurrent locked increments, stale lock
  - relay state: allocation, every join refusal, single use, pruning, removal,
    a full /30
  - UAPI diff and reset, stats parsing
  - URL normalisation
- **End to end in-process** (`internal/node/e2e_test.go`): a real relay over
  HTTP, invites, joins, WireGuard on both sides. gVisor's netstack stands in for
  the OS tunnel, so no admin is needed. The tests cover:
  - TCP both ways, and a third member joining later
  - removal
  - a member restarting, and the relay restarting
  - an impostor relay, a non-member key, an invite used twice
  - stable over `-count=10` and clean under `-race`
- **CLI** (`cmd/san_vpn/main_test.go`): `relay init` (idempotent), `invite`,
  `join` (twice is refused), `join --url`, `list`, `remove`, `status`.
- **Real kernel, Linux** (2026-10-07, Docker, alpine, `NET_ADMIN` +
  `/dev/net/tun`): one relay container and two members.
  - `join` and `up` created `sanvpn0` with 10.77.0.1/24 and the route.
  - ping took about 1 ms; TCP worked both ways.
  - 20 MB crossed in 0.47 s (containers on one host, so not a WAN figure).
  - `status` showed the handshake and byte counts.
  - after `docker restart` of the relay, members came back on their own.
  - `relay remove` cut a member off; it was refused on redial and unreachable.
  - SIGTERM removed the interface.
- **ARM, emulated** (2026-10-08, Docker with QEMU, busybox): every package's
  tests, cross-compiled as ARMv6 and as arm64 binaries, passed. Not yet run on
  a real Pi, so `up` with a real TUN there is untested.
- **Setup** (`internal/setup`, `internal/devtunnel`), against a fake dev
  tunnels service that speaks the CLI's JSON and exit codes:
  - init from scratch: signs in first, makes a random id, the port and the URL
  - idempotence: no changes on a rerun, and the key is kept
  - repair: a deleted tunnel, anonymous access turned off
  - a taken name, and adopting a named tunnel
  - the URL fallback
  - check: healthy, not hosted, member not running, member removed,
    nothing set up
  - the host supervisor, using the test binary as a fake `devtunnel`:
    output is logged, a restart happens, and cancel stops it. With the
    sign-in expired it logs once, does not restart, and hosts again once
    `user show` says "Logged in" (`TestHostWaitsForSignIn`)
  - real CLI 1.0.2094 on this machine: `setup check` found it through the
    registry PATH, read "Not logged in", and reported the fixes
  - not yet run: a full `setup init` with a real GitHub sign-in. It needs the
    user's account.
- **Not yet verified on a real Windows machine:** `up` with Wintun, the
  address, the firewall rule. It needs an elevated terminal, and the session
  that built this had none. Everything above the OS layer is the same code the
  Linux run exercised.

## Cloud Run.

`cloudrun setup` and `cloudrun deploy` run the relay on Google Cloud Run, as
the other way to reach it besides a dev tunnel (asked for on 2026-10-07). Until
v0.4.2 this was one command, `setup cloudrun`, which ran the release's image
from GHCR; since v0.4.3 the image is built from source in the user's project
(asked for on 2026-10-09, see "The image."). `setup cloudrun` is now a hidden
command that names the two new ones. Everything below was checked locally;
the user runs the real setup and deploy.

**What Cloud Run changes, and how the relay meets it:**

- **The disk does not outlive a restart.** The relay's file moves to a Cloud
  Storage object.
  - `--state gs://<bucket>[/<folder>]` works for every relay command, not only
    on the service.
  - `state.Store` has two kinds, `File` and `GCS`, and keeps the promise the
    lock made: no writer loses another's change. GCS does it with
    `ifGenerationMatch`: a loser starts over from fresh content, after a
    random pause that grows (up to 20 tries).
  - The local file's version became a hash of its content. Its modification
    time missed two quick saves of the same size, which share one clock tick
    on Windows.
  - The relay polls every 5 s on GCS (a Class B request each time; about
    US$0.20 a month) and every 1 s on disk.
  - Admin commands run from any machine signed in to gcloud. Tokens come from
    the metadata server when `K_SERVICE` or `GCE_METADATA_HOST` is set,
    otherwise from `gcloud auth print-access-token`. `STORAGE_EMULATOR_HOST`
    points everything at an emulator, without credentials.
  - The client is hand-written over the JSON API. Three calls (get with
    `alt=media`, get the generation, media upload) do not justify Google's
    SDK and the megabytes it adds.
- **Instances.** Members only reach each other inside one process, so the
  service has `--max-instances 1` and concurrency 1000. Cloud Run may briefly
  run two during a deploy; members reconnect.
- **Requests end at the timeout, at most 1 h.** Each member's connection is
  one request.
  - The relay gets a session limit (`--session-limit`,
    `SAN_VPN_SESSION_LIMIT`). The welcome message carries `renew_after_ms`:
    the limit minus a tenth, at most 5 min before. Each member renews at a
    random point in the last twentieth before that.
  - A renewal opens the new connection while the old one still carries
    traffic. It switches when the relay sends the first member list on the new
    one, which proves the relay has attached it.
  - The new connection's auth carries `resume`: the old connection's id. The
    relay gives the new connection that same id, so peers see a member that
    carried on, not one that restarted, and keep their WireGuard sessions.
    Without it, every renewal reset every peer's handshake.
  - If renewal fails, the member retries every 30 s until the old connection
    dies, then reconnects as usual. The relay ends a connection that reaches
    the limit with "session limit reached", so an old member is cut, not
    stranded.
  - Both new fields are optional JSON, so old members and old relays still
    work together, without renewal.
- **`$PORT`.** `relay run` listens on `:$PORT` when it is set, there is no
  dev tunnel, and no `--listen` is given.
- **The image.**
  - The `Dockerfile` builds from source onto
    `gcr.io/distroless/static-debian12:nonroot`: 17.7 MB, CA certificates
    only, no shell. Its entrypoint is `san_vpn relay run`.
  - The release workflow pushes it as `ghcr.io/wargasipil/san_vpn:<tag>`,
    and `:latest` for tags without `-`, for running the relay in Docker.
  - **Until v0.4.2**, Cloud Run ran that image: Cloud Run pulls only from
    Artifact Registry and Docker Hub, so `setup cloudrun` made an Artifact
    Registry *remote* repository `ghcr` with upstream `https://ghcr.io`. That
    needs the GHCR package to be public, and it never was (anonymous token
    requests got 401 through v0.4.2). On 2026-10-09 the user's deploy failed
    with `UNAUTHENTICATED: Unauthorized error returned by the external
    repository`, and the user asked to deploy from source instead. A `ghcr`
    remote repository made by an older run is left unused.
  - **Since v0.4.3**, `cloudrun deploy` builds the same Dockerfile with Cloud
    Build in the user's project, into a standard Artifact Registry repository
    `san-vpn`, as
    `<region>-docker.pkg.dev/<project>/san-vpn/san_vpn:<tag>`. Cloud Run's
    service agent reads its own project's repository with no setup, and
    nothing outside the project has to be public.
  - The source: a release binary fetches its own tag's source from GitHub's
    archive (`<repo>/archive/refs/tags/<tag>.tar.gz`, a web link, not the
    API). GitHub makes that archive on request, so no `SHA256SUMS` lists it;
    it rests on HTTPS to GitHub, as `go install` would. Entries that climb
    out of the folder are refused. `--source <dir>`, or `go run` started in
    a checkout, builds that tree instead, tagged `dev-<UTC time>`.
  - A release's image already in the repository is not built again, so
    rerunning `deploy` changes nothing. A `dev-` tree is built every time,
    since it may hold anything. `--image` runs any image as it is.
  - Not chosen: Cloud Run's "deploy without build" (`gcloud beta run deploy
    --source --no-build --base-image osonly24`), which would upload the
    release's Linux binary with no Cloud Build. It was a Preview feature on
    2026-10-09.
- **`cloudrun setup`** drives `gcloud`, the way `setup init` drives
  `devtunnel`. Each step looks before it acts:
  1. the Run, Artifact Registry and Cloud Build APIs
  2. a private bucket (`--uniform-bucket-level-access`,
     `--public-access-prevention`) in the service's region
  3. the service account `san-vpn-relay`, with `roles/storage.objectUser` on
     that bucket only. A new account is retried while IAM catches up.
  4. the relay key, written with the user's own sign-in, and `cloud_run`
     (project, region, service) recorded in `relay.json`
  5. the image repository `san-vpn`
  6. Cloud Build's account (`gcloud builds get-default-service-account`) may
     build. Projects whose first build ran after mid-2024 build as the Compute
     Engine default account, which may hold no role; older ones use the
     legacy Cloud Build account. Unless the account is already a builder,
     editor or owner, it gets `roles/cloudbuild.builds.builder` on the
     project, with `--condition None` (a project policy with conditions
     refuses an unconditional binding otherwise when nobody can be asked).
- **`cloudrun deploy`** takes project, region and service from what setup
  recorded (flags override), and refuses before setup has run:
  1. the image: `gcloud builds submit <source> --config <recipe>
     --substitutions _IMAGE=...,_VERSION=<tag> --suppress-logs`. The recipe
     is the Dockerfile's `docker build` with `--build-arg VERSION`, so the
     relay reports its tag, and logs go to Cloud Logging only, which every
     build account may write. `--suppress-logs` waits for the build without
     streaming a log gcloud may not be allowed to read; a failure says how to
     find it.
  2. `gcloud run deploy` with `--max-instances 1 --concurrency 1000
     --timeout 3600 --no-cpu-throttling --cpu 1 --memory 512Mi
     --execution-environment gen2 --allow-unauthenticated`. Environment
     variables go in an `--env-vars-file`, so commas survive Windows' command
     line.
  3. the service URL as the relay's URL, with `cloud_run` recorded in
     `relay.json`
  4. GET `/` and the WebSocket challenge

  A rerun redeploys only when the image, environment, max-scale, timeout or
  service account differ from `gcloud run services describe --format json`,
  or when the service is not Ready: a deploy whose image could not be pulled
  can leave a service that matches but never came up. It puts back public
  access if it was removed. After `san_vpn update`, `deploy` moves the relay
  to the new release. `--dry-run` runs only the read-only calls, fetches no
  source and prints the rest.
- **gcloud on Windows** is `gcloud.cmd`, a batch file, which Windows starts
  through `cmd.exe /c`. cmd drops the first and last quote of a line that
  holds more than two.
  - On 2026-10-09, with gcloud under `C:\Users\ASUS TUF\` and a quoted
    argument (`--display-name "san_vpn relay"`), the path lost its quotes.
    The run stopped at the service account with `'C:\Users\ASUS' is not
    recognized`. The earlier calls had no argument with a space.
  - Since v0.4.2 a batch file runs as `cmd.exe /d /s /c "..."`, each argument
    quoted where cmd or the program would split it. `/s` strips only the outer
    pair.
  - Arguments with `%`, `!` or `"` are refused: cmd expands `%VAR%` even in
    quotes, and gcloud.cmd turns on delayed expansion, which eats `!`.
  - Tested with a fake `gcloud.cmd` in an `ASUS TUF` folder that hands `%*`
    on like the real one, and with the real one answering a `--format`
    containing a space.
- **Billing.**
  - Below 1 vCPU, Cloud Run forces concurrency 1, which rules out a relay.
  - Members keep the instance busy, so instance-based billing
    (`--no-cpu-throttling`) is the cheaper kind: $0.000018 per vCPU-second,
    with 240,000 vCPU-seconds and 450,000 GiB-seconds free a month at Tier 1
    (us-central1) prices. That is about US$45 a month, more in Jakarta, plus
    internet egress for every byte relayed.
  - Prices are from cloud.google.com/run/pricing, read 2026-10-07.

**Verified (2026-10-07):**

- Unit tests:
  - the GCS client against `gcstest`: object names with `/`, create-only,
    stale generations, retry after a 401
  - both stores: lifecycle, 60 concurrent increments with none lost
  - `setup cloudrun` against a fake gcloud that models the project, plus a
    real relay at the reported URL:
    - from scratch
    - a rerun changes nothing
    - removed public access is put back alone
    - IAM's delay after creating the account
    - the dry run touches neither the project nor the bucket
    - a local build needs `--image`
- Relay: the welcome's renew time, the cut at the limit, `resume` honoured
  only for the current id.
- End to end, in process (`internal/node/cloudrun_test.go`):
  - a 1.5 s limit, two members, one TCP stream sending a line every 50 ms
    for 6 s
  - at least 4 renewals; no line lost; neither member seen to disconnect;
    WireGuard's last handshake unchanged
  - two mutations each make the test fail:
    - without renewal, the stream stalls
    - without `resume`, WireGuard handshakes again
  - a relay on GCS with an admin on a separate client: invite, join, mesh,
    removal
- `go test -race -count=2 ./...` is clean (Linux, Docker).
- Docker (`fsouza/fake-gcs-server` as the bucket, so a third-party server and
  not only our own fake):
  - the relay image with `PORT=8080` and a 20 s limit
  - two Alpine members with real TUN
  - admin commands from the Windows host
  - 250 pings over 50 s across 4 renewals with 0 % loss
  - `setup check` from a member was all ok
  - `relay remove` from the host cut the member off within 8 s
- **Not verified:** a real deploy, real Cloud Storage and IAM, the Artifact
  Registry remote of GHCR, and Cloud Run's own cut at 60 minutes.

**Verified (2026-10-09), `cloudrun setup` and `deploy`:**

- Against a fake gcloud that also models Cloud Build (the default build
  account, the project policy, image lookups, `builds submit` checking for a
  source folder, the recipe and `--suppress-logs`), plus the real relay:
  - setup from scratch, without building or deploying; its rerun changes
    nothing
  - deploy builds the release once and deploys it; a rerun builds nothing,
    fetches no source and changes nothing
  - `dev-` trees build on every deploy; `--image` builds nothing
  - deploy follows the region and service setup recorded
  - a builder that is already an editor is left alone
  - a service that is not Ready is deployed again
  - both dry runs change nothing and fetch no source
  - two mutations each make a test fail: ignoring readiness, and dropping
    `--condition None`
- The real v0.4.2 archive, fetched from GitHub by `update.Client.Source`,
  built with the recipe's step (`docker build --build-arg VERSION=v0.4.2`)
  into a 17.8 MB image that printed `san_vpn version v0.4.2`.
- **Not verified:** Cloud Build itself and the deploy, in a real project.
  The user runs them.

## Profiles.

`san_vpn profile` switches a member between the networks it joined, such as
the dev tunnel relay's and the Cloud Run relay's (asked for 2026-10-08).

- **A profile is a whole membership.** The two relays are separate networks:
  `setup init` and `cloudrun setup` each create a relay key, and each relay
  keeps its own members and gives out its own addresses. So a profile holds
  this machine's key, address and relay for one network, joined with that
  relay's invite. Making them one network with two fronts was not done:
  - both relays would have to share one `relay.json`, so the dev tunnel relay
    would depend on Cloud Storage too
  - members on different relays still could not reach each other, because
    members meet in one process
- **Layout.** The default profile is the node directory itself. A `node.json`
  from before profiles is the profile `default`, and nothing moves. Other
  profiles are `profiles/<name>/node.json` and `status.json`, and
  `profile.json` names the current one. Names follow member names
  (`[a-z0-9-]`, at most 32), which also keeps them safe as folder names.
- **Which profile.**
  - `--profile` (`SAN_VPN_PROFILE`) on `join`, `up`, `status` and
    `setup check`.
  - Otherwise `join` and `up` use the current profile. `status` and
    `setup check` show the profile `up` is running, else the current one.
  - A machine's first membership becomes current whatever it is called, so
    `join --profile cloudrun` and then `up` work on a fresh machine.
  - Messages name the profile only once there is one besides `default`, so a
    machine in one network sees the same output as before.
- **One at a time.** `up` refuses to start while any profile's `status.json`
  is fresh. Both networks default to `10.77.0.0/24` and to the same interface
  name. On Windows each profile still gets its own adapter GUID, because the
  GUID comes from the key, so Windows keeps a network profile per network.
  `up` replaces the `san_vpn` firewall rule on every start, so the rule
  follows the profile.
- **`profile use`** changes only what the next `up` starts. A running `up`
  keeps its profile, and `profile use` says so.
- **`profile rename`** moves `node.json` (for example `default` to `tunnel`),
  and the current choice follows it. It refuses while `up` runs on that
  profile, because `up` keeps writing `status.json` where it started.
- **Not a global flag.** `--profile` belongs to the member commands only. The
  relay commands pick their relay with `--state`. A global flag that they
  ignored could put an invite in the wrong network.
- **Verified (2026-10-08):**
  - unit tests: the layout and listing, names, renaming in and out of the
    node directory
  - CLI tests against two relays in process: the first join goes to
    `default`; a second is refused without `--profile`; `join --profile`,
    `list`, `use`, `status` and `rename`; a running profile is shown first,
    cannot be renamed, and `use` says it is still running
  - `setup check` names the profile in its title and its fixes
  - the built Windows binary, unelevated with `--state`, against two real
    `relay run` processes on loopback
  - **not run:** `up` itself across a switch, which needs an administrator
    terminal

## Names.

Members find each other by name: `office.vpn` for the member `office` (asked
for 2026-10-10: "can we add dns server in san_vpn"). Decided with the user the
same day:
- member names only: no records of the admin's own, no forwarding of other
  names
- the domain `vpn` by default, settable per network
- `/etc/hosts` on Linux machines without systemd-resolved

The user first chose `.local`, then `.vpn` once the clash with multicast DNS
came up (below).

- **Each member answers; there is no name server.**
  - The relay is not on the overlay (decision 3). To serve DNS there it would
    have to become a WireGuard peer, and then it could read traffic.
  - Every member already gets the member list, so `up` answers from it, and
    keeps answering while the relay reconnects.
  - Member names were already hostname-shaped (`[a-z0-9-]`, at most 32), so
    nobody had to be renamed.
- **The address** is `wire.DNSAddr`, the network's last host address:
  `10.77.0.254` in `10.77.0.0/24`.
  - The relay's `allocate` never gives it out.
  - The route to the overlay already carries it into the tunnel on both
    systems, so no route or address is added.
  - A network smaller than /29 gets none. A /30 has two hosts and would lose
    one.
  - A relay from before names may already have given it to a member. That
    takes 253 members in a /24, or a small network. Then the table has no
    server, that member's traffic to it passes through, and `up` logs that
    names are off.
- **Caught in the tunnel.** `names.Intercept` wraps the `tun.Device`
  WireGuard reads from.
  - An IPv4 UDP packet to the address on port 53 is answered on the spot. The
    reply, with both checksums, is written back into the tunnel.
  - The query's slot in WireGuard's batch gets size 0, which WireGuard skips.
    Swapping buffers would not do: each slot belongs to one of WireGuard's
    pooled elements.
  - Nothing listens on port 53, so there is no clash with Pi-hole, dnsmasq or
    a Windows DNS server. The same code runs on gVisor's netstack in the
    tests.
  - Before the first member list, queries get SERVFAIL, not NXDOMAIN, so no
    resolver caches "no such name".
  - Fragments are dropped. TCP to port 53 is not caught: an answer is one
    record, so it is never truncated and no resolver retries over TCP.
- **Answers** (`names.Answer`, with `golang.org/x/net/dns/dnsmessage`):
  - a member's name: its A record, TTL 30s, because a removed member's
    address goes to the next one to join
  - AAAA, HTTPS and other types for a member, and the domain itself: an empty
    NOERROR, so browsers do not wait for a timeout
  - an unknown name in the domain: NXDOMAIN, without an SOA, so it is not
    cached and a member who joins later resolves at once
  - a name outside the domain: REFUSED
  - an EDNS query gets EDNS back. systemd-resolved otherwise drops to a
    degraded mode and says so in its log.
- **The domain belongs to the relay.**
  - It is `domain` in `relay.json`, set with `relay init --domain`, and sent
    in every netmap (`Netmap.Domain`).
  - Old relays send none, which nodes read as `vpn`; old nodes ignore it.
  - A change reaches members on the relay's next reload, without a restart.
- **The operating system** (`osnet.Names`). `up` sets it after each member
  list, and only when what it would set has changed:
  - Windows: an NRPT rule sends `.vpn` to the address. It is added with
    PowerShell's DnsClient cmdlets (`Add-DnsClientNrptRule`, comment
    `san_vpn`), followed by `Clear-DnsClientCache`. It is removed on exit, and
    a crashed run's rule is replaced on the next start. A DNS server on the
    adapter would not do: Windows may ask any adapter's servers and take the
    first answer.
  - Linux with systemd-resolved (resolvectl installed, and resolv.conf
    pointing at 127.0.0.53): `resolvectl dns`, `domain ~vpn` and
    `default-route false` on the link. The setting goes away with the
    interface.
  - Other Linux: a fenced block in `/etc/hosts`, removed on exit.
    - It is written in place, because Docker bind-mounts the file. Writing in
      place also keeps its owner and SELinux label.
    - Only a whole block is replaced. A begin line without its end line is
      dropped alone, never the entries after it.
- **Why not `.local`.** RFC 6762 gives `.local` to multicast DNS.
  - Raspberry Pi OS and Ubuntu desktop have
    `hosts: files mdns4_minimal [NOTFOUND=return] dns`, so a `.local` name
    never reaches a DNS server.
  - On Windows, the rule would send `printer.local` and other LAN names to
    san_vpn.
  - `relay init --domain local` is still accepted, with a note.
- **Commands.**
  - `up --no-dns` turns names off.
  - `status` shows the names.
  - `setup check` resolves this machine's own name through the system
    resolver, which checks the whole path. Go's resolver gives a hosts entry
    as `::ffff:10.77.0.2`, so the check unmaps addresses first.
- **Verified (2026-10-10):**
  - Unit tests:
    - the answers for each case, EDNS, and junk
    - the interceptor on a fake TUN: only the query leaves the batch, the
      reply's checksums are valid, a held address and IPv6 pass through, and
      fragments are dropped
    - the hosts block: added, replaced, removed, CRLF, and a broken block
    - `DNSAddr` and `ValidDomain`
    - allocation skips the address in a /29
    - `relay init --domain`, `relay list`, `status`, and the `setup check`
      item, a mapped address included
  - End to end on netstack (`TestNames`): lookups from inside each member, a
    member joining and then removed, and a domain change. Every end-to-end
    test now runs with names on. Clean under `-race`, and stable over
    `-count=10`.
  - Each of three mutations failed a test: no UDP checksum, leaving the query
    in the batch, and catching a held address. Without the interceptor,
    `TestNames` times out.
  - Every package's tests also passed as Linux binaries in Docker (alpine),
    the hosts file test included.
  - Real Linux kernel in Docker (alpine, `NET_ADMIN`, `/dev/net/tun`), with a
    relay and two members:
    - `/etc/hosts` got the block
    - `ping office.vpn` worked
    - `nslookup office.vpn 10.77.0.254` was answered through the real TUN
    - `nobody.vpn` got NXDOMAIN
    - `status` showed the names
    - `setup check` passed. It first failed on the mapped address, which was
      then fixed.
    - Ctrl+C removed the block
  - Windows: the cmdlets and their parameters exist, and the read side of the
    cleanup runs unelevated.
  - **Not run:**
    - adding the NRPT rule, which needs an administrator terminal
    - systemd-resolved, which needs systemd in a container

## Open question: direct paths.

Every packet passes the relay, by choice (General 5). The cost:
- Latency is home → relay → office.
- The relay's bandwidth is the network's bandwidth. Through a dev tunnel that
  is a developer service with usage limits, reported as about 5 GB a month.

The bind already hides the transport behind a key, so a direct UDP path could
be added later without touching WireGuard or the relay protocol. That would
need STUN-style discovery of each member's public UDP mapping, exchanging those
mappings in the netmap, hole punching, and the bind choosing per peer between
UDP and the relay. Some carrier-grade NATs map ports in a way that defeats hole
punching, so the relay would stay as the fallback.

## Open question: boot service.

`up` runs in a terminal (General 12). A Windows service
(`golang.org/x/sys/windows/svc`) and a systemd unit would run the same
`node.Run`. The member directory is already a system directory for that
reason.

## Trust model.

- **The relay cannot read traffic** (decision 3), cannot impersonate a member
  to another member (WireGuard keys), and cannot be impersonated to a member
  (decision 4).
- **The relay is trusted to say who the members are.** A compromised relay
  could add a key of its own to everyone's netmap and then talk to members as
  a new peer. Members would see it in `status`. Signing member lists with an
  offline admin key would remove this trust; it is not done.
- **Names come from the relay too.** A relay trusted to say who the members
  are can equally give `office.vpn` to a key of its own. Answers come from
  the member's own process, so nothing between the member and the relay can
  change them.
- **The front (the dev tunnel) is not trusted** with anything but availability.
- **On Cloud Run, the bucket is the relay.** It holds the relay's private key
  and the member list, so whoever can write it can do what a compromised relay
  can. It is private and admits only the service account and the project's
  owners. Google, as the host, is trusted the way the relay machine is.

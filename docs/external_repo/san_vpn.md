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
  cmd/san_vpn/update.go       update: the latest or a named release, in place of this binary
  cmd/san_vpn/main_test.go    the admin flow through the real command tree
  internal/wire/              the protocol both sides speak
               keys.go        X25519 keys (one type for both halves), base64/hex forms
               wire.go        frames, control messages, join MAC, handshake proofs
               ws.go          keepalive, control message read/write
  internal/invite/            the sanvpn1_ invite blob
  internal/state/             JSON files: default dirs, lock, atomic save, Windows ACL
  internal/relay/  state.go   members, invites, address allocation
                   server.go  join endpoint, WebSocket sessions, forwarding, reload
  internal/node/   config.go  node.json, Join
                   bind.go    WireGuard's conn.Bind, carried by the relay
                   uapi.go    netmap -> WireGuard peer config diff, stats parsing
                   node.go    the device, relay session, reconnects, status
                   e2e_test.go relay + members + WireGuard on gVisor netstack
  internal/devtunnel/         the devtunnel CLI: find, install, JSON calls, supervised `host`
  internal/setup/  setup.go   Init: sign in, relay, tunnel, anonymous access, port, URL
                   check.go   Check: relay + tunnel + member, each failure with its fix
  internal/update/            release links, SHA256SUMS, replacing a running binary
  internal/osnet/             the only admin-only code: TUN, address, firewall
                   osnet_windows.go  Wintun (embedded dll), winipcfg, netsh rule
                   osnet_linux.go    TUN, netlink address
                   wintun/           fetched by the build scripts, git-ignored
  build.ps1 / build.sh        fetch Wintun (checksum pinned), vet, test, build both binaries
  .github/workflows/release.yml  on a v* tag: build.sh, then a GitHub release
```

## Releases.

Pushing a version tag (`git tag -a v0.2.0 -m "san_vpn v0.2.0"`, then
`git push origin v0.2.0`) runs `.github/workflows/release.yml` on GitHub
Actions. It runs `build.sh` with `SAN_VPN_VERSION` set to the tag, so
`--version` prints the tag; local builds keep the time stamp. The release gets
`san_vpn-windows-amd64.exe`, `san_vpn-linux-amd64`, `wintun-LICENSE.txt` and
`SHA256SUMS`, with the README's install notes and GitHub's generated
changelog. Asset names carry no version, so
`releases/latest/download/san_vpn-linux-amd64` always fetches the newest. A tag
with a `-` (`v0.2.0-rc1`) becomes a prerelease, which "latest" skips.

- **v0.1.0** (2026-10-07): the first release. CI built it in 1m32s. The
  downloaded files matched `SHA256SUMS`, and the downloaded exe printed
  `san_vpn version v0.1.0`.
- **v0.2.0** (2026-10-07): `san_vpn update` (next section) and urfave/cli
  v3.14.0. CI built it in 1m5s. Windows and Linux builds stamped `v0.0.9`
  updated themselves to it, and the results matched its `SHA256SUMS`.

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
  - `user show` answers `{status: "Logged in" | "Not logged in", provider, username}`
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
- **Finding the CLI right after installing it.** Installers update PATH for
  new terminals only. winget (1.29) makes no shim in `WinGet\Links`: it adds
  its package folder to the user PATH in the registry. So `Find` looks in this
  order:
  1. our PATH
  2. the user and machine PATH read fresh from the registry
  3. winget's `Links` and `Packages\Microsoft.devtunnel_*` folders
  4. `~/bin` and `/usr/local/bin` on Linux
- **Hosting.**
  - `relay run` starts `devtunnel host <id> --nologo` once it is listening
    on `127.0.0.1:<port>`. It logs the host's output and restarts it with
    backoff (5s doubling to 1 min) when it exits.
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
  member's address is reused. Peers trust keys, not addresses.
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
    output is logged, a restart happens, and cancel stops it
  - real CLI 1.0.2094 on this machine: `setup check` found it through the
    registry PATH, read "Not logged in", and reported the fixes
  - not yet run: a full `setup init` with a real GitHub sign-in. It needs the
    user's account.
- **Not yet verified on a real Windows machine:** `up` with Wintun, the
  address, the firewall rule. It needs an elevated terminal, and the session
  that built this had none. Everything above the OS layer is the same code the
  Linux run exercised.

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

## Open question: names.

Member names are already hostname-shaped (`[a-z0-9-]`, at most 32 characters)
so they can become DNS names, such as `office.vpn`, without renaming anyone.
That would need either a tiny resolver on the overlay or hosts-file entries.

## Trust model.

- **The relay cannot read traffic** (decision 3), cannot impersonate a member
  to another member (WireGuard keys), and cannot be impersonated to a member
  (decision 4).
- **The relay is trusted to say who the members are.** A compromised relay
  could add a key of its own to everyone's netmap and then talk to members as
  a new peer. Members would see it in `status`. Signing member lists with an
  offline admin key would remove this trust; it is not done.
- **The front (the dev tunnel) is not trusted** with anything but availability.

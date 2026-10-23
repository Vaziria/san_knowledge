# san_tower

The tower: one place to watch the whole project. `san_tower serve` runs, on
http://127.0.0.1:7477:
- TowerService (Connect RPC), which says the tower is up;
- the San Message broker ([golang/packages/san_message](../../packages/san_message/README.md)),
  embedded, with its MessageService, so apps publish and subscribe at the
  tower's address;
- the tower's UI ([typescripts/san_tower_frontend](../../../typescripts/san_tower_frontend),
  built into the binary). Home shows the tower's status; Message Broker
  manages the broker's topics and subscribers.

What else the tower watches is still to be decided.

Module `github.com/wargasipil/san_tower`, Go 1.26. It uses san_message from
this repository (`replace` in go.mod).

```powershell
pwsh build.ps1                          # UI build + vet + test + bin/san_tower.exe and bin/san_tower
bin\san_tower.exe serve                 # then open http://127.0.0.1:7477
go run .\cmd\san_tower dev              # tower + the UI's Vite dev server; open Vite's address (http://localhost:5173)
bin\san_tower.exe status                # asks the running tower (--server, $env:SAN_TOWER_SERVER)
# after editing a proto: Go and TS for TowerService, then TS for san_message's MessageService
buf generate; buf generate --template buf.gen.san_message.yaml
```

The broker's CLI works against the tower too:
`go run ../../packages/san_message/cmd/san_message --server http://127.0.0.1:7477 topic list`.

## Data

`--data` (default `data`, relative to where the tower runs; `$env:SAN_TOWER_DATA`)
is the tower's folder, git-ignored. The broker keeps its topic and subscriber
metadata and event logs in `<data>/san_message`. San Message does not lock its
folder, so run one tower per data folder. Two towers on the same `--listen`
cannot both start, since the second fails to listen before it opens the
broker.

## Files

- `protos/san/tower/v1/tower.proto`: TowerService. `buf.gen.yaml` makes its
  Go code (`gen/`) and the UI's TS client. `buf.gen.san_message.yaml` makes
  the UI's TS client for san_message's proto; the Go side uses san_message's
  own generated code.
- `cmd/san_tower/main.go`: the CLI (`serve`, `dev`, `status`).
  - `serve` and `dev` open the broker, then serve the tower with
    `san_message.ConfigureServer` (h2c, for the broker's Subscribe stream,
    and pings). On stop the broker closes first, which ends the open
    Subscribe streams.
  - `dev` serves the tower on `--listen` and runs Vite (`node
    node_modules/vite/bin/vite.js`, not npm, so stopping it leaves nothing
    behind) in the frontend's folder, found from the current folder up
    (`--frontend` to give it). `npm ci` runs first if `node_modules` is
    missing. Vite's proxy goes to this tower through `SAN_TOWER_SERVER`.
    When either one stops, so does the other; Ctrl+C stops both. A UI
    change reloads by itself; a Go change needs a restart.
- `internal/server`: the HTTP server. It serves TowerService, MessageService
  and the UI behind `Guard`, which only accepts a loopback Host and loopback
  pages.
- `internal/web`: the embedded UI. `dist/` is the Vite build (git-ignored but
  for `.gitkeep`). Paths without a file extension get `index.html`, so the
  UI's routes can be reloaded and linked to. Before the first UI build the
  page answers 503.

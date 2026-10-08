# smbd — a small SMB2/3 file server in Go

Serves a local directory over SMB2/3 so it can be mounted from Windows, macOS,
or Linux. Written from the MS-SMB2 / MS-NLMP specs with only `golang.org/x/crypto`
(for the MD4 the NTLM hash needs) as a non-test dependency.

It exists to show how SMB works end to end; it is **not** a Samba replacement.

## Run

```sh
cd golang/examples/smb
go run ./cmd/smbd                       # exports ./shared_data as \\host\shared on :4455, guest access
go run ./cmd/smbd -dir /path -share pub # export another directory under another name
go run ./cmd/smbd -user alice:secret    # require NTLMv2 login instead of guest
go run ./cmd/smbd -ro                   # read-only
```

Port 4455 (not the usual 445) is the default because Windows' own SMB service
holds 445 on every address, and binding a privileged port needs elevation.

## Mount it

```sh
# Linux
sudo mount -t cifs //127.0.0.1/shared /mnt -o port=4455,guest,vers=3.0
# or browse without mounting:
smbclient //127.0.0.1/shared -p 4455 -U guest -N

# macOS (Finder: Go > Connect to Server)
open 'smb://guest@127.0.0.1:4455/shared'

# Windows — map a drive (the @port form avoids 445):
net use Z: \\127.0.0.1@4455\shared
```

With `-user alice:secret`, log in as `alice` / `secret` instead of `guest`.

## What it supports

- SMB dialects 2.0.2 through 3.0.2, negotiated with the client.
- NTLMv2 authentication, or guest/anonymous access.
- SMB2 signing (HMAC-SHA256 on 2.x, AES-128-CMAC on 3.x).
- Read, write, create, open, list (with wildcards), rename, delete, mkdir,
  truncate, set timestamps, query file/volume info.
- Path-traversal jailing: a client cannot escape the exported directory.
- Multiple shares and read-only shares (via the library API).

## What it does not

- No SMB 3.1.1 (so no pre-auth integrity), no encryption, oplocks, leases,
  DFS, or NTFS alternate data streams.
- No share enumeration (`IPC$` / SRVSVC): connect to `\\host\share` directly
  rather than browsing `\\host`.
- No SMB1. Modern clients negotiate SMB2 directly; Windows ships with SMB1
  disabled.

## Library

`cmd/smbd` is a thin wrapper over the `server` package:

```go
srv, _ := server.New(server.Config{
    Addr:       "0.0.0.0:4455",
    AllowGuest: true,
    Shares:     []server.ShareConfig{{Name: "shared", Path: "./shared_data"}},
})
srv.Serve(ctx) // blocks until ctx is cancelled
```

## Tests

`server/integration_test.go` starts the server on a random port and drives it
with a real SMB2 client (`github.com/hirochachacha/go-smb2`): guest and NTLMv2
login, read/write/list, mkdir/rename/delete, and a path-traversal check.

```sh
go test ./...
```

It has also been checked by hand against Samba's `smbclient` and `mount -t cifs`.

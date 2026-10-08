// Command smbd serves a local directory over SMB2/3.
//
// By default it exports ./shared_data as the share "shared" on port 4455 with
// guest access, so no elevation is needed (port 445 is reserved by Windows'
// own SMB service). Mount it, for example:
//
//	Windows:  net use Z: \\127.0.0.1@4455\shared        (or map in Explorer)
//	Linux:    sudo mount -t cifs //127.0.0.1/shared /mnt -o port=4455,guest,vers=3.0
//	smbclient: smbclient //127.0.0.1/shared -p 4455 -U guest -N
//	macOS:    open smb://guest@127.0.0.1:4455/shared
//
// With -user alice:secret, clients authenticate with NTLMv2 instead of guest.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/wargasipil/examples/smb/server"
)

func main() {
	addr := flag.String("addr", "0.0.0.0:4455", "listen address")
	dir := flag.String("dir", "./shared_data", "directory to export")
	name := flag.String("share", "shared", "share name")
	readOnly := flag.Bool("ro", false, "export read-only")
	userFlag := flag.String("user", "", "user:password for NTLMv2 (repeatable); guest if empty")
	var users multiFlag
	flag.Var(&users, "u", "additional user:password (repeatable)")
	flag.Parse()

	abs, err := filepath.Abs(*dir)
	if err != nil {
		fatal("resolve dir", err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		fatal("export dir", fmt.Errorf("%s is not a directory", abs))
	}

	creds := map[string]string{}
	for _, u := range append(users, *userFlag) {
		if u == "" {
			continue
		}
		name, pass, ok := strings.Cut(u, ":")
		if !ok {
			fatal("user flag", fmt.Errorf("want user:password, got %q", u))
		}
		creds[strings.ToLower(name)] = pass
	}

	cfg := server.Config{
		Addr:       *addr,
		ServerName: "GOSMB",
		AllowGuest: len(creds) == 0,
		Users:      creds,
		Shares: []server.ShareConfig{
			{Name: *name, Path: abs, ReadOnly: *readOnly},
		},
		Logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	srv, err := server.New(cfg)
	if err != nil {
		fatal("configure server", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mode := "guest"
	if len(creds) > 0 {
		mode = "ntlmv2"
	}
	slog.Info("starting", "dir", abs, "share", *name, "auth", mode, "readonly", *readOnly)
	if err := srv.Serve(ctx); err != nil {
		fatal("serve", err)
	}
}

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func fatal(what string, err error) {
	slog.Error(what, "err", err)
	os.Exit(1)
}

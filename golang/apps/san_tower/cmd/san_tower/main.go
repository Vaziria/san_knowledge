// Command san_tower watches the project from one place. `san_tower serve`
// runs the tower on 127.0.0.1:7477: TowerService, the San Message broker
// (golang/packages/san_message, data in --data) with its MessageService, and
// the UI (typescripts/san_tower_frontend, built into the binary), where the
// Message Broker page manages the broker's topics and subscribers.
// `san_tower dev` runs the tower with the UI's Vite dev server instead, for
// hot reload.
//
//	san_tower serve
//	san_tower dev
//	san_tower status
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"
	"github.com/wargasipil/san_message"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
	"github.com/wargasipil/san_tower/gen/san/tower/v1/towerv1connect"
	"github.com/wargasipil/san_tower/internal/server"
)

// version is stamped at build time.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root(os.Stdout).Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "san_tower: %v\n", err)
		os.Exit(1)
	}
}

func root(out io.Writer) *cli.Command {
	return &cli.Command{
		Name:    "san_tower",
		Usage:   "watch the project from one place",
		Version: version,
		Writer:  out,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "server",
				Usage:   "URL of the running `san_tower serve`",
				Value:   server.DefaultURL,
				Sources: cli.EnvVars("SAN_TOWER_SERVER"),
			},
		},
		Commands: []*cli.Command{serveCommand(), devCommand(), statusCommand()},
	}
}

var (
	listenFlag   = &cli.StringFlag{Name: "listen", Value: server.DefaultAddr, Usage: "address to listen on (loopback only)"}
	logLevelFlag = &cli.StringFlag{Name: "log-level", Value: "info", Usage: "debug, info, warn or error"}
	// San Message does not lock its folder, so one tower per data folder.
	// Two towers on the same --listen cannot both start: the second fails to
	// listen before it opens the broker.
	dataFlag = &cli.StringFlag{
		Name:    "data",
		Value:   "data",
		Usage:   "the tower's data folder; the broker keeps its topics and logs in <data>/san_message",
		Sources: cli.EnvVars("SAN_TOWER_DATA"),
	}
)

func logger(cmd *cli.Command) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
		return nil, fmt.Errorf("bad --log-level %q", cmd.String("log-level"))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})), nil
}

// listen opens the tower's address, which must be a loopback one.
func listen(addr string) (net.Listener, error) {
	if !server.IsLoopback(addr) {
		return nil, fmt.Errorf("--listen %s: san_tower only listens on a loopback address", addr)
	}
	return net.Listen("tcp", addr)
}

// serveTower opens the broker in dataDir and serves the tower on ln until ctx
// ends.
func serveTower(ctx context.Context, log *slog.Logger, ln net.Listener, dataDir string) error {
	broker, err := san_message.Open(san_message.Options{Dir: filepath.Join(dataDir, "san_message"), Log: log})
	if err != nil {
		return fmt.Errorf("san_message: %w", err)
	}
	defer broker.Close()

	srv := &http.Server{Handler: server.New(version, broker).Handler(), ReadHeaderTimeout: 10 * time.Second}
	// HTTP/2 without TLS for the broker's Subscribe stream, and pings that
	// drop dead connections.
	san_message.ConfigureServer(srv)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("san_tower listening", "url", "http://"+ln.Addr().String(), "data", dataDir, "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	broker.Close() // ends the open Subscribe streams, so Shutdown does not wait on them
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

// ---------------------------------------------------------------- serve ---

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:   "serve",
		Usage:  "run the tower: TowerService, the San Message broker and the UI",
		Flags:  []cli.Flag{listenFlag, dataFlag, logLevelFlag},
		Action: runServe,
	}
}

func runServe(ctx context.Context, cmd *cli.Command) error {
	log, err := logger(cmd)
	if err != nil {
		return err
	}
	ln, err := listen(cmd.String("listen"))
	if err != nil {
		return err
	}
	return serveTower(ctx, log, ln, cmd.String("data"))
}

// ------------------------------------------------------------------ dev ---

func devCommand() *cli.Command {
	return &cli.Command{
		Name:  "dev",
		Usage: "run the tower and the UI's Vite dev server (hot reload) together",
		Flags: []cli.Flag{
			listenFlag, dataFlag, logLevelFlag,
			&cli.StringFlag{Name: "frontend", Usage: "the frontend's folder (default: typescripts/san_tower_frontend, looked for from the current folder up)"},
		},
		Action: runDev,
	}
}

func runDev(ctx context.Context, cmd *cli.Command) error {
	log, err := logger(cmd)
	if err != nil {
		return err
	}
	dir := cmd.String("frontend")
	if dir == "" {
		if dir, err = findFrontend(); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "node_modules")); errors.Is(err, fs.ErrNotExist) {
		log.Info("installing the frontend's packages", "dir", dir)
		npm := exec.CommandContext(ctx, "npm", "ci", "--no-audit", "--no-fund")
		npm.Dir, npm.Stdout, npm.Stderr = dir, os.Stderr, os.Stderr
		if err := npm.Run(); err != nil {
			return fmt.Errorf("npm ci in %s: %w", dir, err)
		}
	}
	// Listen first, so a busy port fails before Vite starts.
	ln, err := listen(cmd.String("listen"))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log.Info("open the UI at Vite's address below; a Go change needs a restart, a UI change reloads by itself")
	errc := make(chan error, 2)
	go func() { errc <- serveTower(ctx, log, ln, cmd.String("data")) }()
	go func() { errc <- runVite(ctx, dir, "http://"+ln.Addr().String()) }()
	// When either stops, stop the other.
	err = <-errc
	cancel()
	return errors.Join(err, <-errc)
}

// runVite runs the frontend's Vite dev server until ctx ends, with its proxy
// pointed at the tower (SAN_TOWER_SERVER, read in vite.config.ts). Vite runs
// under node directly, not through npm, so stopping it stops the server
// itself instead of leaving it running behind npm or cmd.exe.
func runVite(ctx context.Context, dir, towerURL string) error {
	c := exec.CommandContext(ctx, "node", filepath.Join("node_modules", "vite", "bin", "vite.js"))
	c.Dir = dir
	c.Env = append(os.Environ(), "SAN_TOWER_SERVER="+towerURL)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	c.WaitDelay = 5 * time.Second
	err := c.Run()
	if ctx.Err() != nil {
		return nil // stopped with the tower, or by Ctrl+C
	}
	if err != nil {
		return fmt.Errorf("vite: %w", err)
	}
	return nil
}

// findFrontend looks for typescripts/san_tower_frontend from the current
// folder up, so dev works from the app's folder (`go run ./cmd/san_tower dev`)
// as well as from the repository root.
func findFrontend() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, "typescripts", "san_tower_frontend")
		if _, err := os.Stat(filepath.Join(p, "package.json")); err == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("typescripts/san_tower_frontend is not in this folder or above it: run from the repository, or pass --frontend")
		}
		dir = parent
	}
}

// --------------------------------------------------------------- status ---

func statusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "ask the running tower whether it is up",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.String("server")
			client := towerv1connect.NewTowerServiceClient(&http.Client{Timeout: 10 * time.Second}, url)
			r, err := client.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
			if connect.CodeOf(err) == connect.CodeUnavailable {
				return fmt.Errorf("no tower at %s: start it with `san_tower serve`", url)
			}
			if err != nil {
				return err
			}
			started := r.Msg.StartedAt.AsTime().Local()
			fmt.Fprintf(cmd.Root().Writer, "version  %s\nhost     %s\nstarted  %s (up %s)\n",
				r.Msg.Version, r.Msg.Hostname, started.Format("2006-01-02 15:04:05"), time.Since(started).Round(time.Second))
			return nil
		},
	}
}

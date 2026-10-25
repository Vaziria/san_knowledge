// Command san_tower watches the project from one place. `san_tower serve`
// runs the tower on 127.0.0.1:7477: TowerService, the San Message broker
// (golang/packages/san_message, data in --data) with its MessageService,
// WorkflowService (workflows and their runs in --data's tower.db, and the
// engine that runs them), and the UI (typescripts/san_tower_frontend, built
// into the binary), where the Message Broker page manages the broker's
// topics and subscribers and the Workflow pages edit and run the workflows.
// The Browser Remote Server page switches on the San Remote server
// (golang/apps/san_remote) inside the tower, on san_remote's own addresses.
// `san_tower dev` runs the tower with the UI's Vite dev server instead, for
// hot reload. `san_tower update` installs the newest release and restarts
// the running tower into it (update.go).
//
//	san_tower serve [--listen 0.0.0.0:7477 --lan]
//	san_tower dev
//	san_tower status
//	san_tower update [--check]
//	san_tower restart
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
	"github.com/wargasipil/san_tower/internal/broker"
	"github.com/wargasipil/san_tower/internal/browserremote"
	"github.com/wargasipil/san_tower/internal/engine"
	"github.com/wargasipil/san_tower/internal/server"
	"github.com/wargasipil/san_tower/internal/store"
	"github.com/wargasipil/san_tower/internal/update"
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
		Commands: []*cli.Command{serveCommand(), devCommand(), statusCommand(), updateCommand(), restartCommand()},
	}
}

var (
	listenFlag   = &cli.StringFlag{Name: "listen", Value: server.DefaultAddr, Usage: "address to listen on (loopback only, unless --lan)"}
	lanFlag      = &cli.BoolFlag{Name: "lan", Usage: "allow a network address in --listen (0.0.0.0:7477): the tower has no login, so anyone on the local network can use it"}
	logLevelFlag = &cli.StringFlag{Name: "log-level", Value: "info", Usage: "debug, info, warn or error"}
	// San Message does not lock its folder, so one tower per data folder.
	// Two towers on the same --listen cannot both start: the second fails to
	// listen before it opens the broker.
	dataFlag = &cli.StringFlag{
		Name:    "data",
		Value:   "data",
		Usage:   "the tower's data folder: its settings (tower.json), its database (tower.db) and, unless the settings name another folder, the broker's in <data>/san_message",
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

// restartEnv marks a tower that a restart started, whose address the one
// before may still be letting go of.
const restartEnv = "SAN_TOWER_RESTARTED"

// listen opens the tower's address: a loopback one, or with lan any (the
// user's choice, 2026-10-26: the tower on the Raspberry Pi, opened from the
// PC). After a restart it tries for 10 s.
func listen(addr string, lan bool) (net.Listener, error) {
	if !lan && !server.IsLoopback(addr) {
		return nil, fmt.Errorf("--listen %s: san_tower only listens on a loopback address, unless --lan (it has no login: anyone who reaches it can use it)", addr)
	}
	ln, err := net.Listen("tcp", addr)
	for deadline := time.Now().Add(10 * time.Second); err != nil && os.Getenv(restartEnv) != "" && time.Now().Before(deadline); {
		time.Sleep(250 * time.Millisecond)
		ln, err = net.Listen("tcp", addr)
	}
	return ln, err
}

// serveTower opens the database and the broker (in the data folder its
// setting names, in dataDir), serves the tower on ln until ctx ends, and
// switches on the active workflows' triggers once it is listening. lan
// answers on the network too; restart, when set, is how TowerService's
// Restart stops it to start again.
func serveTower(ctx context.Context, log *slog.Logger, ln net.Listener, dataDir string, lan bool, restart func()) error {
	st, err := store.Open(dataDir, log)
	if err != nil {
		return err
	}
	defer st.Close()
	b, err := broker.Open(dataDir, log)
	if err != nil {
		return err
	}
	defer b.Close()
	// The San Remote server starts here when it is switched on; one that
	// cannot (san_remote already runs) only says so on its page.
	rm, err := browserremote.Open(dataDir, browserremote.HubAddr, browserremote.APIAddr, log)
	if err != nil {
		return err
	}
	defer rm.Close()

	eng := engine.New(st, b, log)
	// Runs still going are cut short and recorded before the database closes.
	defer eng.Close()
	s := server.New(version, b, st, eng, rm)
	s.LAN, s.OnRestart = lan, restart
	srv := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	// HTTP/2 without TLS for the broker's Subscribe stream, and pings that
	// drop dead connections.
	san_message.ConfigureServer(srv)
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("san_tower listening", "url", "http://"+ln.Addr().String(), "data", dataDir, "broker", brokerWhere(b), "version", version)
	if lan {
		log.Warn("listening on the network (--lan): the tower has no login, so anyone on the local network can open it and run workflows")
	}
	// The triggers stream from the broker inside the tower at its own address,
	// or from the separate server.
	if err := eng.Start("http://" + ln.Addr().String()); err != nil {
		log.Error("switch on the workflows' triggers", "err", err)
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// The triggers' streams stop first, then the broker inside the tower ends
	// the Subscribe streams still open, so Shutdown does not wait on them.
	eng.Close()
	b.Close()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

// ---------------------------------------------------------------- serve ---

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:   "serve",
		Usage:  "run the tower: TowerService, the San Message broker and the UI",
		Flags:  []cli.Flag{listenFlag, lanFlag, dataFlag, logLevelFlag},
		Action: runServe,
	}
}

// runServe serves until the tower is stopped, or until TowerService's
// Restart (after `san_tower update`): then it closes everything and starts
// again from its executable (reexec), the new one if an update replaced it.
func runServe(ctx context.Context, cmd *cli.Command) error {
	log, err := logger(cmd)
	if err != nil {
		return err
	}
	// The path as it starts: an update renames the file it runs from.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	update.Clean(exe)
	ln, err := listen(cmd.String("listen"), cmd.Bool("lan"))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	restarting := false
	err = serveTower(ctx, log, ln, cmd.String("data"), cmd.Bool("lan"), func() {
		restarting = true
		cancel()
	})
	if !restarting {
		return err
	}
	log.Info("restarting", "executable", exe)
	os.Setenv(restartEnv, "1")
	return reexec(exe)
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
	ln, err := listen(cmd.String("listen"), false)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log.Info("open the UI at Vite's address below; a Go change needs a restart, a UI change reloads by itself")
	errc := make(chan error, 2)
	go func() { errc <- serveTower(ctx, log, ln, cmd.String("data"), false, nil) }()
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

// brokerWhere is where the broker in use is, for the log: its data folder
// inside the tower, or a separate server's URI.
func brokerWhere(b *broker.Broker) string {
	if uri := b.Remote(); uri != "" {
		return uri
	}
	return b.State().DataPath
}

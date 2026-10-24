// Command san_remote drives the user's own browser. `san_remote serve` is the
// server the San Remote extension (typescripts/san_remote_extension) attaches
// to; the other commands, and `san_remote mcp` for Claude, send it JavaScript
// to run in a tab and tab commands.
//
//	san_remote serve
//	san_remote tabs
//	san_remote exec 'return document.title'
//	san_remote exec --tab 123 --world main --file script.js
//	san_remote open https://example.com
//	san_remote navigate --tab 123 https://example.com
//	san_remote close --tab 123
//	san_remote mcp
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"

	v1 "github.com/wargasipil/san_remote/gen/san/remote/v1"
	"github.com/wargasipil/san_remote/gen/san/remote/v1/remotev1connect"
	"github.com/wargasipil/san_remote/internal/hub"
	"github.com/wargasipil/san_remote/internal/mcp"
)

// version is stamped at build time.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root(os.Stdin, os.Stdout).Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "san_remote: %v\n", err)
		os.Exit(1)
	}
}

func root(in io.Reader, out io.Writer) *cli.Command {
	return &cli.Command{
		Name:    "san_remote",
		Usage:   "run JavaScript in your own browser through the San Remote extension",
		Version: version,
		Reader:  in,
		Writer:  out,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "server",
				Usage:   "URL of the running `san_remote serve`",
				Value:   hub.DefaultServer,
				Sources: cli.EnvVars("SAN_REMOTE_SERVER"),
			},
			&cli.StringFlag{
				Name:    "browser",
				Usage:   "id or name of the browser to use, when several are attached",
				Sources: cli.EnvVars("SAN_REMOTE_BROWSER"),
			},
		},
		Commands: []*cli.Command{
			serveCommand(), browsersCommand(), tabsCommand(), execCommand(),
			openCommand(), navigateCommand(), closeCommand(), focusCommand(), screenshotCommand(),
			threadsCommand(), vidaraCommand(), facebookCommand(), mcpCommand(),
		},
	}
}

// client calls the server, and says plainly when it is not running.
func client(cmd *cli.Command) remotev1connect.RemoteServiceClient {
	return hub.NewClient(cmd.String("server"))
}

func clientError(cmd *cli.Command, err error) error {
	return hub.ClientError(err, cmd.String("server"))
}

var (
	tabFlag     = &cli.IntFlag{Name: "tab", Usage: "tab id from `san_remote tabs` (default: the active tab of the focused window)"}
	timeoutFlag = &cli.DurationFlag{Name: "timeout", Value: 30 * time.Second, Usage: "how long to wait for the browser"}
)

func timeoutMs(cmd *cli.Command) int32 { return int32(cmd.Duration("timeout").Milliseconds()) }

// ---------------------------------------------------------------- serve ---

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "run the server the extension attaches to",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: "127.0.0.1:7476", Usage: "address to listen on (loopback only)"},
			&cli.StringSliceFlag{Name: "extension-id", Value: []string{hub.ExtensionID}, Usage: "extension ids allowed to attach"},
			&cli.StringFlag{Name: "log-level", Value: "info", Usage: "debug, info, warn or error"},
		},
		Action: runServe,
	}
}

func runServe(ctx context.Context, cmd *cli.Command) error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
		return fmt.Errorf("bad --log-level %q", cmd.String("log-level"))
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	addr := cmd.String("listen")
	if !hub.IsLoopback(addr) {
		// There is no token yet: anyone who can reach the server can run code
		// in every site the browser is signed in to.
		return fmt.Errorf("--listen %s: san_remote only listens on a loopback address", addr)
	}

	h := hub.New(log)
	// No WriteTimeout: the Attach streams stay open as long as the browser does.
	srv := &http.Server{Addr: addr, Handler: h.Handler(cmd.StringSlice("extension-id")...), ReadHeaderTimeout: 10 * time.Second}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("san_remote listening", "addr", addr, "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	h.Close()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

// ------------------------------------------------------------- browsers ---

func browsersCommand() *cli.Command {
	return &cli.Command{
		Name:  "browsers",
		Usage: "list the attached browsers",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			r, err := client(cmd).ListBrowsers(ctx, connect.NewRequest(&v1.ListBrowsersRequest{}))
			if err != nil {
				return clientError(cmd, err)
			}
			tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tATTACHED\tEXTENSION")
			for _, b := range r.Msg.Browsers {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", b.Id, b.Name, b.AttachedAt.AsTime().Local().Format("2006-01-02 15:04:05"), b.ExtensionVersion)
			}
			return tw.Flush()
		},
	}
}

// ----------------------------------------------------------------- tabs ---

func tabsCommand() *cli.Command {
	return &cli.Command{
		Name:  "tabs",
		Usage: "list the browser's tabs",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			r, err := client(cmd).ListTabs(ctx, connect.NewRequest(&v1.ListTabsRequest{Browser: cmd.String("browser")}))
			if err != nil {
				return clientError(cmd, err)
			}
			tw := tabwriter.NewWriter(cmd.Root().Writer, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tWINDOW\tACTIVE\tTITLE\tURL")
			for _, t := range r.Msg.Tabs {
				active := ""
				if t.Active {
					active = "*"
				}
				fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\n", t.Id, t.WindowId, active, clip(t.Title, 50), t.Url)
			}
			return tw.Flush()
		},
	}
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// ----------------------------------------------------------------- exec ---

func execCommand() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "run JavaScript in a tab and print what it returns, as JSON",
		ArgsUsage: "CODE | -",
		Description: "CODE is the body of an async function: use return and await.\n" +
			"With - or --file, the code is read from stdin or the file.",
		Flags: []cli.Flag{
			tabFlag, timeoutFlag,
			&cli.StringFlag{Name: "world", Value: "isolated", Usage: "isolated (sees the DOM, not the page's JavaScript) or main (the page's own world)"},
			&cli.StringFlag{Name: "file", Usage: "read the code from this file"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			code, err := readCode(cmd)
			if err != nil {
				return err
			}
			world, err := mcp.ParseWorld(cmd.String("world"))
			if err != nil {
				return err
			}
			r, err := client(cmd).Exec(ctx, connect.NewRequest(&v1.ExecRequest{
				Browser: cmd.String("browser"), TabId: int32(cmd.Int("tab")), Code: code, World: world, TimeoutMs: timeoutMs(cmd),
			}))
			if err != nil {
				return clientError(cmd, err)
			}
			var pretty bytes.Buffer
			if json.Indent(&pretty, []byte(r.Msg.ResultJson), "", "  ") != nil {
				pretty.Reset()
				pretty.WriteString(r.Msg.ResultJson)
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, pretty.String())
			return err
		},
	}
}

func readCode(cmd *cli.Command) (string, error) {
	var b []byte
	var err error
	switch {
	case cmd.String("file") != "":
		b, err = os.ReadFile(cmd.String("file"))
	case cmd.Args().First() == "-":
		b, err = io.ReadAll(cmd.Root().Reader)
	default:
		b = []byte(strings.Join(cmd.Args().Slice(), " "))
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(b)) == "" {
		return "", errors.New("no code: give it as an argument, with - for stdin, or with --file")
	}
	return string(b), nil
}

// ----------------------------------------------------------- open, etc. ---

func openCommand() *cli.Command {
	return &cli.Command{
		Name:      "open",
		Usage:     "open a url in a new tab and wait until it has loaded",
		ArgsUsage: "URL",
		Flags: []cli.Flag{
			timeoutFlag,
			&cli.BoolFlag{Name: "background", Usage: "open it without switching to it"},
			&cli.BoolFlag{Name: "focus", Usage: "also bring the browser window to the front"},
			&cli.BoolFlag{Name: "new-window", Usage: "open it in a new window of its own, the size of the current one"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.Args().First()
			if url == "" {
				return errors.New("open needs a URL")
			}
			r, err := client(cmd).OpenTab(ctx, connect.NewRequest(&v1.OpenTabRequest{
				Browser: cmd.String("browser"), Url: url, Background: cmd.Bool("background"), Focus: cmd.Bool("focus"), NewWindow: cmd.Bool("new-window"), TimeoutMs: timeoutMs(cmd),
			}))
			if err != nil {
				return clientError(cmd, err)
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, mcp.FormatTab(r.Msg.Tab))
			return err
		},
	}
}

func navigateCommand() *cli.Command {
	return &cli.Command{
		Name:      "navigate",
		Usage:     "load a url in a tab and wait until it has loaded",
		ArgsUsage: "URL",
		Flags:     []cli.Flag{tabFlag, timeoutFlag},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			url := cmd.Args().First()
			if url == "" {
				return errors.New("navigate needs a URL")
			}
			r, err := client(cmd).Navigate(ctx, connect.NewRequest(&v1.NavigateRequest{
				Browser: cmd.String("browser"), TabId: int32(cmd.Int("tab")), Url: url, TimeoutMs: timeoutMs(cmd),
			}))
			if err != nil {
				return clientError(cmd, err)
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, mcp.FormatTab(r.Msg.Tab))
			return err
		},
	}
}

func closeCommand() *cli.Command {
	return &cli.Command{
		Name:  "close",
		Usage: "close a tab",
		Flags: []cli.Flag{&cli.IntFlag{Name: "tab", Required: true, Usage: "tab id from `san_remote tabs`"}},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, err := client(cmd).CloseTab(ctx, connect.NewRequest(&v1.CloseTabRequest{Browser: cmd.String("browser"), TabId: int32(cmd.Int("tab"))}))
			return clientError(cmd, err)
		},
	}
}

func focusCommand() *cli.Command {
	return &cli.Command{
		Name:  "focus",
		Usage: "make a tab the active one and bring its window to the front",
		Flags: []cli.Flag{tabFlag},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			r, err := client(cmd).FocusTab(ctx, connect.NewRequest(&v1.FocusTabRequest{Browser: cmd.String("browser"), TabId: int32(cmd.Int("tab"))}))
			if err != nil {
				return clientError(cmd, err)
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, mcp.FormatTab(r.Msg.Tab))
			return err
		},
	}
}

func screenshotCommand() *cli.Command {
	return &cli.Command{
		Name:  "screenshot",
		Usage: "save what a tab shows as a PNG; the tab becomes the active one of its window",
		Flags: []cli.Flag{
			tabFlag,
			&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Value: "screenshot.png", Usage: "the PNG file to write"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			r, err := client(cmd).Screenshot(ctx, connect.NewRequest(&v1.ScreenshotRequest{Browser: cmd.String("browser"), TabId: int32(cmd.Int("tab"))}))
			if err != nil {
				return clientError(cmd, err)
			}
			if err := os.WriteFile(cmd.String("out"), r.Msg.Png, 0o644); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.Root().Writer, cmd.String("out"))
			return err
		},
	}
}

// ------------------------------------------------------------------ mcp ---

func mcpCommand() *cli.Command {
	return &cli.Command{
		Name:  "mcp",
		Usage: "serve the browser to Claude over MCP (stdio); needs a running `san_remote serve`",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Threads' and Vidara's steps go to stderr, which Claude Code keeps in its MCP log;
			// stdout carries only JSON-RPC.
			s := &mcp.Server{Client: client(cmd), Threads: threadsService(cmd), Vidara: vidaraService(cmd), Facebook: facebookService(cmd), Server: cmd.String("server"), Version: version}
			return s.Serve(ctx, cmd.Root().Reader, cmd.Root().Writer)
		},
	}
}

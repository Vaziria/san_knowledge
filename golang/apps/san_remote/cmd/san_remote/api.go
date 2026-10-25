package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_remote/internal/api"
	"github.com/wargasipil/san_remote/internal/hub"
)

// ------------------------------------------------------------------ api ---

func apiCommand() *cli.Command {
	return &cli.Command{
		Name:  "api",
		Usage: "serve the site services (Threads, X, Facebook, Vidara, Medium, LinkedIn, TikTok, ChatGPT, Gemini, Claude) over Connect (gRPC) for other programs, such as the tower; needs a running `san_remote serve`",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: api.DefaultAddr, Usage: "address to listen on (loopback only)"},
			&cli.StringFlag{Name: "log-level", Value: "info", Usage: "debug, info, warn or error"},
		},
		Action: runAPI,
	}
}

func runAPI(ctx context.Context, cmd *cli.Command) error {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
		return fmt.Errorf("bad --log-level %q", cmd.String("log-level"))
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	addr := cmd.String("listen")
	if !hub.IsLoopback(addr) {
		// Like serve: anyone who can reach it can post as the user.
		return fmt.Errorf("--listen %s: san_remote only listens on a loopback address", addr)
	}

	// The sites' steps go to the log, as debug lines.
	a := &api.API{
		Client:   client(cmd),
		Server:   cmd.String("server"),
		Threads:  threadsService(cmd),
		X:        xService(cmd),
		Facebook: facebookService(cmd),
		Vidara:   vidaraService(cmd),
		Medium:   mediumService(cmd),
		LinkedIn: linkedinService(cmd),
		TikTok:   tiktokService(cmd),
		ChatGPT:  chatgptService(cmd),
		Gemini:   geminiService(cmd),
		Claude:   claudeService(cmd),
	}
	progress := func(site string) func(string) {
		return func(step string) { log.Debug(site, "step", step) }
	}
	a.Threads.Progress, a.X.Progress, a.Facebook.Progress, a.Vidara.Progress = progress("threads"), progress("x"), progress("facebook"), progress("vidara")
	a.Medium.Progress, a.LinkedIn.Progress, a.TikTok.Progress, a.ChatGPT.Progress = progress("medium"), progress("linkedin"), progress("tiktok"), progress("chatgpt")
	a.Gemini.Progress, a.Claude.Progress = progress("gemini"), progress("claude")

	// No WriteTimeout: a post takes a minute or two.
	srv := &http.Server{Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	log.Info("san_remote api listening", "addr", addr, "server", a.Server, "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(sctx)
}

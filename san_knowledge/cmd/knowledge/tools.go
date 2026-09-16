package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"san_knowledge/internal/kb"
	"san_knowledge/internal/mcpserver"
	"san_knowledge/internal/ownview"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

// newAnnotator is swapped in tests.
var newAnnotator = func(model string) kb.Annotator { return kb.ClaudeCLI{Model: model} }

// authorFrom reads --author (or $KNOWLEDGE_AUTHOR via the flag source).
func authorFrom(cmd *cli.Command, def string) string {
	if v := cmd.String("author"); v != "" {
		return v
	}
	return def
}

func opener(dir string) func(func(*kb.Store) error) error {
	return func(fn func(*kb.Store) error) error { return kb.With(dir, fn) }
}

// explain prints the knowledge context around a free-text query.
func (c *app) explain() error {
	ex, err := c.store.Explain(strings.Join(c.args, " "), int(c.cmd.Int("limit")), int(c.cmd.Int("depth")))
	if err != nil {
		return err
	}
	return c.emit(ex, func() { fmt.Fprint(c.out, ex.Markdown()) })
}

// syncCmd tracks ./docs, then runs the AI step unless --no-ai.
func syncCmd(ctx context.Context, cmd *cli.Command) error {
	dir, err := dataDir(cmd.String("data"))
	if err != nil {
		return err
	}
	w, asJSON := cmd.Root().Writer, cmd.Bool("json")

	var rep *kb.SyncReport
	if err := kb.With(dir, func(s *kb.Store) error {
		rep, err = s.Sync()
		return err
	}); err != nil {
		return err
	}
	if !asJSON {
		printSyncReport(w, rep)
	}

	var aiRep *kb.AIReport
	if !cmd.Bool("no-ai") {
		if !asJSON {
			fmt.Fprintf(w, "\nAI step (claude -p, model %s)…\n", cmd.String("model"))
		}
		aiRep, err = kb.RunAI(ctx, dir, newAnnotator(cmd.String("model")), kb.AIOptions{
			DryRun:   cmd.Bool("dry-run"),
			Parallel: int(cmd.Int("parallel")),
			Progress: func(loc string, err error) {
				if asJSON {
					return
				}
				if err != nil {
					fmt.Fprintf(w, "  ✗ %s: %v\n", loc, err)
				} else {
					fmt.Fprintf(w, "  ✓ %s\n", loc)
				}
			},
		})
		if err != nil {
			return err
		}
		if !asJSON {
			printAIReport(w, aiRep, cmd.Bool("dry-run"))
		}
	}
	if asJSON {
		return emit(w, true, map[string]any{"sync": rep, "ai": aiRep}, nil)
	}
	if aiRep != nil && len(aiRep.Failed) > 0 {
		return fmt.Errorf("AI step failed for %d doc(s); they stay pending and are retried on the next sync", len(aiRep.Failed))
	}
	return nil
}

func printSyncReport(w io.Writer, r *kb.SyncReport) {
	fmt.Fprintf(w, "docs:     +%d ~%d -%d\n", len(r.DocsAdded), len(r.DocsUpdated), len(r.DocsRemoved))
	fmt.Fprintf(w, "sections: +%d ~%d -%d", r.SectionsAdded, r.SectionsUpdated, r.SectionsRemoved)
	if r.SummariesKept > 0 {
		fmt.Fprintf(w, " (%d renamed, summary kept)", r.SummariesKept)
	}
	fmt.Fprintf(w, "\nneeds summary: %d\n", r.NeedsSummary)
	for _, e := range r.Errors {
		fmt.Fprintln(w, "  error:", e)
	}
}

func printAIReport(w io.Writer, r *kb.AIReport, dryRun bool) {
	if len(r.Docs) == 0 {
		fmt.Fprintln(w, "  nothing to do")
		return
	}
	if dryRun {
		locs := make([]string, 0, len(r.Proposals))
		for loc := range r.Proposals {
			locs = append(locs, loc)
		}
		sort.Strings(locs)
		for _, loc := range locs {
			p := r.Proposals[loc]
			fmt.Fprintf(w, "\n%s\n  domains: %s\n  summary: %s\n  keyword: %s\n", loc, strings.Join(p.Domains, ", "), p.Summary, strings.Join(p.Keyword, ", "))
			for _, s := range p.Sections {
				fmt.Fprintf(w, "  - %s: %s [%s]\n", s.Key, s.Summary, strings.Join(s.Keyword, ", "))
			}
		}
		fmt.Fprintln(w, "\n(dry run: nothing saved)")
		return
	}
	fmt.Fprintf(w, "summarized: %d nodes in %d docs\n", r.Summarized, len(r.Docs)-len(r.Failed))
	if len(r.DomainsCreated) > 0 {
		fmt.Fprintf(w, "new domains: %s\n", strings.Join(r.DomainsCreated, ", "))
	}
}

// startupSync runs a structural sync (no AI) for view/mcp; failures are
// reported but do not stop the server.
func startupSync(dir string, log io.Writer) {
	err := kb.With(dir, func(s *kb.Store) error {
		r, err := s.Sync()
		if err == nil {
			fmt.Fprintf(log, "synced docs: +%d ~%d -%d, sections +%d ~%d -%d, needs summary %d\n",
				len(r.DocsAdded), len(r.DocsUpdated), len(r.DocsRemoved),
				r.SectionsAdded, r.SectionsUpdated, r.SectionsRemoved, r.NeedsSummary)
		}
		return err
	})
	if err != nil {
		fmt.Fprintln(log, "sync failed:", err)
	}
}

// view serves the Knowledge UI and opens it in the browser until Ctrl+C.
// The database is opened per request, so the CLI keeps working meanwhile.
func view(cmd *cli.Command, dir string, out io.Writer) error {
	if err := kb.With(dir, func(*kb.Store) error { return nil }); err != nil {
		return err
	}
	if !cmd.Bool("no-sync") {
		startupSync(dir, out)
	}
	ln, err := net.Listen("tcp", cmd.String("addr"))
	if err != nil && !cmd.IsSet("addr") {
		ln, err = net.Listen("tcp", "127.0.0.1:0") // default port busy: pick a free one
	}
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"

	srv := &http.Server{
		Handler: ownview.Handler(ownview.Options{
			Dir: dir, Open: opener(dir), Author: authorFrom(cmd, "user"), Annotator: newAnnotator(cmd.String("model")),
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	fmt.Fprintf(out, "Knowledge UI: %s\ndata:         %s\npress Ctrl+C to stop\n", url, dir)
	if !cmd.Bool("no-open") {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(os.Stderr, "could not open browser:", err)
		}
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-stop:
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// serveMCP runs the MCP stdio server. Nothing else may write to stdout.
func serveMCP(cmd *cli.Command, dir string, in io.Reader, out io.Writer) error {
	if err := kb.With(dir, func(*kb.Store) error { return nil }); err != nil {
		return err
	}
	if !cmd.Bool("no-sync") {
		startupSync(dir, os.Stderr)
	}
	fmt.Fprintf(os.Stderr, "knowledge mcp: serving %s on stdio\n", dir)
	return mcpserver.New(opener(dir), authorFrom(cmd, "ai"), version).Serve(in, out)
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

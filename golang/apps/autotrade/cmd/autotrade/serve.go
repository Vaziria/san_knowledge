package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/wargasipil/autotrade/internal/bot"
	"github.com/wargasipil/autotrade/internal/view"
)

// parseInterleaved parses flags that come before, between or after the
// words of a command ("open long -usdt 500", "knowledge position 3
// -snapshot"): the flag package stops at the first word, so parsing starts
// again after each one. It returns the words.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return words, nil
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

// serveView runs the knowledge preview until ctx ends (Ctrl+C). It syncs the
// graph first, so the page shows the current journal.
func serveView(ctx context.Context, stdout, stderr io.Writer, b *bot.Bot, addr string, open bool) error {
	if res, err := b.SyncKnowledge(ctx); err != nil {
		fmt.Fprintln(stderr, "autotrade: knowledge sync:", err)
	} else {
		fmt.Fprintf(stdout, "Synced: %d pairs, %d positions (%d open), %d approaches.\n", res.Pairs, res.Positions, res.Open, res.Approaches)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	url := "http://" + ln.Addr().String() + "/"
	srv := &http.Server{
		Handler:           view.Handler(view.Options{Dir: b.KnowledgeDir(), Sync: b.SyncKnowledge}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	fmt.Fprintf(stdout, "Knowledge view: %s\ndata:           %s\npress Ctrl+C to stop\n", url, b.KnowledgeDir())
	if open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stderr, "could not open the browser:", err)
		}
	}
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shut)
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

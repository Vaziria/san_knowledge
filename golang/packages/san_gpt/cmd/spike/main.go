// Command spike proves the design end to end: attach to a logged-in Chrome,
// install the tee, send a prompt, and stream the answer back into Go.
//
// Sign in to chatgpt.com once, by hand, in Chrome on the profile the san_*
// tools share (close the window when done):
//
//	spike -login
//
// then:
//
//	spike -prompt "halo, apa kabar?"
//
// -profile picks another profile (a name or a folder), and -remote a Chrome
// you started yourself with --remote-debugging-port and keep running.
//
// Further prompts are read from stdin, one per line, which is the turn-taking
// loop from the design doc with a human at one end of it.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	san_gpt "github.com/wargasipil/san_gpt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spike:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		remote   = flag.String("remote", "", "attach to a Chrome already running with --remote-debugging-port, e.g. http://127.0.0.1:9222")
		profile  = flag.String("profile", "", `Chrome profile: a name (default "default", shared by the san_* tools) or a folder`)
		login    = flag.Bool("login", false, "open chatgpt.com's sign-in page on the profile, and return when that window is closed")
		execPath = flag.String("exec", "", "path to the Chrome binary, when launching")
		prompt   = flag.String("prompt", "hello", "opening prompt")
		resume   = flag.String("conversation", "", "resume an existing conversation id")
		idle     = flag.Duration("idle-timeout", 90*time.Second, "give up after this long with no data mid-answer")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var opts []san_gpt.Option
	if *remote != "" {
		opts = append(opts, san_gpt.WithRemote(*remote))
	}
	if *profile != "" {
		opts = append(opts, san_gpt.WithProfile(*profile))
	}
	if *execPath != "" {
		opts = append(opts, san_gpt.WithExecPath(*execPath))
	}

	if *login {
		fmt.Fprintln(os.Stderr, "== Chrome is open on chatgpt.com's sign-in page. Sign in there, then close that window.")
		return san_gpt.Login(ctx, opts...)
	}

	client, err := san_gpt.New(ctx, opts...)
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Fprintln(os.Stderr, "== opening a tab and installing the tee")

	convOpts := []san_gpt.ConvOption{
		san_gpt.WithPrompt(*prompt),
		san_gpt.WithIdleTimeout(*idle),
		// Live tokens, which is the whole point of teeing the stream rather
		// than scraping the rendered answer.
		san_gpt.WithDelta(func(s string) { fmt.Print(s) }),
	}
	if *resume != "" {
		convOpts = append(convOpts, san_gpt.WithConversation(*resume))
	}

	fmt.Fprintf(os.Stderr, "== you: %s\n", *prompt)

	conv, err := client.NewConversation(ctx, convOpts...)
	if err != nil {
		return explain(err)
	}
	defer conv.Close()

	stdin := bufio.NewScanner(os.Stdin)

	// The loop from the design doc, verbatim in shape.
	for conv.Receive() {
		msg := conv.Read()
		fmt.Fprintf(os.Stderr, "\n== turn complete: %d chars, conversation %s\n", len(msg.Text), conv.ID())

		fmt.Fprint(os.Stderr, "you: ")
		if !stdin.Scan() {
			break
		}
		next := strings.TrimSpace(stdin.Text())
		if next == "" || next == "exit" {
			break
		}
		if err := conv.Send(ctx, next); err != nil {
			return explain(err)
		}
	}
	return explain(conv.Err())
}

func explain(err error) error {
	if errors.Is(err, san_gpt.ErrNeedsHuman) {
		return fmt.Errorf("%w\n\nLook at the browser window: the session may have expired, or a\nverification challenge may be waiting. Nothing here can clear it.", err)
	}
	return err
}

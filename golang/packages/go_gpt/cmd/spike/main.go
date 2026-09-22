// Command spike proves the design end to end: attach to a logged-in Chrome,
// install the tee, send a prompt, and stream the answer back into Go.
//
// Start Chrome yourself and log in by hand first:
//
//	chrome --remote-debugging-port=9222 --user-data-dir=/path/to/profile
//
// then:
//
//	spike -remote http://127.0.0.1:9222 -prompt "halo, apa kabar?"
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

	go_gpt "github.com/wargasipil/go_gpt"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spike:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		remote      = flag.String("remote", "", "attach to a Chrome already running with --remote-debugging-port, e.g. http://127.0.0.1:9222")
		userDataDir = flag.String("user-data-dir", "", "launch Chrome against this profile directory instead of attaching")
		execPath    = flag.String("exec", "", "path to the Chrome binary, when launching")
		prompt      = flag.String("prompt", "hello", "opening prompt")
		resume      = flag.String("conversation", "", "resume an existing conversation id")
		idle        = flag.Duration("idle-timeout", 90*time.Second, "give up after this long with no data mid-answer")
	)
	flag.Parse()

	if *remote == "" && *userDataDir == "" {
		return errors.New("one of -remote or -user-data-dir is required; see -h")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	var opts []go_gpt.Option
	switch {
	case *remote != "":
		opts = append(opts, go_gpt.WithRemote(*remote))
	default:
		opts = append(opts, go_gpt.WithUserDataDir(*userDataDir))
	}
	if *execPath != "" {
		opts = append(opts, go_gpt.WithExecPath(*execPath))
	}

	client, err := go_gpt.New(ctx, opts...)
	if err != nil {
		return err
	}
	defer client.Close()

	fmt.Fprintln(os.Stderr, "== opening a tab and installing the tee")

	convOpts := []go_gpt.ConvOption{
		go_gpt.WithPrompt(*prompt),
		go_gpt.WithIdleTimeout(*idle),
		// Live tokens, which is the whole point of teeing the stream rather
		// than scraping the rendered answer.
		go_gpt.WithDelta(func(s string) { fmt.Print(s) }),
	}
	if *resume != "" {
		convOpts = append(convOpts, go_gpt.WithConversation(*resume))
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
	if errors.Is(err, go_gpt.ErrNeedsHuman) {
		return fmt.Errorf("%w\n\nLook at the browser window: the session may have expired, or a\nverification challenge may be waiting. Nothing here can clear it.", err)
	}
	return err
}

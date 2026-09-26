// Command san_youtube prints the chat of a YouTube live stream as it happens.
//
//	san_youtube @KompasTV
//	san_youtube https://www.youtube.com/watch?v=DOOrIxw5xOw
//	san_youtube -json -for 1m DOOrIxw5xOw > chat.jsonl
//
// Messages go to stdout, one per line; status lines, starting with "==", go
// to stderr. It stops when the stream ends, on Ctrl+C, or after -for.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	san_youtube "github.com/wargasipil/san_youtube"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		// The package's errors already say where they come from.
		msg := err.Error()
		if !strings.HasPrefix(msg, "san_youtube: ") {
			msg = "san_youtube: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("san_youtube", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		asJSON  = fs.Bool("json", false, "print one JSON object per message instead of text")
		newOnly = fs.Bool("new", false, "skip the recent history the chat starts with")
		dur     = fs.Duration("for", 0, "stop after this long, e.g. 1m (0: until the stream ends or Ctrl+C)")
		lang    = fs.String("lang", "en", `language for YouTube's own texts; "id" shows amounts as "Rp 20.000"`)
		poll    = fs.Duration("poll", 0, "poll at least this often, e.g. 1s, instead of as YouTube asks (about every 10s); YouTube's pace again for 10 minutes after it says too many requests")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: san_youtube [flags] <video URL | video ID | @handle | channel URL>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("give exactly one stream")
	}

	if *dur > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *dur)
		defer cancel()
	}

	var chatOpts []san_youtube.ChatOption
	if *newOnly {
		chatOpts = append(chatOpts, san_youtube.WithoutHistory())
	}
	if *poll > 0 {
		chatOpts = append(chatOpts, san_youtube.WithPollInterval(*poll))
	}
	client := san_youtube.New(san_youtube.WithLanguage(*lang))
	chat, err := client.Open(ctx, fs.Arg(0), chatOpts...)
	if err != nil {
		return quiet(ctx, err)
	}
	fmt.Fprintf(stderr, "== reading the live chat of https://www.youtube.com/watch?v=%s\n", chat.VideoID())

	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	count := 0
	defer func() { fmt.Fprintf(stderr, "== %d messages\n", count) }()

	for msg, err := range chat.Messages(ctx) {
		if errors.Is(err, san_youtube.ErrEnded) {
			fmt.Fprintln(stderr, "== the stream has ended")
			return nil
		}
		if err != nil {
			return quiet(ctx, err)
		}
		count++
		if *asJSON {
			if err := enc.Encode(msg); err != nil {
				return err
			}
			continue
		}
		fmt.Fprintln(stdout, line(msg))
	}
	return nil
}

// quiet turns the end the user asked for, Ctrl+C or -for, into a clean exit.
// It asks ctx rather than the error: an HTTP client timeout also reads as
// context.DeadlineExceeded, and that one is a failure to report.
func quiet(ctx context.Context, err error) error {
	if ctx.Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		return nil
	}
	return err
}

// line renders a message for the terminal: time, what kind of event it is
// when it is not plain chat, the author and the text.
func line(msg san_youtube.Message) string {
	b := msg.Meta()
	ts := b.Time.Local().Format("15:04:05")
	who := author(b.Author)
	switch m := msg.(type) {
	case *san_youtube.TextMessage:
		return fmt.Sprintf("%s %s: %s", ts, who, m.Text)
	case *san_youtube.SuperChat:
		return fmt.Sprintf("%s [Super Chat %s] %s: %s", ts, m.Amount.Text, who, m.Text)
	case *san_youtube.SuperSticker:
		return fmt.Sprintf("%s [Super Sticker %s] %s: %s", ts, m.Amount.Text, who, m.Sticker)
	case *san_youtube.Membership:
		if m.Milestone {
			return fmt.Sprintf("%s [%s] %s: %s", ts, m.Header, who, m.Text)
		}
		return fmt.Sprintf("%s [New member] %s: %s", ts, who, m.Subtext)
	case *san_youtube.GiftPurchase:
		return fmt.Sprintf("%s [Gift] %s: %s", ts, who, m.Text)
	case *san_youtube.GiftRedemption:
		return fmt.Sprintf("%s [Gift] %s %s", ts, who, m.Text)
	case *san_youtube.Deletion:
		return strings.TrimSpace(fmt.Sprintf("%s [Deleted] message %s %s", ts, m.TargetID, m.Text))
	case *san_youtube.AuthorDeletion:
		return strings.TrimSpace(fmt.Sprintf("%s [Deleted] all messages by %s %s", ts, b.Author.ChannelID, m.Text))
	case *san_youtube.Replacement:
		inner := line(m.Message)
		_, rest, _ := strings.Cut(inner, " ")
		return fmt.Sprintf("%s [Replaces %s] %s", ts, m.TargetID, rest)
	}
	return fmt.Sprintf("%s [%s] %s", ts, msg.Kind(), who)
}

func author(a san_youtube.Author) string {
	var tags []string
	for _, t := range []struct {
		on   bool
		name string
	}{{a.Owner, "owner"}, {a.Moderator, "mod"}, {a.Verified, "verified"}, {a.Member, "member"}} {
		if t.on {
			tags = append(tags, t.name)
		}
	}
	if len(tags) == 0 {
		return a.Name
	}
	return a.Name + " [" + strings.Join(tags, ", ") + "]"
}

// Keywords watches a live stream's chat for words, such as the brands or
// products a host talks about, and prints every message that mentions one. At
// the end it says how many messages, and how many different viewers, mentioned
// each word.
//
//	go run ./examples/keywords -for 10m @KompasTV shopee tokopedia "tiktok shop"
//
// A word matches anywhere in a message, ignoring case, so it can be a phrase.
// It starts with the recent history the chat replays, and stops when the
// stream ends, on Ctrl+C, or after -for.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	san_youtube "github.com/wargasipil/san_youtube"
)

type count struct {
	messages int
	viewers  map[string]bool
}

func main() {
	log.SetFlags(0)
	dur := flag.Duration("for", 0, "stop after this long, e.g. 10m (0: until the stream ends or Ctrl+C)")
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("usage: keywords [-for 10m] <video URL | video ID | @handle | channel URL> <word>...")
	}
	words := flag.Args()[1:]
	counts := make([]count, len(words))
	for i, w := range words {
		words[i] = strings.ToLower(w)
		counts[i].viewers = map[string]bool{}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *dur > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *dur)
		defer cancel()
	}

	client := san_youtube.New()
	chat, err := client.Open(ctx, flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("watching https://www.youtube.com/watch?v=" + chat.VideoID())

	read := 0
	for msg, err := range chat.Messages(ctx) {
		if err != nil {
			if errors.Is(err, san_youtube.ErrEnded) {
				fmt.Println("the stream has ended")
			} else if ctx.Err() == nil {
				log.Print(err) // still report what was counted
			}
			break
		}
		t := text(msg)
		if t == "" {
			continue
		}
		read++
		lower := strings.ToLower(t)
		a := msg.Meta().Author
		var hits []string
		for i, w := range words {
			if strings.Contains(lower, w) {
				counts[i].messages++
				counts[i].viewers[cmp.Or(a.ChannelID, a.Name)] = true
				hits = append(hits, w)
			}
		}
		if len(hits) > 0 {
			ts := msg.Meta().Time.Local().Format("15:04:05")
			fmt.Printf("%s [%s] %s: %s\n", ts, strings.Join(hits, ", "), a.Name, t)
		}
	}

	fmt.Printf("\n== %d messages read\n", read)
	for i, w := range words {
		fmt.Printf("%-20s %5d messages  %5d viewers\n", w, counts[i].messages, len(counts[i].viewers))
	}
}

// text returns what the author wrote, for the kinds of message that carry
// their own words, and "" for the rest.
func text(msg san_youtube.Message) string {
	switch m := msg.(type) {
	case *san_youtube.TextMessage:
		return m.Text
	case *san_youtube.SuperChat:
		return m.Text
	case *san_youtube.Membership:
		return m.Text
	case *san_youtube.Replacement:
		return text(m.Message)
	}
	return ""
}

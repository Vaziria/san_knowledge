// Multi reads the chats of several live streams at once and prints them as one
// feed, each line tagged with the stream it came from.
//
//	go run ./examples/multi -for 5m @KompasTV DOOrIxw5xOw
//
// One Client serves every stream, since a Client can be shared between
// goroutines. A Chat cannot, so each goroutine opens and reads its own. A
// stream that cannot be opened, or that ends, is reported and the others go
// on. Keep it to a handful: every stream is one more browser tab's worth of
// requests from the same IP, and YouTube rate-limits.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	san_youtube "github.com/wargasipil/san_youtube"
)

func main() {
	log.SetFlags(0)
	dur := flag.Duration("for", 0, "stop after this long, e.g. 5m (0: until every stream ends or Ctrl+C)")
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("usage: multi [-for 5m] <video URL | video ID | @handle | channel URL>...")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *dur > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *dur)
		defer cancel()
	}

	client := san_youtube.New()
	lines := make(chan string)
	var wg sync.WaitGroup
	for _, input := range flag.Args() {
		wg.Go(func() {
			err := read(ctx, client, input, lines)
			if err != nil && ctx.Err() == nil { // Ctrl+C and -for are no failure
				lines <- fmt.Sprintf("[%s] %v", input, err)
			}
		})
	}
	go func() {
		wg.Wait()
		close(lines)
	}()
	// One goroutine prints, so lines from different streams never mix.
	for l := range lines {
		fmt.Println(l)
	}
}

// read sends one stream's messages to out until it ends or ctx is done. The
// stream ending is returned as ErrEnded, like any other error.
func read(ctx context.Context, client *san_youtube.Client, input string, out chan<- string) error {
	chat, err := client.Open(ctx, input)
	if err != nil {
		return err
	}
	out <- fmt.Sprintf("[%s] reading https://www.youtube.com/watch?v=%s", input, chat.VideoID())
	for msg, err := range chat.Messages(ctx) {
		if err != nil {
			return err
		}
		ts := msg.Meta().Time.Local().Format("15:04:05")
		switch m := msg.(type) {
		case *san_youtube.TextMessage:
			out <- fmt.Sprintf("[%s] %s %s: %s", input, ts, m.Author.Name, m.Text)
		case *san_youtube.SuperChat:
			out <- fmt.Sprintf("[%s] %s [Super Chat %s] %s: %s", input, ts, m.Amount.Text, m.Author.Name, m.Text)
		}
	}
	return nil
}

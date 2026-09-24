// Basic prints a live stream's chat as it happens, until the stream ends or
// Ctrl+C.
//
//	go run ./examples/basic @KompasTV
//	go run ./examples/basic https://www.youtube.com/watch?v=DOOrIxw5xOw
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	san_youtube "github.com/wargasipil/san_youtube"
)

func main() {
	log.SetFlags(0)
	if len(os.Args) != 2 {
		log.Fatal("usage: basic <video URL | video ID | @handle | channel URL>")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := san_youtube.New()
	chat, err := client.Open(ctx, os.Args[1])
	if err != nil {
		log.Fatal(err) // e.g. ErrNoLiveStream, ErrChatDisabled, ErrMembersOnly
	}
	fmt.Println("reading https://www.youtube.com/watch?v=" + chat.VideoID())

	for msg, err := range chat.Messages(ctx) {
		if err != nil {
			switch {
			case errors.Is(err, san_youtube.ErrEnded):
				fmt.Println("the stream has ended")
			case ctx.Err() != nil:
				// Ctrl+C: Messages ends with ctx.Err(), which is no failure.
			default:
				log.Fatal(err)
			}
			return
		}
		show(msg)
	}
}

// show prints the kinds of message most readers want; the rest (deletions,
// gift redemptions) are skipped.
func show(msg san_youtube.Message) {
	b := msg.Meta()
	ts := b.Time.Local().Format("15:04:05")
	switch m := msg.(type) {
	case *san_youtube.TextMessage:
		fmt.Printf("%s %s: %s\n", ts, m.Author.Name, m.Text)
	case *san_youtube.SuperChat:
		fmt.Printf("%s [Super Chat %s] %s: %s\n", ts, m.Amount.Text, m.Author.Name, m.Text)
	case *san_youtube.SuperSticker:
		fmt.Printf("%s [Super Sticker %s] %s: %s\n", ts, m.Amount.Text, m.Author.Name, m.Sticker)
	case *san_youtube.Membership:
		if m.Milestone {
			fmt.Printf("%s [%s] %s: %s\n", ts, m.Header, m.Author.Name, m.Text)
		} else {
			fmt.Printf("%s [New member] %s: %s\n", ts, m.Author.Name, m.Subtext)
		}
	case *san_youtube.GiftPurchase:
		fmt.Printf("%s [Gift] %s: %s\n", ts, m.Author.Name, m.Text)
	case *san_youtube.Replacement:
		// A message held for review and now released: new to this reader.
		show(m.Message)
	}
}

// Superchats counts the money a live stream takes in Super Chats and Super
// Stickers. It prints each one as it comes, and at the end the total per
// currency and the viewers who paid most often.
//
//	go run ./examples/superchats -for 30m @KompasTV
//
// It stops when the stream ends, on Ctrl+C, or after -for.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"maps"
	"math"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"

	san_youtube "github.com/wargasipil/san_youtube"
)

// total is what one currency brought in.
type total struct {
	count int
	sum   float64
}

// supporter is one viewer who paid.
type supporter struct {
	name  string
	count int
	paid  map[string]float64 // currency → sum
}

func main() {
	log.SetFlags(0)
	dur := flag.Duration("for", 0, "stop after this long, e.g. 30m (0: until the stream ends or Ctrl+C)")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal("usage: superchats [-for 30m] <video URL | video ID | @handle | channel URL>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *dur > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *dur)
		defer cancel()
	}

	// "id" makes YouTube write amounts as "Rp 20.000"; Value and Currency are
	// read either way. WithoutHistory leaves out what was paid before Open, so
	// the totals cover exactly the time this ran.
	client := san_youtube.New(san_youtube.WithLanguage("id"))
	chat, err := client.Open(ctx, flag.Arg(0), san_youtube.WithoutHistory())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("counting Super Chats on https://www.youtube.com/watch?v=" + chat.VideoID())

	totals := map[string]*total{}
	supporters := map[string]*supporter{}
	add := func(a san_youtube.Author, amt san_youtube.Amount) {
		// Currency is empty for a symbol that is ambiguous ("kr") or unknown;
		// such amounts are counted, but cannot be added to anything.
		cur := cmp.Or(amt.Currency, "?")
		t := totals[cur]
		if t == nil {
			t = &total{}
			totals[cur] = t
		}
		t.count++
		t.sum += amt.Value

		id := cmp.Or(a.ChannelID, a.Name)
		s := supporters[id]
		if s == nil {
			s = &supporter{name: a.Name, paid: map[string]float64{}}
			supporters[id] = s
		}
		s.count++
		s.paid[cur] += amt.Value
	}

	for msg, err := range chat.Messages(ctx) {
		if err != nil {
			if errors.Is(err, san_youtube.ErrEnded) {
				fmt.Println("the stream has ended")
			} else if ctx.Err() == nil {
				log.Print(err) // still report what was counted
			}
			break
		}
		ts := msg.Meta().Time.Local().Format("15:04:05")
		switch m := msg.(type) {
		case *san_youtube.SuperChat:
			fmt.Printf("%s %s %s: %s\n", ts, m.Amount.Text, m.Author.Name, m.Text)
			add(m.Author, m.Amount)
		case *san_youtube.SuperSticker:
			fmt.Printf("%s %s %s: [sticker] %s\n", ts, m.Amount.Text, m.Author.Name, m.Sticker)
			add(m.Author, m.Amount)
		}
	}
	report(totals, supporters)
}

func report(totals map[string]*total, supporters map[string]*supporter) {
	n := 0
	for _, t := range totals {
		n += t.count
	}
	fmt.Printf("\n== %d Super Chats and Super Stickers\n", n)
	for _, cur := range slices.Sorted(maps.Keys(totals)) {
		t := totals[cur]
		if cur == "?" {
			fmt.Printf("%-4s %4d paid  (currency not recognised)\n", cur, t.count)
			continue
		}
		fmt.Printf("%-4s %4d paid  %s\n", cur, t.count, number(t.sum))
	}

	if len(supporters) == 0 {
		return
	}
	list := slices.Collect(maps.Values(supporters))
	slices.SortFunc(list, func(a, b *supporter) int {
		return cmp.Or(cmp.Compare(b.count, a.count), strings.Compare(a.name, b.name))
	})
	fmt.Println("\n== most frequent supporters")
	for _, s := range list[:min(len(list), 10)] {
		var paid []string
		for cur, v := range s.paid {
			paid = append(paid, cur+" "+number(v))
		}
		slices.Sort(paid)
		fmt.Printf("%4d  %s  (%s)\n", s.count, s.name, strings.Join(paid, ", "))
	}
}

// number writes v rounded to cents with thousands separators:
// 1250000 → "1,250,000", 12.5 → "12.5".
func number(v float64) string {
	s := strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
	whole, frac, _ := strings.Cut(s, ".")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + "," + whole[i:]
	}
	if frac != "" {
		return whole + "." + frac
	}
	return whole
}

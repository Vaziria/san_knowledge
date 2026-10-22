// Embedded runs San Message inside the program and calls it directly: it
// creates a topic and a webhook subscriber served by this same program, then
// publishes three events and prints what the webhook gets.
//
//	go run ./examples/embedded
package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/wargasipil/san_message"
)

func main() {
	dir, err := os.MkdirTemp("", "san_message_example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// The webhook: every event San Message delivers is one POST here.
	got := make(chan string)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- fmt.Sprintf("event %s on %s: %s", r.Header.Get("X-Event-Id"), r.Header.Get("X-Topic"), body)
	}))

	b, err := san_message.Open(san_message.Options{Dir: dir})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	if err := b.CreateTopic("orders"); err != nil {
		log.Fatal(err)
	}
	if err := b.CreateSubscriber("billing", "orders", san_message.Webhook, "http://"+ln.Addr().String()+"/hook"); err != nil {
		log.Fatal(err)
	}
	for _, order := range []string{"order 1", "order 2", "order 3"} {
		id, err := b.Publish("orders", []byte(order))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("published event", id)
	}
	for range 3 {
		fmt.Println("webhook got", <-got)
	}
}

package san_message_test

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/wargasipil/san_message"
)

// Calling the broker directly from Go.
func Example() {
	dir, err := os.MkdirTemp("", "san_message")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)

	b, err := san_message.Open(san_message.Options{Dir: dir})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	if err := b.CreateTopic("orders"); err != nil {
		log.Fatal(err)
	}
	for _, order := range []string{"order 1", "order 2"} {
		id, err := b.Publish("orders", []byte(order))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("published event", id)
	}
	topics, err := b.Topics()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(topics[0].Key, "last event", topics[0].LastEventID)
	// Output:
	// published event 1
	// published event 2
	// orders last event 2
}

// Mounting the connect-rpc service in an app's own server.
func ExampleBroker_Handler() {
	b, err := san_message.Open(san_message.Options{Dir: "event_data"})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	srv := &http.Server{Addr: "127.0.0.1:7070", Handler: mux}
	san_message.ConfigureServer(srv)
	log.Fatal(srv.ListenAndServe())
}

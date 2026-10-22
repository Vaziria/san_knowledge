// Publisher publishes events to a running server over connect-rpc.
//
//	go run ./examples/publisher --topic orders "order 1" "order 2"
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"

	"connectrpc.com/connect"

	v1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

func main() {
	server := flag.String("server", "http://127.0.0.1:7070", "URL of the running server")
	topic := flag.String("topic", "", "topic to publish to")
	flag.Parse()
	if *topic == "" || flag.NArg() == 0 {
		log.Fatal("usage: publisher --topic <topic> <event data>...")
	}

	// Publish is a plain unary call, so the default client (HTTP/1.1) is
	// enough; only Subscribe needs HTTP/2.
	client := messagev1connect.NewMessageServiceClient(http.DefaultClient, *server)
	for _, data := range flag.Args() {
		resp, err := client.Publish(context.Background(), connect.NewRequest(&v1.PublishRequest{
			Topic: *topic,
			Data:  []byte(data),
		}))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("published event %d: %s\n", resp.Msg.GetEventId(), data)
	}
}

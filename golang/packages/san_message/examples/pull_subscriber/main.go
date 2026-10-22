// Pull subscriber connects to a running server, prints each event it gets
// and acks it. The server sends the next event only after the ack.
//
//	go run ./cmd/san_message subscriber create app --topic orders --type pull
//	go run ./examples/pull_subscriber --subscriber app
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	v1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

func main() {
	server := flag.String("server", "http://127.0.0.1:7070", "URL of the running server")
	key := flag.String("subscriber", "", "pull subscriber key")
	flag.Parse()
	if *key == "" {
		log.Fatal("usage: pull_subscriber --subscriber <key>")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Subscribe is a bidirectional stream, so the client speaks HTTP/2
	// without TLS (h2c), like the server.
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: protocols}}
	client := messagev1connect.NewMessageServiceClient(httpClient, *server)

	stream := client.Subscribe(ctx)
	defer stream.CloseResponse()
	hello := &v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Hello{Hello: &v1.Hello{Subscriber: *key}}}
	if err := stream.Send(hello); err != nil {
		log.Fatal(err)
	}
	log.Printf("connected as %s, waiting for events", *key)

	for {
		resp, err := stream.Receive()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// For example already_exists: another client is connected with this key.
			log.Fatal(err)
		}
		ev := resp.GetEvent()
		log.Printf("event %d on %s, created %s: %s",
			ev.GetId(), ev.GetTopic(), time.UnixMilli(ev.GetCreatedMs()).Format(time.DateTime), ev.GetData())

		ack := &v1.SubscribeRequest{Payload: &v1.SubscribeRequest_Ack{Ack: &v1.Ack{EventId: ev.GetId()}}}
		if err := stream.Send(ack); err != nil {
			log.Fatal(err)
		}
	}
}

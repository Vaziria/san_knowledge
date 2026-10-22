// Mount serves San Message as one more connect-rpc service in an app's own
// server, next to the app's own routes.
//
//	go run ./examples/mount
//	curl http://127.0.0.1:7080/health
//	go run ./cmd/san_message --server http://127.0.0.1:7080 topic create orders
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/wargasipil/san_message"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:7080", "address to listen on")
	data := flag.String("data", "event_data", "data folder")
	flag.Parse()

	b, err := san_message.Open(san_message.Options{Dir: *data})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok") // the app's own route
	})
	mux.Handle(b.Handler())

	srv := &http.Server{Addr: *listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	// HTTP/2 without TLS for the Subscribe stream, and pings that drop dead
	// connections.
	san_message.ConfigureServer(srv)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		b.Close() // ends the open Subscribe streams, so Shutdown does not wait on them
		srv.Shutdown(context.Background())
	}()

	log.Printf("app with San Message on %s", *listen)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

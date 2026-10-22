// Webhook receiver prints every event a webhook subscriber gets. Its answer
// decides what San Message does next: 2xx delivered; 5xx, 408 and 429 tried
// again later; any other code skips the event. Use --status to try that.
//
//	go run ./examples/webhook_receiver
//	go run ./cmd/san_message subscriber create billing --topic orders --type webhook --webhook-uri http://127.0.0.1:8081/hook
package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8081", "address to listen on")
	status := flag.Int("status", http.StatusOK, "status code to answer with")
	flag.Parse()

	http.HandleFunc("POST /hook", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created, _ := strconv.ParseInt(r.Header.Get("X-Event-Created"), 10, 64)
		log.Printf("event %s on %s, created %s: %s (answering %d)",
			r.Header.Get("X-Event-Id"), r.Header.Get("X-Topic"),
			time.UnixMilli(created).Format(time.DateTime), body, *status)
		w.WriteHeader(*status)
	})

	log.Printf("webhook receiver on http://%s/hook", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

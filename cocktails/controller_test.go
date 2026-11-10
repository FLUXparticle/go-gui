package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Der erste Stream bricht ab. Die nachgeholte Historie liefert eine
// Bewertung aus der Lücke, der zweite Stream liefert sie nochmals.
func TestFeedNachWiederverbindung(t *testing.T) {
	var connections atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ratings/stream":
			attempt := connections.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: ready\ndata: true\n\n")
			if attempt == 1 {
				return
			}
			fmt.Fprint(w, "event: rating\ndata: {\"id\":\"b\",\"stars\":4}\n\n")
			fmt.Fprint(w, "event: rating\ndata: {\"id\":\"c\",\"stars\":5}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		case "/api/ratings":
			fmt.Fprint(w, `[{"id":"b","stars":4},{"id":"a","stars":3}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := &controller{api: &api{base: server.URL, client: *server.Client()}}
	// Wie Fyne führt der Test alle Änderungen am Feed nacheinander aus.
	var loop sync.Mutex
	dispatch := func(f func()) {
		loop.Lock()
		defer loop.Unlock()
		f()
	}
	var disconnected, recovered bool
	c.watchFeed(ctx, dispatch, func(status, historyError string, entries []rating) {
		if status != "Live verbunden" {
			disconnected = true
		}
		if disconnected && connections.Load() >= 2 && status == "Live verbunden" &&
			historyError == "" && len(entries) == 3 && entries[0].ID == "c" &&
			entries[0].Stars == 5 && entries[1].ID == "b" && entries[2].ID == "a" {
			recovered = true
			cancel()
		}
	})
	loop.Lock()
	defer loop.Unlock()
	if !recovered {
		t.Fatalf("Kein vollständiger Feed nach Wiederverbindung (%d Verbindungen)", connections.Load())
	}
}

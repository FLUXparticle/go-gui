package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestBewertungsAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/cocktails/7/ratings":
			var input ratingInput
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.Stars != 4 || input.Comment != "Gut!" {
				t.Errorf("Unerwartete Bewertung: %+v (%v)", input, err)
			}
			w.WriteHeader(http.StatusCreated)
		case "GET /api/ratings":
			fmt.Fprint(w, `[{"id":"a","cocktailId":7,"cocktailName":"Test","stars":4,"comment":"Gut!","createdAt":"2026-11-10T12:00:00Z"}]`)
		case "GET /api/ratings/stream":
			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			fmt.Fprint(w, "event: ready\r\ndata: true\r\n\r\nevent: rating\ndata: {\"id\":\"b\",\ndata: \"stars\":5,\"createdAt\":\"2026-11-10T13:00:00Z\"}\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	a := &api{base: server.URL, client: *server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.json(ctx, http.MethodPost, "/cocktails/7/ratings", ratingInput{4, "Gut!"}, nil); err != nil {
		t.Fatal(err)
	}
	var history []rating
	if err := a.json(ctx, http.MethodGet, "/ratings", nil, &history); err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].CocktailID != 7 || history[0].Stars != 4 || history[0].Comment != "Gut!" {
		t.Fatalf("Historie: %+v", history)
	}
	var events []string
	var live rating
	_ = a.stream(ctx, func(kind, data string) {
		events = append(events, kind)
		if kind == "rating" {
			if err := json.Unmarshal([]byte(data), &live); err != nil {
				t.Error(err)
			}
		}
	})
	if !reflect.DeepEqual(events, []string{"ready", "rating"}) || live.Stars != 5 {
		t.Fatalf("Stream: %v, %+v", events, live)
	}
	merged := mergeRatings(history, []rating{history[0], live})
	if len(merged) != 2 || merged[0].ID != "b" || merged[1].ID != "a" {
		t.Fatalf("Zusammenführung: %+v", merged)
	}
}

package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2/app"
)

func main() {
	base := os.Getenv("COCKTAILS_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	// Nur der Empfang der HTTP-Kopfzeilen wird begrenzt, nicht der laufende Feed.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	c := &controller{api: &api{
		base:   strings.TrimRight(base, "/"),
		client: http.Client{Transport: transport},
	}}
	c.view = newView(app.New(), c)
	c.showCocktails()
	c.view.window.ShowAndRun()
}

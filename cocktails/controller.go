package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
)

// controller steuert Navigation und Aktionen. Die Ansicht kennt keine API-Pfade;
// das Modell kennt weder Fenster noch Widgets.
type controller struct {
	api        *api
	view       *view
	cancelPage context.CancelFunc
	generation uint64
	closeFeed  func()
}

// Beim Navigieren brechen wir alte Leseanfragen ab und ignorieren bereits
// eingeplante Antworten, damit sie die neue Seite nicht überschreiben.
func loadPage[T any](c *controller, title, path string, show func(T)) {
	if c.cancelPage != nil {
		c.cancelPage()
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelPage = cancel
	c.generation++
	generation := c.generation
	c.view.loading(title)
	go func() {
		var result T
		err := c.api.json(ctx, http.MethodGet, path, nil, &result)
		fyne.Do(func() {
			if generation != c.generation {
				return
			}
			if err != nil {
				c.view.loadError(err, func() { loadPage(c, title, path, show) })
				return
			}
			show(result)
		})
	}()
}

func (c *controller) cocktails(title, path string) {
	loadPage(c, title, path, func(items []item) { c.view.links(items, c.cocktail) })
}

func (c *controller) ingredients(title, path string) {
	loadPage(c, title, path, func(items []item) { c.view.links(items, c.ingredient) })
}

func (c *controller) ingredient(id uint) {
	loadPage(c, "Zutat", fmt.Sprintf("/ingredients/%d", id), func(result ingredient) {
		c.view.title.SetText(result.Name)
		c.view.links(result.Cocktails, c.cocktail)
	})
}

func (c *controller) cocktail(id uint) {
	path := fmt.Sprintf("/cocktails/%d", id)
	loadPage(c, "Cocktail", path, func(result cocktail) {
		if result.ID == 0 {
			c.view.loadError(fmt.Errorf("Cocktail nicht gefunden."), func() { c.cocktail(id) })
			return
		}
		c.view.recipe(result, c.ingredient, func(input ratingInput, done func(error)) {
			if input.Stars < 1 || input.Stars > 5 || utf8.RuneCountInString(input.Comment) > 500 {
				done(fmt.Errorf("Bitte 1 bis 5 Sterne und höchstens 500 Zeichen eingeben."))
				return
			}
			c.write(http.MethodPost, path+"/ratings", input, done)
		})
	})
}

func (c *controller) search(query string) {
	query = strings.TrimSpace(query)
	c.view.search.SetText(query)
	if query == "" {
		if c.cancelPage != nil {
			c.cancelPage()
		}
		c.generation++
		c.view.loading("Suche")
		c.view.message("Suchbegriff eingeben und Enter drücken.")
		return
	}
	c.cocktails("Suche: "+query, "/search?"+url.Values{"query": {query}}.Encode())
}

func (c *controller) fridge() {
	loadPage(c, "Kühlschrank", "/user/fridge", func(items []stock) {
		c.view.stock(items, c.ingredient, func(entry stock, done func(error)) {
			generation := c.generation
			c.write(http.MethodPatch, fmt.Sprintf("/user/fridge/%d", entry.ID),
				struct {
					InFridge bool `json:"inFridge"`
				}{!entry.InFridge}, func(err error) {
					done(err)
					if err == nil && generation == c.generation {
						c.fridge()
					}
				})
		})
	})
}

// Schreibvorgänge laufen auch bei einem Seitenwechsel zu Ende. Die Antwort
// erreicht die Ansicht ausschließlich im Fyne-Eventloop.
func (c *controller) write(method, path string, input any, done func(error)) {
	go func() {
		err := c.api.json(context.Background(), method, path, input, nil)
		fyne.Do(func() { done(err) })
	}()
}

// mergeRatings hält höchstens 20 Einträge und entfernt Duplikate aus
// Historie und Stream. Bei gleicher Zeit entscheidet die ID wie im Web.
func mergeRatings(current, incoming []rating) []rating {
	byID := make(map[string]rating)
	for _, list := range [][]rating{current, incoming} {
		for _, entry := range list {
			byID[entry.ID] = entry
		}
	}
	result := make([]rating, 0, len(byID))
	for _, entry := range byID {
		result = append(result, entry)
	}
	slices.SortFunc(result, func(a, b rating) int {
		if order := b.CreatedAt.Compare(a.CreatedAt); order != 0 {
			return order
		}
		return strings.Compare(b.ID, a.ID)
	})
	return result[:min(20, len(result))]
}

func (c *controller) feed() {
	if c.closeFeed != nil {
		c.view.feedWindow.RequestFocus()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	display, closeWindow := c.view.openFeed(func(id uint) {
		c.cocktail(id)
		c.view.window.RequestFocus()
	}, func() { cancel(); c.closeFeed = nil })
	c.closeFeed = func() { cancel(); closeWindow() }
	go c.watchFeed(ctx, fyne.Do, display)
}

func (c *controller) watchFeed(ctx context.Context, dispatch func(func()), display func(string, string, []rating)) {
	// Der Feed-Zustand gehört dem Fyne-Thread. Historienanfragen laufen
	// parallel zum Stream, damit neue Bewertungen weiter empfangen werden.
	var entries []rating
	status, historyError := "Live-Verbindung wird aufgebaut …", ""
	update := func(change func()) {
		dispatch(func() {
			if ctx.Err() != nil {
				return
			}
			change()
			display(status, historyError, entries)
		})
	}
	history := func() {
		go func() {
			var recent []rating
			err := c.api.json(ctx, http.MethodGet, "/ratings", nil, &recent)
			update(func() {
				if err != nil {
					historyError = err.Error()
					return
				}
				historyError = ""
				entries = mergeRatings(entries, recent)
			})
		}()
	}
	for ctx.Err() == nil {
		err := c.api.stream(ctx, func(kind, data string) {
			switch kind {
			case "ready":
				update(func() { status = "Live verbunden" })
				// Erst nach „ready“ ist der Stream abonniert. Die Historie schließt
				// dann die Lücke vor dem Start beziehungsweise der Wiederverbindung.
				history()
			case "rating":
				var entry rating
				if json.Unmarshal([]byte(data), &entry) == nil && entry.ID != "" {
					update(func() { entries = mergeRatings(entries, []rating{entry}) })
				}
			}
		})
		if ctx.Err() != nil {
			return
		}
		update(func() { status = "Live-Verbindung unterbrochen: " + err.Error() })
		history()
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (c *controller) showCocktails()   { c.cocktails("Cocktails", "/cocktails") }
func (c *controller) showIngredients() { c.ingredients("Zutaten", "/ingredients") }
func (c *controller) showPossible()    { c.cocktails("Was ist möglich?", "/user/possible") }
func (c *controller) showShopping()    { c.ingredients("Einkaufsliste", "/user/shopping") }

// Beim Schließen des Hauptfensters enden beide Lesevorgänge.
func (c *controller) close() {
	c.generation++
	if c.cancelPage != nil {
		c.cancelPage()
	}
	if c.closeFeed != nil {
		c.closeFeed()
	}
}

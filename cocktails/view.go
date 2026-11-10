package main

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// view baut die Widgets und reicht Benutzeraktionen als Rückrufe weiter.
type view struct {
	app                fyne.App
	window, feedWindow fyne.Window
	title              *widget.Label
	search             *widget.Entry
	body               *fyne.Container
}

func newView(app fyne.App, c *controller) *view {
	v := &view{app: app, window: app.NewWindow("Cocktails")}
	v.title = heading("")
	v.search = widget.NewEntry()
	v.search.SetPlaceHolder("Cocktails suchen")
	v.search.OnSubmitted = c.search
	v.body = container.NewStack(widget.NewLabel(""))
	nav := container.NewVBox(
		widget.NewButton("Cocktails", c.showCocktails),
		widget.NewButton("Zutaten", c.showIngredients),
		widget.NewButton("Kühlschrank", c.fridge),
		widget.NewButton("Was ist möglich?", c.showPossible),
		widget.NewButton("Einkaufsliste", c.showShopping),
		widget.NewButton("Live-Feed", c.feed),
	)
	search := container.NewBorder(nil, nil, nil, widget.NewButton("Suchen", func() { c.search(v.search.Text) }), v.search)
	v.window.SetContent(container.NewBorder(container.NewVBox(search, v.title), nil, nav, nil, v.body))
	v.window.Resize(fyne.NewSize(760, 540))
	v.window.SetMaster()
	v.window.SetOnClosed(c.close)
	return v
}

func heading(text string) *widget.Label {
	return widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func wrapped(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

func link(text string, open func()) *widget.Button {
	button := widget.NewButton(text, open)
	button.Alignment = widget.ButtonAlignLeading
	button.Importance = widget.LowImportance
	return button
}

func (v *view) content(objects ...fyne.CanvasObject) {
	v.body.Objects = []fyne.CanvasObject{container.NewVScroll(container.NewVBox(objects...))}
	v.body.Refresh()
}

func (v *view) message(text string) { v.content(wrapped(text)) }

func (v *view) loading(title string) {
	v.title.SetText(title)
	v.message("Wird geladen …")
}

func (v *view) loadError(err error, retry func()) {
	v.content(wrapped("Laden fehlgeschlagen: "+err.Error()), widget.NewButton("Erneut versuchen", retry))
}

func (v *view) links(items []item, open func(uint)) {
	if len(items) == 0 {
		v.message("Keine Einträge.")
		return
	}
	// Die Liste erzeugt nur Widgets für sichtbare Zeilen.
	list := widget.NewList(func() int { return len(items) },
		func() fyne.CanvasObject { return link("", nil) },
		func(i widget.ListItemID, object fyne.CanvasObject) {
			entry := items[i]
			button := object.(*widget.Button)
			button.SetText(entry.Name)
			button.OnTapped = func() { open(entry.ID) }
		})
	v.body.Objects = []fyne.CanvasObject{list}
	// Eine virtuelle Liste braucht den gesamten verfügbaren Bereich.
	v.body.Refresh()
}

func (v *view) recipe(result cocktail, open func(uint), save func(ratingInput, func(error))) {
	v.title.SetText(result.Name)
	rows := []fyne.CanvasObject{heading("Rezept")}
	for _, step := range result.Instructions {
		rows = append(rows, link(fmt.Sprintf("%d cl %s", step.Amount, step.Ingredient.Name),
			func() { open(step.Ingredient.ID) }))
	}
	stars := widget.NewSelect([]string{"1", "2", "3", "4", "5"}, nil)
	comment := widget.NewMultiLineEntry()
	comment.SetPlaceHolder("Optional, höchstens 500 Zeichen")
	status := wrapped("")
	submit := widget.NewButton("Bewertung speichern", nil)
	submit.Disable()
	stars.OnChanged = func(value string) {
		if value == "" {
			submit.Disable()
		} else {
			submit.Enable()
		}
	}
	submit.OnTapped = func() {
		count, _ := strconv.Atoi(stars.Selected)
		submit.Disable()
		stars.Disable()
		comment.Disable()
		status.SetText("Wird gespeichert …")
		save(ratingInput{Stars: count, Comment: comment.Text}, func(err error) {
			stars.Enable()
			comment.Enable()
			if err != nil {
				status.SetText(err.Error())
				submit.Enable()
				return
			}
			stars.ClearSelected()
			comment.SetText("")
			status.SetText("Bewertung gespeichert.")
		})
	}
	v.content(append(rows, widget.NewSeparator(), heading("Cocktail bewerten"), widget.NewForm(
		widget.NewFormItem("Sterne", stars),
		widget.NewFormItem("Kommentar", comment),
	), submit, status)...)
}

func (v *view) stock(items []stock, open func(uint), toggle func(stock, func(error))) {
	status := wrapped("")
	rows := []fyne.CanvasObject{status}
	var buttons []*widget.Button
	for _, inFridge := range []bool{true, false} {
		title, action := "Nicht im Kühlschrank", "+"
		if inFridge {
			title, action = "Im Kühlschrank", "−"
		}
		rows = append(rows, heading(title))
		for _, entry := range items {
			if entry.InFridge != inFridge {
				continue
			}
			button := widget.NewButton(action, func() {
				for _, b := range buttons {
					b.Disable()
				}
				status.SetText("Wird gespeichert …")
				toggle(entry, func(err error) {
					if err != nil {
						status.SetText(err.Error())
						for _, b := range buttons {
							b.Enable()
						}
					}
				})
			})
			buttons = append(buttons, button)
			rows = append(rows, container.NewBorder(nil, nil, button, nil, link(entry.Name, func() { open(entry.ID) })))
		}
	}
	v.content(rows...)
}

// Ein eigenes Fenster hält den Feed sichtbar, während im Hauptfenster
// Cocktails geöffnet und bewertet werden.
func (v *view) openFeed(open func(uint), closed func()) (func(string, string, []rating), func()) {
	window := v.app.NewWindow("Live-Feed für Cocktailbewertungen")
	v.feedWindow = window
	status := wrapped("Live-Verbindung wird aufgebaut …")
	historyError := wrapped("")
	rows := container.NewVBox()
	window.SetContent(container.NewBorder(container.NewVBox(status, historyError), nil, nil, nil, container.NewVScroll(rows)))
	window.Resize(fyne.NewSize(540, 460))
	window.SetOnClosed(closed)
	window.Show()
	return func(connection, history string, entries []rating) {
		status.SetText(connection)
		historyError.SetText(history)
		rows.RemoveAll()
		if len(entries) == 0 && history == "" {
			rows.Add(widget.NewLabel("Noch keine Bewertungen."))
		}
		for _, entry := range entries {
			rows.Add(link(entry.CocktailName, func() { open(entry.CocktailID) }))
			rows.Add(widget.NewLabel(fmt.Sprintf("%d von 5 Sternen · %s", entry.Stars, entry.CreatedAt.Local().Format("02.01.2006 15:04:05"))))
			if entry.Comment != "" {
				rows.Add(wrapped(entry.Comment))
			}
			rows.Add(widget.NewSeparator())
		}
	}, window.Close
}

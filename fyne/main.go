package main

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"
)

func main() {
	window := app.New().NewWindow("Fyne")
	input := widget.NewEntry()
	output := widget.NewEntry()
	input.OnSubmitted = output.SetText
	window.SetContent(widget.NewForm(
		widget.NewFormItem("Eingabe:", input),
		widget.NewFormItem("Ausgabe:", output),
	))
	window.ShowAndRun()
}

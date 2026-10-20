package main

import (
	"os"

	qt "github.com/mappu/miqt/qt6"
)

func main() {
	qt.NewQApplication(os.Args)
	window := qt.NewQWidget(nil)
	layout := qt.NewQFormLayout(window)
	input := qt.NewQLineEdit2()
	output := qt.NewQLineEdit2()
	layout.AddRow3("Eingabe:", input.QWidget)
	layout.AddRow3("Ausgabe:", output.QWidget)
	input.OnReturnPressed(func() {
		output.SetText(input.Text())
	})
	window.Show()
	qt.QApplication_Exec()
}

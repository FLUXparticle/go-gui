package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed index.html
var assets embed.FS

func onStartup(ctx context.Context) {
	runtime.EventsOn(ctx, "input:enter", func(data ...any) {
		if len(data) != 1 {
			return
		}
		if input, ok := data[0].(string); ok {
			text, _ := json.Marshal(input)
			runtime.WindowExecJS(ctx, `document.getElementById("output").value = `+string(text))
		}
	})
}

func main() {
	if err := wails.Run(&options.App{
		Title:       "Wails",
		Width:       320,
		Height:      140,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   onStartup,
	}); err != nil {
		log.Fatal(err)
	}
}

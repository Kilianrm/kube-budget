package main

import (
	"embed"
	"log"

	manifestwails "kube-budget/internal/adapters/wails"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	err := wails.Run(&options.App{
		Title:     "KubeBudget",
		Width:     1440,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 1},
		Bind: []interface{}{
			manifestwails.NewManifestAdapter(),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

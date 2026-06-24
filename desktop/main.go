package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	elfApp := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Elf",
		Description: "Hardware Agent Voice Assistant",
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyRegular,
		},
		Services: []application.Service{
			application.NewService(elfApp),
		},
	})

	// Main window
	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:    "Elf",
		Width:    420,
		Height:   620,
		URL:      "/",
		MinWidth: 360,
		MinHeight: 480,
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 40,
		},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

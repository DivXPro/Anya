package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Elf",
		Description: "Hardware Agent Voice Assistant",
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Services: []application.Service{
			application.NewService(app),
		},
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

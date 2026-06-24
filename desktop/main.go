package main

import (
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
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
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Services: []application.Service{
			application.NewService(elfApp),
		},
	})

	// Main window — hidden initially, shown via menu bar icon
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:         "Elf",
		Width:         420,
		Height:        620,
		URL:           "/",
		MinWidth:      360,
		MinHeight:     480,
		Hidden:        true,
		HideOnEscape:  true,
		HideOnFocusLost: false,
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 40,
		},
	})

	// Close button hides instead of quitting
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mainWindow.Hide()
		e.Cancel()
	})

	// ── Menu bar icon ──
	systemTray := wailsApp.SystemTray.New()
	systemTray.SetTooltip("Elf")

	if runtime.GOOS == "darwin" {
		systemTray.SetTemplateIcon(icons.SystrayMacTemplate)
	}

	// Right-click menu
	menu := wailsApp.NewMenu()
	menu.Add("Show Elf").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.Add("Hide Elf").OnClick(func(_ *application.Context) {
		mainWindow.Hide()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		wailsApp.Quit()
	})
	systemTray.SetMenu(menu)

	// Left-click tray icon → toggle window
	systemTray.AttachWindow(mainWindow).WindowOffset(5)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

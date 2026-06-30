package main

import (
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed frontend/dist
var assets embed.FS

//go:embed frontend/public/anya-app.png
var appIcon []byte

//go:embed frontend/public/anya-tray.png
var trayIcon []byte

func main() {
	elfApp := NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Anya",
		Description: "Hardware Agent Voice Assistant",
		Icon:        appIcon,
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		Services: []application.Service{
			application.NewService(elfApp),
		},
	})

	if runtime.GOOS == "darwin" {
		setupMacMenuBar(wailsApp, elfApp)
	}

	// Main window — hidden initially, opened via menu
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Anya",
		Width:           960,
		Height:          680,
		URL:             "/",
		MinWidth:        820,
		MinHeight:       520,
		Hidden:          true,
		HideOnEscape:    true,
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
	systemTray.SetTooltip("Anya")

	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		systemTray.SetIcon(trayIcon)
	}

	// Tray menu
	menu := wailsApp.NewMenu()
	deviceItem := menu.Add("")
	deviceItem.SetEnabled(false)
	menu.AddSeparator()

	// Working directory item
	cwdItem := menu.Add("")
	cwdItem.OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
		wailsApp.Event.Emit("navigate-to-working-directory", nil)
	})
	menu.AddSeparator()

	// Agent submenu: shows available agents and lets the user pick the active one.
	agentMenu := menu.AddSubmenu("Agent")
	menu.AddSeparator()

	openItem := menu.Add("")
	openItem.OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.AddSeparator()
	quitItem := menu.Add("")
	quitItem.OnClick(func(_ *application.Context) {
		mainWindow.Close()
		wailsApp.Quit()
	})

	systemTray.SetMenu(menu)

	// Give App access to update tray menus
	elfApp.SetTrayDeviceItem(deviceItem)
	elfApp.SetTrayCWDItem(cwdItem)
	elfApp.SetTrayAgentMenu(agentMenu)
	elfApp.SetTrayOpenItem(openItem)
	elfApp.SetTrayQuitItem(quitItem)

	// Left-click → show menu
	systemTray.OnClick(func() {
		systemTray.OpenMenu()
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

func setupMacMenuBar(wailsApp *application.App, elfApp *App) {
	menu := application.DefaultApplicationMenu()

	// App menu: customize About to show version
	if aboutItem := menu.FindByRole(application.About); aboutItem != nil {
		aboutItem.OnClick(func(_ *application.Context) {
			wailsApp.Menu.ShowAbout()
		})
	}

	wailsApp.Menu.SetApplicationMenu(menu)
}

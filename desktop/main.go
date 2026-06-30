package main

import (
	"embed"
	"log"
	"os"
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

	// A post-update relaunch passes --updated so we can surface the window on
	// startup, instead of leaving the freshly-updated app hidden in the tray.
	updatedRelaunch := false
	for _, arg := range os.Args[1:] {
		if arg == "--updated" {
			updatedRelaunch = true
			break
		}
	}

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

	// macOS: the app runs as a tray-only accessory (no Dock icon / menu bar).
	// Promote it to a regular app while the window is visible so the standard
	// menu bar shows, and demote it back when the window hides. Registering
	// these hooks is also what makes the native WindowShow/WindowHide events
	// fire. Covers every path: tray show, close button, and Escape.
	mainWindow.RegisterHook(events.Common.WindowShow, func(_ *application.WindowEvent) {
		setMacActivationRegular()
	})
	mainWindow.RegisterHook(events.Common.WindowHide, func(_ *application.WindowEvent) {
		setMacActivationAccessory()
	})

	// After a self-update relaunch, show the window once the app has started so
	// the user sees the updated app come back rather than only a tray icon.
	if updatedRelaunch {
		wailsApp.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(_ *application.ApplicationEvent) {
			mainWindow.Show()
			mainWindow.Focus()
		})
	}

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
	menu := application.NewMenu()

	// App menu (the first menu macOS shows under the bold app name). Built
	// manually so "Check for Updates" can sit right under About, instead of
	// being appended after Quit like a plain DefaultApplicationMenu().Add would.
	appMenu := menu.AddSubmenu("Anya")
	appMenu.AddRole(application.About)
	checkUpdate := appMenu.Add(elfApp.trayText("checkUpdate"))
	checkUpdate.OnClick(func(_ *application.Context) {
		go elfApp.CheckForUpdateInteractive()
	})
	appMenu.AddSeparator()
	appMenu.AddRole(application.ServicesMenu)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Hide)
	appMenu.AddRole(application.HideOthers)
	appMenu.AddRole(application.UnHide)
	appMenu.AddSeparator()
	appMenu.AddRole(application.Quit)

	// Standard remaining menus.
	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.ViewMenu)
	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)

	// Customize About to show the native about panel.
	if aboutItem := menu.FindByRole(application.About); aboutItem != nil {
		aboutItem.OnClick(func(_ *application.Context) {
			wailsApp.Menu.ShowAbout()
		})
	}

	// Let the app keep the menu item's label in sync with the UI language.
	elfApp.SetMenuCheckUpdateItem(checkUpdate)

	wailsApp.Menu.SetApplicationMenu(menu)
}

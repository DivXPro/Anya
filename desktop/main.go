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

	// Main window — hidden initially, opened via menu
	mainWindow := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Elf",
		Width:           420,
		Height:          620,
		URL:             "/",
		MinWidth:        360,
		MinHeight:       480,
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
	systemTray.SetTooltip("Elf")

	if runtime.GOOS == "darwin" {
		systemTray.SetTemplateIcon(icons.SystrayMacTemplate)
	}

	// Tray menu
	menu := wailsApp.NewMenu()
	deviceItem := menu.Add("未连接设备")
	deviceItem.SetEnabled(false)
	menu.AddSeparator()

	// Agent submenu: shows available agents and lets the user pick the active one.
	agentMenu := menu.AddSubmenu("Agent")
	menu.AddSeparator()

	menu.Add("打开 Elf").OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	menu.AddSeparator()
	menu.Add("退出").OnClick(func(_ *application.Context) {
		mainWindow.Close()
		wailsApp.Quit()
	})

	systemTray.SetMenu(menu)

	// Give App access to update tray menus
	elfApp.SetTrayDeviceItem(deviceItem)
	elfApp.SetTrayAgentMenu(agentMenu)

	// Left-click → show menu
	systemTray.OnClick(func() {
		systemTray.OpenMenu()
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

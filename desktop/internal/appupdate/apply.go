package appupdate

// Applier installs a verified update asset and relaunches the app. Implemented
// per-OS in apply_darwin.go / apply_windows.go.
type Applier interface {
	// Apply installs the verified asset (downloaded to assetPath) over the
	// running application, atomically where possible.
	Apply(assetPath string) error
	// Relaunch starts the updated app and signals the current process to exit.
	Relaunch() error
}

// Emitter pushes events to the frontend. Backed by application.App.Event.Emit in
// production (wired in app.go), a fake in tests. Defined here to keep appupdate
// free of any Wails dependency.
type Emitter interface {
	Emit(name string, data any)
}

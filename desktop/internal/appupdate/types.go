package appupdate

// State is the updater state-machine value mirrored to the frontend.
type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateUpToDate    State = "up_to_date"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateVerifying   State = "verifying"
	StateApplying    State = "applying"
	StateError       State = "error"
)

// Wails event names; the frontend subscribes to these.
const (
	EventAvailable = "update:available"
	EventProgress  = "update:progress"
	EventApplying  = "update:applying"
	EventError     = "update:error"
)

// UpdateInfo describes an available release for the current platform.
type UpdateInfo struct {
	Version      string `json:"version"`
	Notes        string `json:"notes"`
	AssetName    string `json:"assetName"`
	AssetURL     string `json:"assetURL"`
	Size         int64  `json:"size"`
	ChecksumsURL string `json:"checksumsURL"`
	SignatureURL string `json:"signatureURL"`
}

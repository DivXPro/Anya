package firmware

// SerialPortInfo describes a candidate USB/serial port for flashing.
type SerialPortInfo struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// FlashStage represents the current phase of a firmware flash operation.
type FlashStage string

const (
	StageIdle       FlashStage = "idle"
	StageDetecting  FlashStage = "detecting"
	StageErasing    FlashStage = "erasing"
	StageWriting    FlashStage = "writing"
	StageVerifying  FlashStage = "verifying"
	StageDone       FlashStage = "done"
	StageCancelled  FlashStage = "cancelled"
	StageError      FlashStage = "error"
)

// FlashProgress is the public progress snapshot exposed to the frontend.
type FlashProgress struct {
	Running bool       `json:"running"`
	Stage   FlashStage `json:"stage"`
	Percent int        `json:"percent"`
	Message string     `json:"message"`
	Error   string     `json:"error"`
}

// OTAStage represents the current phase of an over-the-air firmware update.
type OTAStage string

const (
	OTAStageIdle       OTAStage = "idle"
	OTAStageChecking   OTAStage = "checking"
	OTAStageAwaiting   OTAStage = "awaiting"
	OTAStageWriting    OTAStage = "writing"
	OTAStageVerifying  OTAStage = "verifying"
	OTAStageCommitting OTAStage = "committing"
	OTAStageDone       OTAStage = "done"
	OTAStageCancelled  OTAStage = "cancelled"
	OTAStageError      OTAStage = "error"
)

// OTAProgress is the public progress snapshot for a wireless firmware update.
type OTAProgress struct {
	Running       bool   `json:"running"`
	Stage         string `json:"stage"`
	Percent       int    `json:"percent"`
	Message       string `json:"message"`
	Error         string `json:"error"`
	DeviceVersion string `json:"device_version"`
}


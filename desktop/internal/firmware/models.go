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



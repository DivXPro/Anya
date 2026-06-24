package gateway

type DeviceInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Capabilities []string `json:"capabilities"`
}

type DeviceAdapter interface {
	Info() DeviceInfo
	SetDeviceID(id string)
	SendText(msg DeviceMessage) error
	SendBinary(data []byte) error
	ReceiveEvent() (<-chan DeviceEvent, error)
	ReceiveBinary() (<-chan []byte, error)
	OnDisconnect() <-chan struct{}
	Close() error
}

// NewStickCS3Adapter creates a StickC S3 adapter over an established WebSocket connection.
// Signature lives in the adapters package; the gateway imports and calls it directly.

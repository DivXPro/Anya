package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

type DiscoveredDevice struct {
	DeviceID string
	Name     string
	IP       string
	Port     int
}

func ScanDevices() ([]DiscoveredDevice, error) {
	entries := make(chan *mdns.ServiceEntry, 32)
	var devices []DiscoveredDevice

	params := mdns.DefaultParams("_elf-device._tcp")
	params.Entries = entries
	params.Timeout = 3 * time.Second

	go func() {
		for entry := range entries {
			d := DiscoveredDevice{
				IP:   entry.AddrV4.String(),
				Port: entry.Port,
			}
			for _, field := range entry.InfoFields {
				if strings.HasPrefix(field, "device_id=") {
					d.DeviceID = strings.TrimPrefix(field, "device_id=")
				} else if strings.HasPrefix(field, "name=") {
					d.Name = strings.TrimPrefix(field, "name=")
				}
			}
			if d.DeviceID != "" {
				devices = append(devices, d)
			}
		}
	}()

	if err := mdns.Query(params); err != nil {
		return nil, fmt.Errorf("mDNS scan: %w", err)
	}
	return devices, nil
}

func LocalIP() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "127.0.0.1"
}

func NotifyDevice(deviceIP string, devicePort int, desktopIP string, desktopPort int, desktopID string, pairingToken string) error {
	body := map[string]interface{}{
		"desktop_ip":    desktopIP,
		"desktop_port":  desktopPort,
		"desktop_id":    desktopID,
		"pairing_token": pairingToken,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s:%d/connect", deviceIP, devicePort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

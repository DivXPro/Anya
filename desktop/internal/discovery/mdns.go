package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	serviceName  = "_elf-device._tcp"
	queryTimeout = 3 * time.Second
)

// DiscoveredDevice is a device discovered via mDNS.
type DiscoveredDevice struct {
	DeviceID string
	Name     string
	IP       string
	Port     int
}

// ScanDevices runs an mDNS query for _elf-device._tcp on every usable local
// network interface and returns the merged, de-duplicated list of discovered
// devices.
//
// The hashicorp/mdns client uses the system default multicast interface when no
// interface is specified. On machines with multiple interfaces (Wi-Fi +
// Ethernet, VPN tunnels, VM networks, etc.) that default can be the wrong
// interface, causing devices on the same LAN to be invisible. Scanning each
// interface explicitly avoids that problem.
func ScanDevices() ([]DiscoveredDevice, error) {
	ifaces, err := interfacesToScan()
	if err != nil {
		return nil, fmt.Errorf("list interfaces: %w", err)
	}
	if len(ifaces) == 0 {
		log.Printf("[discovery] no suitable network interface found for mDNS scan")
		return nil, nil
	}

	log.Printf("[discovery] scanning for %s on %d interface(s)", serviceName, len(ifaces))

	entries := make(chan *mdns.ServiceEntry, 32)
	var (
		devices []DiscoveredDevice
		seen    = make(map[string]bool)
		mu      sync.Mutex
	)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			d := serviceEntryToDevice(entry)
			if d.DeviceID == "" {
				continue
			}
			key := fmt.Sprintf("%s/%s:%d", d.DeviceID, d.IP, d.Port)
			mu.Lock()
			if !seen[key] {
				seen[key] = true
				devices = append(devices, d)
				log.Printf("[discovery] found device %s (%s) at %s:%d", d.Name, d.DeviceID, d.IP, d.Port)
			}
			mu.Unlock()
		}
	}()

	var wg sync.WaitGroup
	for i := range ifaces {
		iface := &ifaces[i]
		wg.Add(1)
		go func(ifc *net.Interface) {
			defer wg.Done()
			params := mdns.DefaultParams(serviceName)
			params.Interface = ifc
			params.Entries = entries
			params.Timeout = queryTimeout
			if err := mdns.Query(params); err != nil {
				log.Printf("[discovery] mDNS query on %s failed: %v", ifc.Name, err)
			}
		}(iface)
	}

	wg.Wait()
	close(entries)
	<-done

	log.Printf("[discovery] scan complete: %d device(s) found", len(devices))
	return devices, nil
}

func serviceEntryToDevice(entry *mdns.ServiceEntry) DiscoveredDevice {
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
	return d
}

// interfacesToScan returns local network interfaces that can be used for mDNS
// queries: they are up, not loopback, support multicast, and have at least one
// IPv4 address assigned.
func interfacesToScan() ([]net.Interface, error) {
	all, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []net.Interface
	for _, iface := range all {
		if !isUsableInterface(iface) {
			continue
		}
		if !hasIPv4Address(iface) {
			continue
		}
		out = append(out, iface)
	}
	return out, nil
}

func isUsableInterface(iface net.Interface) bool {
	if iface.Flags&net.FlagUp == 0 {
		return false
	}
	if iface.Flags&net.FlagLoopback != 0 {
		return false
	}
	// mDNS is a multicast protocol; without multicast support we cannot send
	// queries or receive responses on this interface.
	if iface.Flags&net.FlagMulticast == 0 {
		return false
	}
	return true
}

func hasIPv4Address(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			if ip4 := v.IP.To4(); ip4 != nil && !ip4.IsLoopback() {
				return true
			}
		case *net.IPAddr:
			if ip4 := v.IP.To4(); ip4 != nil && !ip4.IsLoopback() {
				return true
			}
		}
	}
	return false
}

// LocalIP returns a local IPv4 address that the device can use to connect back
// to this desktop. It prefers interfaces that look like physical LAN/Wi-Fi and
// falls back to any non-loopback IPv4 interface if necessary.
func LocalIP() string {
	ip, _ := bestLocalIP(nil)
	return ip
}

// LocalIPFor returns a local IPv4 address that is on the same subnet as the
// given target device IP. This is useful when the machine has multiple
// interfaces and we want to make sure the device connects to the correct one.
func LocalIPFor(target net.IP) string {
	ip, _ := bestLocalIP(target)
	return ip
}

func bestLocalIP(target net.IP) (string, *net.Interface) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1", nil
	}

	type candidate struct {
		iface net.Interface
		ip    net.IP
		ipnet *net.IPNet
	}

	var candidates []candidate
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if !looksLikePhysicalInterface(iface.Name) {
			continue
		}
		adds, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range adds {
			var ip net.IP
			var ipnet *net.IPNet
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
				ipnet = v
			case *net.IPAddr:
				ip = v.IP
				ipnet = &net.IPNet{IP: v.IP, Mask: net.CIDRMask(32, 32)}
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				candidates = append(candidates, candidate{iface, ip4, ipnet})
			}
		}
	}

	// Prefer an address on the same subnet as the target device.
	if target != nil {
		target4 := target.To4()
		if target4 != nil {
			for _, c := range candidates {
				if c.ipnet != nil && c.ipnet.Contains(target4) {
					return c.ip.String(), &c.iface
				}
			}
		}
	}

	if len(candidates) > 0 {
		return candidates[0].ip.String(), &candidates[0].iface
	}

	// Fallback: take any non-loopback IPv4 address.
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		adds, _ := iface.Addrs()
		for _, addr := range adds {
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
				return ip4.String(), &iface
			}
		}
	}

	return "127.0.0.1", nil
}

// looksLikePhysicalInterface excludes common virtual/tunnel/VM network
// interfaces so that we do not announce a VPN or VM address to the device.
func looksLikePhysicalInterface(name string) bool {
	lower := strings.ToLower(name)
	prefixes := []string{
		"lo",        // loopback (already filtered, but keep for safety)
		"bridge",    // bridge interfaces
		"vmnet",     // VMware
		"vboxnet",   // VirtualBox
		"veth",      // Linux virtual ethernet
		"vethernet", // Hyper-V / WSL virtual ethernet on Windows
		"docker",    // Docker
		"utun",      // VPN / tunnel on macOS
		"awdl",      // Apple Wireless Direct Link
		"llw",       // Low-latency WLAN
		"gif",       // generic tunnel interface
		"stf",       // 6to4 tunnel
		"tun",       // tunnel
		"tap",       // tap tunnel
		"ppp",       // point-to-point protocol
		"pptp",      // PPTP VPN
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return false
		}
	}
	return true
}

// NotifyDevice tells the discovered device where the desktop gateway is
// listening and supplies a one-time pairing token.
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
	log.Printf("[discovery] POST %s body=%s", url, string(data))

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // ESP32 WebServer closes the connection; avoid "connection reset by peer"

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("connect endpoint returned %s", resp.Status)
	}
	return nil
}

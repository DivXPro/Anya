package discovery

import (
	"net"
	"testing"

	"github.com/hashicorp/mdns"
)

func TestLooksLikePhysicalInterface(t *testing.T) {
	tests := []struct {
		name     string
		ifName   string
		physical bool
	}{
		{"ethernet en0", "en0", true},
		{"ethernet en1", "en1", true},
		{"wifi wlan0", "wlan0", true},
		{"loopback", "lo0", false},
		{"bridge", "bridge0", false},
		{"vmware", "vmnet1", false},
		{"virtualbox", "vboxnet0", false},
		{"linux veth", "veth1234", false},
		{"hyper-v vethernet", "vEthernet (Default Switch)", false},
		{"docker", "docker0", false},
		{"vpn utun", "utun2", false},
		{"awdl", "awdl0", false},
		{"tun", "tun0", false},
		{"tap", "tap0", false},
		{"ppp", "ppp0", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := looksLikePhysicalInterface(tc.ifName)
			if got != tc.physical {
				t.Errorf("looksLikePhysicalInterface(%q) = %v, want %v", tc.ifName, got, tc.physical)
			}
		})
	}
}

func TestIsUsableInterface(t *testing.T) {
	tests := []struct {
		name    string
		iface   net.Interface
		usable  bool
	}{
		{
			name:   "up multicast",
			iface:  net.Interface{Flags: net.FlagUp | net.FlagMulticast},
			usable: true,
		},
		{
			name:   "down",
			iface:  net.Interface{Flags: net.FlagMulticast},
			usable: false,
		},
		{
			name:   "loopback",
			iface:  net.Interface{Flags: net.FlagUp | net.FlagLoopback | net.FlagMulticast},
			usable: false,
		},
		{
			name:   "no multicast",
			iface:  net.Interface{Flags: net.FlagUp},
			usable: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isUsableInterface(tc.iface)
			if got != tc.usable {
				t.Errorf("isUsableInterface() = %v, want %v", got, tc.usable)
			}
		})
	}
}

func TestServiceEntryToDevice(t *testing.T) {
	entry := &mdns.ServiceEntry{
		AddrV4: net.ParseIP("192.168.1.42"),
		Port:   80,
		InfoFields: []string{
			"device_id=112233445566-1234",
			"name=elf-5566",
		},
	}

	got := serviceEntryToDevice(entry)
	want := DiscoveredDevice{
		DeviceID: "112233445566-1234",
		Name:     "elf-5566",
		IP:       "192.168.1.42",
		Port:     80,
	}

	if got != want {
		t.Errorf("serviceEntryToDevice() = %+v, want %+v", got, want)
	}
}

func TestServiceEntryToDeviceMissingDeviceID(t *testing.T) {
	entry := &mdns.ServiceEntry{
		AddrV4: net.ParseIP("192.168.1.42"),
		Port:   80,
		InfoFields: []string{
			"name=elf-5566",
		},
	}

	got := serviceEntryToDevice(entry)
	if got.DeviceID != "" {
		t.Errorf("expected empty DeviceID, got %q", got.DeviceID)
	}
}

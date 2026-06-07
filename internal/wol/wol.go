package wol

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

// Device represents a WOL-capable device.
type Device struct {
	Name         string `json:"name"`
	MACAddress   string `json:"mac_address"`
	IPAddress    string `json:"ip_address,omitempty"`
	Group        string `json:"group,omitempty"`
	Description  string `json:"description,omitempty"`
	LastWakeTime string `json:"last_wake_time,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// Manager manages WOL devices.
type Manager struct {
	mu      sync.RWMutex
	devices map[string]*Device
}

// NewManager creates a new WOL manager.
func NewManager() *Manager {
	return &Manager{
		devices: make(map[string]*Device),
	}
}

// AddDevice adds a device to the WOL list.
func (m *Manager) AddDevice(dev Device) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mac := strings.ToUpper(strings.TrimSpace(dev.MACAddress))
	if _, err := net.ParseMAC(mac); err != nil {
		return fmt.Errorf("invalid MAC address: %s", dev.MACAddress)
	}
	dev.MACAddress = mac
	dev.Enabled = true
	m.devices[mac] = &dev
	log.Printf("WOL设备已添加: %s (%s)", dev.Name, mac)
	return nil
}

// RemoveDevice removes a device from the WOL list.
func (m *Manager) RemoveDevice(mac string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, strings.ToUpper(strings.TrimSpace(mac)))
}

// ListDevices returns all WOL devices.
func (m *Manager) ListDevices() []Device {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Device, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, *d)
	}
	return result
}

// Wake sends a WOL magic packet to a device.
func (m *Manager) Wake(mac string) error {
	m.mu.RLock()
	dev, ok := m.devices[strings.ToUpper(strings.TrimSpace(mac))]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("device not found: %s", mac)
	}
	if !dev.Enabled {
		return fmt.Errorf("device disabled: %s", dev.Name)
	}

	if err := SendMagicPacket(dev.MACAddress); err != nil {
		return fmt.Errorf("failed to send magic packet: %w", err)
	}

	m.mu.Lock()
	dev.LastWakeTime = fmt.Sprintf("%d", timeNow().Unix())
	m.mu.Unlock()

	log.Printf("✅ WOL唤醒包已发送: %s (%s)", dev.Name, dev.MACAddress)
	return nil
}

// WakeGroup sends WOL to all devices in a group.
func (m *Manager) WakeGroup(group string) []error {
	m.mu.RLock()
	var targets []*Device
	for _, d := range m.devices {
		if d.Group == group && d.Enabled {
			targets = append(targets, d)
		}
	}
	m.mu.RUnlock()

	var errs []error
	for _, d := range targets {
		if err := SendMagicPacket(d.MACAddress); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.Name, err))
		}
	}
	return errs
}

// SendMagicPacket sends a WOL magic packet to the specified MAC address.
func SendMagicPacket(macStr string) error {
	mac, err := net.ParseMAC(macStr)
	if err != nil {
		return err
	}
	if len(mac) != 6 {
		return fmt.Errorf("invalid MAC length: %d", len(mac))
	}

	// Build magic packet: 6 bytes of 0xFF + 16 repetitions of MAC
	packet := make([]byte, 102)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}

	// Send via UDP broadcast
	addr := &net.UDPAddr{
		IP:   net.IPv4bcast,
		Port: 9,
	}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("dial UDP: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("send packet: %w", err)
	}

	// Decode for log
	macHex := hex.EncodeToString(mac)
	log.Printf("Magic packet sent to %s", macHex)
	return nil
}

func timeNow() time.Time {
	return time.Now()
}

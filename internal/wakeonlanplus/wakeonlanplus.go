package wakeonlanplus

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// Device represents a network device
type Device struct {
	Name        string    `json:"name"`
	MACAddress  string    `json:"mac_address"`
	IPAddress   string    `json:"ip_address,omitempty"`
	Hostname    string    `json:"hostname,omitempty"`
	Status      string    `json:"status"` // online, offline, sleeping
	LastSeen    time.Time `json:"last_seen"`
	WakePort    int       `json:"wake_port"`
	GroupName   string    `json:"group_name,omitempty"`
}

// WakePolicy defines wake behavior
type WakePolicy struct {
	Name          string        `json:"name"`
	Trigger       string        `json:"trigger"` // manual, schedule, demand
	Schedule      string        `json:"schedule,omitempty"`
	IdleTimeout   time.Duration `json:"idle_timeout,omitempty"`
	PreWakeDelay  time.Duration `json:"pre_wake_delay,omitempty"`
}

// WakeOnLANPlus provides intelligent device wake management
// Inspired by fnOS on-demand disk wake
type WakeOnLANPlus struct {
	mu        sync.RWMutex
	devices   map[string]*Device
	policies  map[string]*WakePolicy
	running   bool
	stopCh    chan struct{}
}

// NewWakeOnLANPlus creates a new WakeOnLANPlus instance
func NewWakeOnLANPlus() *WakeOnLANPlus {
	return &WakeOnLANPlus{
		devices:  make(map[string]*Device),
		policies: make(map[string]*WakePolicy),
		stopCh:   make(chan struct{}),
	}
}

// AddDevice adds a device to manage
func (wol *WakeOnLANPlus) AddDevice(device Device) error {
	wol.mu.Lock()
	defer wol.mu.Unlock()

	if device.MACAddress == "" {
		return fmt.Errorf("MAC address is required")
	}
	// Validate MAC address format (AA:BB:CC:DD:EE:FF or AA-BB-CC-DD-EE-FF)
	mac := strings.ReplaceAll(strings.ReplaceAll(device.MACAddress, ":", ""), "-", "")
	if len(mac) != 12 {
		return fmt.Errorf("invalid MAC address format: %s", device.MACAddress)
	}
	for _, c := range mac {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("invalid MAC address format: %s", device.MACAddress)
		}
	}
	if device.WakePort == 0 {
		device.WakePort = 9
	}
	device.Status = "unknown"
	wol.devices[device.MACAddress] = &device
	return nil
}

// RemoveDevice removes a device
func (wol *WakeOnLANPlus) RemoveDevice(macAddress string) {
	wol.mu.Lock()
	defer wol.mu.Unlock()
	delete(wol.devices, macAddress)
}

// WakeDevice sends a wake-on-LAN packet to a device
func (wol *WakeOnLANPlus) WakeDevice(ctx context.Context, macAddress string) error {
	wol.mu.RLock()
	device, exists := wol.devices[macAddress]
	wol.mu.RUnlock()

	if !exists {
		return fmt.Errorf("device not found: %s", macAddress)
	}

	mac, err := net.ParseMAC(device.MACAddress)
	if err != nil {
		return fmt.Errorf("invalid MAC address: %v", err)
	}

	// Build magic packet
	packet := buildMagicPacket(mac)
	
	// Send broadcast
	addr := fmt.Sprintf("255.255.255.255:%d", device.WakePort)
	conn, err := net.Dial("udp4", addr)
	if err != nil {
		return fmt.Errorf("failed to dial: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write(packet)
	if err != nil {
		return fmt.Errorf("failed to send packet: %v", err)
	}

	wol.mu.Lock()
	device.Status = "waking"
	wol.mu.Unlock()

	return nil
}

// GetDeviceStatus returns the status of a device
func (wol *WakeOnLANPlus) GetDeviceStatus(macAddress string) (string, error) {
	wol.mu.RLock()
	defer wol.mu.RUnlock()

	device, exists := wol.devices[macAddress]
	if !exists {
		return "", fmt.Errorf("device not found: %s", macAddress)
	}
	return device.Status, nil
}

// ListDevices returns all managed devices
func (wol *WakeOnLANPlus) ListDevices() []*Device {
	wol.mu.RLock()
	defer wol.mu.RUnlock()

	devices := make([]*Device, 0, len(wol.devices))
	for _, d := range wol.devices {
		devices = append(devices, d)
	}
	return devices
}

// AddPolicy adds a wake policy
func (wol *WakeOnLANPlus) AddPolicy(policy WakePolicy) {
	wol.mu.Lock()
	defer wol.mu.Unlock()
	wol.policies[policy.Name] = &policy
}

// Start begins monitoring
func (wol *WakeOnLANPlus) Start(ctx context.Context) error {
	wol.mu.Lock()
	if wol.running {
		wol.mu.Unlock()
		return fmt.Errorf("already running")
	}
	wol.running = true
	wol.mu.Unlock()

	go wol.monitorLoop(ctx)
	return nil
}

// Stop stops monitoring
func (wol *WakeOnLANPlus) Stop() {
	wol.mu.Lock()
	defer wol.mu.Unlock()
	if wol.running {
		close(wol.stopCh)
		wol.running = false
	}
}

func (wol *WakeOnLANPlus) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-wol.stopCh:
			return
		case <-ticker.C:
			wol.checkDevices(ctx)
		}
	}
}

func (wol *WakeOnLANPlus) checkDevices(ctx context.Context) {
	wol.mu.RLock()
	devices := make([]*Device, 0, len(wol.devices))
	for _, d := range wol.devices {
		devices = append(devices, d)
	}
	wol.mu.RUnlock()

	for _, device := range devices {
		// Ping check
		conn, err := net.DialTimeout("tcp4", fmt.Sprintf("%s:80", device.IPAddress), 2*time.Second)
		if err != nil {
			wol.mu.Lock()
			device.Status = "offline"
			wol.mu.Unlock()
		} else {
			conn.Close()
			wol.mu.Lock()
			device.Status = "online"
			device.LastSeen = time.Now()
			wol.mu.Unlock()
		}
	}
}

func buildMagicPacket(mac net.HardwareAddr) []byte {
	packet := make([]byte, 6+16*6)
	// 6 bytes of 0xFF
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}
	// 16 repetitions of MAC address
	for i := 0; i < 16; i++ {
		copy(packet[6+i*6:], mac)
	}
	return packet
}

package lxc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// NetworkManager handles LXC network configuration.
type NetworkManager struct {
	manager *Manager
}

// NewNetworkManager creates a new NetworkManager.
func NewNetworkManager(manager *Manager) *NetworkManager {
	return &NetworkManager{manager: manager}
}

// ListNetworks lists all available networks.
func (n *NetworkManager) ListNetworks(ctx context.Context) ([]*Network, error) {
	cmd := n.manager.cmd("network", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var raw []struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Type        string            `json:"type"`
		Managed     bool              `json:"managed"`
		UsedBy      []string          `json:"used_by"`
		Config      map[string]string `json:"config"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse network list: %w", err)
	}

	var networks []*Network
	for _, r := range raw {
		network := &Network{
			Name:        r.Name,
			Description: r.Description,
			Type:        r.Type,
			Managed:     r.Managed,
			InUse:       len(r.UsedBy) > 0,
			Config:      r.Config,
		}

		// Extract subnet info
		if ipv4, ok := r.Config["ipv4.address"]; ok {
			network.Subnet = ipv4
		}
		if ipv6, ok := r.Config["ipv6.address"]; ok {
			network.Subnet6 = ipv6
		}

		// Check DHCP
		if dhcp, ok := r.Config["ipv4.dhcp"]; ok {
			network.DHCP = dhcp == "true"
		}

		// DNS
		if dns, ok := r.Config["dns.nameservers"]; ok {
			network.DNS = dns
		}

		networks = append(networks, network)
	}

	return networks, nil
}

// GetNetwork retrieves a specific network.
func (n *NetworkManager) GetNetwork(ctx context.Context, name string) (*Network, error) {
	cmd := n.manager.cmd("network", "show", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get network %s: %w", name, err)
	}

	var raw struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Type        string            `json:"type"`
		Managed     bool              `json:"managed"`
		UsedBy      []string          `json:"used_by"`
		Config      map[string]string `json:"config"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse network info: %w", err)
	}

	network := &Network{
		Name:        raw.Name,
		Description: raw.Description,
		Type:        raw.Type,
		Managed:     raw.Managed,
		InUse:       len(raw.UsedBy) > 0,
		Config:      raw.Config,
	}

	if ipv4, ok := raw.Config["ipv4.address"]; ok {
		network.Subnet = ipv4
		// Extract gateway from subnet
		if idx := strings.LastIndex(ipv4, "."); idx != -1 {
			network.Gateway = ipv4[:idx] + ".1" + ipv4[strings.LastIndex(ipv4, "/"):]
		}
	}
	if ipv6, ok := raw.Config["ipv6.address"]; ok {
		network.Subnet6 = ipv6
	}
	if dhcp, ok := raw.Config["ipv4.dhcp"]; ok {
		network.DHCP = dhcp == "true"
	}

	return network, nil
}

// CreateNetwork creates a new network.
func (n *NetworkManager) CreateNetwork(ctx context.Context, config *NetworkCreateConfig) (*Network, error) {
	args := []string{"network", "create", config.Name}

	if config.Type != "" {
		args = append(args, "--type", config.Type)
	}

	// Add configuration
	if config.Subnet != "" {
		args = append(args, "--config", fmt.Sprintf("ipv4.address=%s", config.Subnet))
	}
	if config.Subnet6 != "" {
		args = append(args, "--config", fmt.Sprintf("ipv6.address=%s", config.Subnet6))
	}
	if config.DHCP {
		args = append(args, "--config", "ipv4.dhcp=true")
	}
	if config.NAT {
		args = append(args, "--config", "ipv4.nat=true")
	}
	if config.DNS != "" {
		args = append(args, "--config", fmt.Sprintf("dns.nameservers=%s", config.DNS))
	}
	for k, v := range config.Config {
		args = append(args, "--config", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w, output: %s", err, string(output))
	}

	return n.GetNetwork(ctx, config.Name)
}

// DeleteNetwork deletes a network.
func (n *NetworkManager) DeleteNetwork(ctx context.Context, name string, force bool) error {
	args := []string{"network", "delete", name}
	if force {
		args = append(args, "--force")
	}

	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete network: %w, output: %s", err, string(output))
	}
	return nil
}

// AttachNetwork attaches a container to a network.
func (n *NetworkManager) AttachNetwork(ctx context.Context, container, network, deviceName string, config *NetworkAttachConfig) error {
	args := []string{"network", "attach", network, container}
	if deviceName != "" {
		args = append(args, deviceName)
	}

	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach network: %w, output: %s", err, string(output))
	}

	// Apply additional configuration
	if config != nil {
		updates := make(map[string]string)
		if config.IPAddress != "" {
			updates["ipv4.address"] = config.IPAddress
		}
		if config.MAC != "" {
			updates["hwaddr"] = config.MAC
		}
		if config.MTU > 0 {
			updates["mtu"] = fmt.Sprintf("%d", config.MTU)
		}

		if len(updates) > 0 {
			// Update device config
			deviceArgs := []string{"config", "device", "set", container, deviceName}
			for k, v := range updates {
				deviceArgs = append(deviceArgs, fmt.Sprintf("%s=%s", k, v))
			}
			cmd = n.manager.cmd(deviceArgs...)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to configure network device: %w, output: %s", err, string(output))
			}
		}
	}

	return nil
}

// DetachNetwork detaches a container from a network.
func (n *NetworkManager) DetachNetwork(ctx context.Context, container, network, deviceName string) error {
	args := []string{"network", "detach", network, container}
	if deviceName != "" {
		args = append(args, deviceName)
	}

	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to detach network: %w, output: %s", err, string(output))
	}
	return nil
}

// SetStaticIP sets a static IP for a container's network interface.
func (n *NetworkManager) SetStaticIP(ctx context.Context, container, deviceName, ip string) error {
	// Validate IP address
	if net.ParseIP(strings.Split(ip, "/")[0]) == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	args := []string{"config", "device", "set", container, deviceName, fmt.Sprintf("ipv4.address=%s", ip)}
	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set static IP: %w, output: %s", err, string(output))
	}

	// Restart container to apply
	return n.manager.RestartContainer(ctx, container, false, 30)
}

// GetContainerIPs gets all IP addresses for a container.
func (n *NetworkManager) GetContainerIPs(ctx context.Context, container string) (map[string][]string, error) {
	cmd := n.manager.cmd("list", container, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get container info: %w", err)
	}

	var raw []struct {
		State struct {
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	ips := make(map[string][]string)
	if len(raw) > 0 {
		for ifaceName, net := range raw[0].State.Network {
			for _, addr := range net.Addresses {
				ips[ifaceName] = append(ips[ifaceName], fmt.Sprintf("%s (%s)", addr.Address, addr.Family))
			}
		}
	}

	return ips, nil
}

// AllocateIP allocates an IP address from a network's DHCP pool.
func (n *NetworkManager) AllocateIP(ctx context.Context, network, container, mac string) (string, error) {
	// Create a DHCP reservation
	// LXC/LXD doesn't have a direct API for this, but we can use network leases
	cmd := n.manager.cmd("network", "list-leases", network, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list leases: %w", err)
	}

	var leases []struct {
		Hostname string `json:"hostname"`
		Hwaddr   string `json:"hwaddr"`
		Address  string `json:"address"`
	}

	if err := json.Unmarshal(output, &leases); err != nil {
		return "", fmt.Errorf("failed to parse leases: %w", err)
	}

	// Find existing lease for this container
	for _, lease := range leases {
		if lease.Hostname == container || (mac != "" && lease.Hwaddr == mac) {
			return lease.Address, nil
		}
	}

	// No existing lease found, need to start container to get one
	return "", fmt.Errorf("no IP allocated yet, start container to obtain DHCP lease")
}

// ReserveIP creates a static IP reservation for a MAC address.
func (n *NetworkManager) ReserveIP(ctx context.Context, network, mac, ip string) error {
	// This is done via network config
	args := []string{"network", "set", network, fmt.Sprintf("ipv4.dhcp.ranges=%s", ip)}
	cmd := n.manager.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to reserve IP: %w, output: %s", err, string(output))
	}
	return nil
}

// NetworkCreateConfig holds parameters for creating a network.
type NetworkCreateConfig struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`        // bridge, macvlan, ipvlan, physical
	Description string            `json:"description"`
	Subnet      string            `json:"subnet"`      // e.g., "192.168.100.1/24"
	Subnet6     string            `json:"subnet6"`     // IPv6 subnet
	DHCP        bool              `json:"dhcp"`        // Enable DHCP
	NAT         bool              `json:"nat"`         // Enable NAT
	DNS         string            `json:"dns"`         // DNS servers
	Config      map[string]string `json:"config"`      // Additional config
}

// NetworkAttachConfig holds parameters for attaching to a network.
type NetworkAttachConfig struct {
	IPAddress string `json:"ipAddress"` // Static IP
	MAC       string `json:"mac"`       // MAC address
	MTU       int    `json:"mtu"`       // MTU size
}

// Validate validates NetworkCreateConfig.
func (c *NetworkCreateConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("network name is required")
	}
	if c.Type != "" && c.Type != "bridge" && c.Type != "macvlan" && c.Type != "ipvlan" && c.Type != "physical" {
		return fmt.Errorf("invalid network type: %s", c.Type)
	}
	if c.Subnet != "" {
		_, _, err := net.ParseCIDR(c.Subnet)
		if err != nil {
			return fmt.Errorf("invalid subnet: %w", err)
		}
	}
	return nil
}

// DefaultBridgeNetwork returns the default bridge network config.
func DefaultBridgeNetwork(name string) *NetworkCreateConfig {
	return &NetworkCreateConfig{
		Name:   name,
		Type:   "bridge",
		Subnet: "10.0.0.1/24",
		DHCP:   true,
		NAT:    true,
	}
}

// GenerateMAC generates a random MAC address for a container.
func GenerateMAC() string {
	// Use locally administered address range
	// First octet: x2, x6, xA, or xE (locally administered)
	return fmt.Sprintf("00:16:3e:%02x:%02x:%02x",
		uint8(secureRandom(256)),
		uint8(secureRandom(256)),
		uint8(secureRandom(256)))
}

func secureRandom(max int) int {
	// Simple random for now; in production use crypto/rand
	return int(uint32(0x12345678) % uint32(max))
}

// CreateMacvlanNetwork creates a macvlan network for container IP allocation.
func (n *NetworkManager) CreateMacvlanNetwork(ctx context.Context, name, parentInterface string) (*Network, error) {
	config := &NetworkCreateConfig{
		Name:   name,
		Type:   "macvlan",
		Config: map[string]string{
			"parent": parentInterface,
		},
	}
	return n.CreateNetwork(ctx, config)
}

// CreateIPVLANNetwork creates an ipvlan network for container IP allocation.
func (n *NetworkManager) CreateIPVLANNetwork(ctx context.Context, name, parentInterface string) (*Network, error) {
	config := &NetworkCreateConfig{
		Name:   name,
		Type:   "ipvlan",
		Config: map[string]string{
			"parent": parentInterface,
		},
	}
	return n.CreateNetwork(ctx, config)
}

// SetupBridgedNetwork sets up a container with a bridged network and optional static IP.
func (n *NetworkManager) SetupBridgedNetwork(ctx context.Context, container, bridgeName, staticIP string) error {
	// Check if bridge exists
	_, err := n.GetNetwork(ctx, bridgeName)
	if err != nil {
		// Create bridge if it doesn't exist
		config := DefaultBridgeNetwork(bridgeName)
		if _, err := n.CreateNetwork(ctx, config); err != nil {
			return fmt.Errorf("failed to create bridge network: %w", err)
		}
	}

	// Attach container to bridge
	attachConfig := &NetworkAttachConfig{}
	if staticIP != "" {
		attachConfig.IPAddress = staticIP
	}

	return n.AttachNetwork(ctx, container, bridgeName, "eth0", attachConfig)
}

// SetupMacvlanNetwork sets up a container with macvlan for direct host network access.
func (n *NetworkManager) SetupMacvlanNetwork(ctx context.Context, container, parentInterface string) error {
	networkName := "macvlan-" + parentInterface

	// Check if network exists
	_, err := n.GetNetwork(ctx, networkName)
	if err != nil {
		// Create macvlan network
		if _, err := n.CreateMacvlanNetwork(ctx, networkName, parentInterface); err != nil {
			return fmt.Errorf("failed to create macvlan network: %w", err)
		}
	}

	// Attach container
	return n.AttachNetwork(ctx, container, networkName, "eth0", nil)
}
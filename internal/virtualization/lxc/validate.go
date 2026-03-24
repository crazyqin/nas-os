package lxc

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Validate validates CreateConfig.
func (c *CreateConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("container name is required")
	}

	// Validate name format
	if !isValidContainerName(c.Name) {
		return fmt.Errorf("invalid container name: must be lowercase alphanumeric with hyphens, max 63 characters")
	}

	if c.Image == "" {
		return fmt.Errorf("image is required")
	}

	// Validate resources
	if err := ValidateResourceConfig(&c.Resources); err != nil {
		return fmt.Errorf("invalid resource config: %w", err)
	}

	// Validate networks
	for _, net := range c.Networks {
		if err := validateNetworkConfig(&net); err != nil {
			return fmt.Errorf("invalid network config: %w", err)
		}
	}

	// Validate volumes
	for _, vol := range c.Volumes {
		if err := validateStorageConfig(&vol); err != nil {
			return fmt.Errorf("invalid storage config: %w", err)
		}
	}

	return nil
}

// isValidContainerName checks if a container name is valid.
func isValidContainerName(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}

	// Single character names are valid if lowercase letter
	if len(name) == 1 {
		return regexp.MustCompile(`^[a-z]$`).MatchString(name)
	}

	// Must start with a letter
	if !regexp.MustCompile(`^[a-z]`).MatchString(name) {
		return false
	}

	// Must end with alphanumeric
	if !regexp.MustCompile(`[a-z0-9]$`).MatchString(name) {
		return false
	}

	// Only lowercase alphanumeric and hyphens
	matched, _ := regexp.MatchString(`^[a-z][a-z0-9-]*[a-z0-9]$`, name)
	return matched
}

// validateNetworkConfig validates a network configuration.
func validateNetworkConfig(config *NetworkConfig) error {
	if config.Name == "" {
		// Empty name is OK, will use default
		return nil
	}

	if config.Type != "" {
		validTypes := map[string]bool{
			"bridge":   true,
			"macvlan":  true,
			"ipvlan":   true,
			"physical": true,
			"nic":      true,
			"":         true,
		}
		if !validTypes[config.Type] {
			return fmt.Errorf("invalid network type: %s", config.Type)
		}
	}

	if config.IPAddress != "" {
		// Validate IP or CIDR
		ip := config.IPAddress
		if strings.Contains(ip, "/") {
			_, _, err := net.ParseCIDR(ip)
			if err != nil {
				return fmt.Errorf("invalid IP address with CIDR: %s", ip)
			}
		} else {
			if net.ParseIP(ip) == nil {
				return fmt.Errorf("invalid IP address: %s", ip)
			}
		}
	}

	if config.Gateway != "" {
		if net.ParseIP(config.Gateway) == nil {
			return fmt.Errorf("invalid gateway address: %s", config.Gateway)
		}
	}

	if config.DNS != "" {
		// DNS can be multiple IPs separated by comma
		dnsServers := strings.Split(config.DNS, ",")
		for _, dns := range dnsServers {
			dns = strings.TrimSpace(dns)
			if net.ParseIP(dns) == nil {
				return fmt.Errorf("invalid DNS server: %s", dns)
			}
		}
	}

	if config.MAC != "" {
		if _, err := net.ParseMAC(config.MAC); err != nil {
			return fmt.Errorf("invalid MAC address: %s", config.MAC)
		}
	}

	if config.MTU > 0 && (config.MTU < 576 || config.MTU > 9216) {
		return fmt.Errorf("MTU must be between 576 and 9216")
	}

	return nil
}

// validateStorageConfig validates a storage configuration.
func validateStorageConfig(config *StorageConfig) error {
	if config.Pool == "" {
		return fmt.Errorf("storage pool is required")
	}

	if config.Size > 0 && config.Size < 1 {
		return fmt.Errorf("disk size must be at least 1GB")
	}

	if config.Path == "" {
		return fmt.Errorf("mount path is required")
	}

	if !strings.HasPrefix(config.Path, "/") {
		return fmt.Errorf("mount path must be absolute: %s", config.Path)
	}

	return nil
}

// ValidateExecConfig validates an ExecConfig.
func ValidateExecConfig(config *ExecConfig) error {
	if len(config.Command) == 0 {
		return fmt.Errorf("command is required")
	}
	return nil
}

// ValidateMigrationConfig validates a MigrationConfig.
func ValidateMigrationConfig(config *MigrationConfig) error {
	if config.TargetHost == "" {
		return fmt.Errorf("target host is required")
	}
	if config.TargetPort <= 0 || config.TargetPort > 65535 {
		config.TargetPort = 8443 // Default Incus/LXD port
	}
	return nil
}

// ValidateBackupConfig validates a BackupConfig.
func ValidateBackupConfig(config *BackupConfig) error {
	if config.Name == "" {
		return fmt.Errorf("backup name is required")
	}

	validCompression := map[string]bool{
		"gzip":  true,
		"bzip2": true,
		"xz":    true,
		"none":  true,
		"":      true,
	}
	if !validCompression[config.Compression] {
		return fmt.Errorf("invalid compression type: %s", config.Compression)
	}

	if config.Expiration != nil && config.Expiration.Before(time.Now()) {
		return fmt.Errorf("expiration time cannot be in the past")
	}

	return nil
}

// ContainerValidator provides validation helpers for containers.
type ContainerValidator struct {
	manager *Manager
}

// NewContainerValidator creates a new ContainerValidator.
func NewContainerValidator(manager *Manager) *ContainerValidator {
	return &ContainerValidator{manager: manager}
}

// ValidateCreate validates a container creation request.
func (v *ContainerValidator) ValidateCreate(config *CreateConfig) []string {
	var warnings []string

	// Check for potentially problematic configurations
	if config.Privileged {
		warnings = append(warnings, "privileged containers have full host access and may pose security risks")
	}

	if config.Resources.MemoryLimit == 0 {
		warnings = append(warnings, "no memory limit set; container may consume all available memory")
	}

	if config.Resources.CPUCores == 0 {
		warnings = append(warnings, "no CPU limit set; container may consume all CPU resources")
	}

	// Check for network configuration
	if len(config.Networks) == 0 {
		warnings = append(warnings, "no network configured; container will have no network access")
	}

	// Check image availability (warning only)
	// Note: Image availability check could be added here if needed in the future

	return warnings
}

// ValidateResourceQuota checks if the requested resources are available.
func (v *ContainerValidator) ValidateResourceQuota(resources *ResourceConfig) error {
	// Get host resources
	cpuInfo, err := getHostCPUInfo()
	if err == nil && resources.CPUCores > cpuInfo.Cores {
		return fmt.Errorf("requested %d CPU cores but host only has %d", resources.CPUCores, cpuInfo.Cores)
	}

	memInfo, err := getHostMemoryInfo()
	if err == nil && resources.MemoryLimit > memInfo.TotalMB {
		return fmt.Errorf("requested %d MB memory but host only has %d MB available", resources.MemoryLimit, memInfo.TotalMB)
	}

	return nil
}

// HostResourceInfo contains host resource information.
type HostResourceInfo struct {
	Cores   int
	MHz     int
	Model   string
	TotalMB uint64
	FreeMB  uint64
}

type cpuInfo struct {
	Cores int
	MHz   int
	Model string
}

type memInfo struct {
	TotalMB uint64
	FreeMB  uint64
}

func getHostCPUInfo() (*cpuInfo, error) {
	info := &cpuInfo{}

	// Get CPU cores
	if output, err := exec.Command("nproc").Output(); err == nil {
		if n, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			info.Cores = n
		}
	}

	// Get CPU model
	if content, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "model name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					info.Model = strings.TrimSpace(parts[1])
				}
				break
			}
		}
	}

	return info, nil
}

func getHostMemoryInfo() (*memInfo, error) {
	info := &memInfo{}

	if content, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
						info.TotalMB = kb / 1024
					}
				}
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
						info.FreeMB = kb / 1024
					}
				}
			}
		}
	}

	return info, nil
}

// SanitizeName sanitizes a string to be a valid container name.
func SanitizeName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace invalid characters with hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]`)
	name = reg.ReplaceAllString(name, "-")

	// Remove consecutive hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	// Trim hyphens from start and end
	name = strings.Trim(name, "-")

	// Truncate to 63 characters
	if len(name) > 63 {
		name = name[:63]
	}

	return name
}

// ParseImageRef parses an image reference into components.
func ParseImageRef(ref string) (server, image, alias string, err error) {
	// Formats:
	// - image (alias from default server)
	// - server:image (image from specific server)
	// - server:image/alias

	parts := strings.SplitN(ref, ":", 2)
	if len(parts) == 1 {
		// Just an image/alias name
		image = parts[0]
		alias = parts[0]
		return
	}

	server = parts[0]
	imageParts := strings.SplitN(parts[1], "/", 2)
	if len(imageParts) == 1 {
		image = imageParts[0]
		alias = imageParts[0]
	} else {
		image = imageParts[0]
		alias = imageParts[1]
	}

	return
}

// IsImageAvailable checks if an image is available locally.
func (m *Manager) IsImageAvailable(image string) (bool, error) {
	cmd := m.cmd("image", "show", image)
	return cmd.Run() == nil, nil
}

// CommonImages provides common image aliases for convenience.
var CommonImages = map[string][]string{
	"ubuntu":     {"ubuntu/22.04", "ubuntu/24.04", "ubuntu/20.04"},
	"debian":     {"debian/12", "debian/11", "debian/10"},
	"alpine":     {"alpine/3.19", "alpine/3.18", "alpine/edge"},
	"centos":     {"centos/9-Stream", "centos/8-Stream"},
	"rockylinux": {"rockylinux/9", "rockylinux/8"},
	"fedora":     {"fedora/39", "fedora/40"},
	"archlinux":  {"archlinux"},
}

// GetImageAlias returns the best image alias for a given OS.
func GetImageAlias(os string, version string) string {
	if images, ok := CommonImages[os]; ok {
		if version != "" {
			for _, img := range images {
				if strings.Contains(img, version) {
					return img
				}
			}
		}
		if len(images) > 0 {
			return images[0]
		}
	}
	return os
}

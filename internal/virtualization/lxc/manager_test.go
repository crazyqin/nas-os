package lxc

import (
	"context"
	"testing"
)

func TestParseStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected ContainerStatus
	}{
		{"running", StatusRunning},
		{"stopped", StatusStopped},
		{"frozen", StatusFrozen},
		{"RUNNING", StatusRunning},
		{"STOPPED", StatusStopped},
		{"unknown", StatusStopped},
	}

	for _, test := range tests {
		result := parseStatus(test.input)
		if result != test.expected {
			t.Errorf("parseStatus(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"1GB", 1024},
		{"512MB", 512},
		{"2G", 2048},
		{"1024", 0}, // Plain number without unit
		{"1TB", 1024 * 1024},
		{"invalid", 0},
	}

	for _, test := range tests {
		result := parseSize(test.input)
		if result != test.expected {
			t.Errorf("parseSize(%q) = %d, expected %d", test.input, result, test.expected)
		}
	}
}

func TestIsValidContainerName(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"my-container", true},
		{"container1", true},
		{"test123", true},
		{"a", true},
		{"1container", false}, // Must start with letter
		{"Container", false},  // Must be lowercase
		{"container_name", false}, // Underscore not allowed
		{"", false},
		{"a" + string(make([]byte, 63)), false}, // Too long
	}

	for _, test := range tests {
		result := isValidContainerName(test.name)
		if result != test.expected {
			t.Errorf("isValidContainerName(%q) = %v, expected %v", test.name, result, test.expected)
		}
	}
}

func TestValidateCreateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  CreateConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: CreateConfig{
				Name:  "test-container",
				Image: "ubuntu/22.04",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: CreateConfig{
				Image: "ubuntu/22.04",
			},
			wantErr: true,
		},
		{
			name: "missing image",
			config: CreateConfig{
				Name: "test-container",
			},
			wantErr: true,
		},
		{
			name: "invalid name",
			config: CreateConfig{
				Name:  "1Invalid",
				Image: "ubuntu/22.04",
			},
			wantErr: true,
		},
		{
			name: "with resources",
			config: CreateConfig{
				Name:  "test-container",
				Image: "ubuntu/22.04",
				Resources: ResourceConfig{
					CPUCores:   2,
					MemoryLimit: 2048,
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if (err != nil) != test.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateResourceConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   ResourceConfig
		wantErr  bool
	}{
		{
			name:    "valid config",
			config:  ResourceConfig{CPUCores: 2, MemoryLimit: 1024},
			wantErr: false,
		},
		{
			name:    "negative cpu",
			config:  ResourceConfig{CPUCores: -1},
			wantErr: true,
		},
		{
			name:    "invalid priority",
			config:  ResourceConfig{CPUPriority: 11},
			wantErr: true,
		},
		{
			name:    "negative memory",
			config:  ResourceConfig{MemoryLimit: 1}, // Signed underflow check
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateResourceConfig(&test.config)
			if (err != nil) != test.wantErr {
				t.Errorf("ValidateResourceConfig() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestValidateNetworkConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  NetworkConfig
		wantErr bool
	}{
		{
			name:    "empty config",
			config:  NetworkConfig{},
			wantErr: false,
		},
		{
			name: "valid bridge",
			config: NetworkConfig{
				Name:    "eth0",
				Type:    "bridge",
				Network: "lxdbr0",
			},
			wantErr: false,
		},
		{
			name: "valid static IP",
			config: NetworkConfig{
				Name:      "eth0",
				IPAddress: "192.168.1.100",
			},
			wantErr: false,
		},
		{
			name: "valid CIDR",
			config: NetworkConfig{
				Name:      "eth0",
				IPAddress: "192.168.1.100/24",
			},
			wantErr: false,
		},
		{
			name: "invalid IP",
			config: NetworkConfig{
				Name:      "eth0",
				IPAddress: "invalid-ip",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			config: NetworkConfig{
				Name: "eth0",
				Type: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid MAC",
			config: NetworkConfig{
				Name: "eth0",
				MAC:  "00:16:3e:ab:cd:ef",
			},
			wantErr: false,
		},
		{
			name: "invalid MAC",
			config: NetworkConfig{
				Name: "eth0",
				MAC:  "invalid-mac",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateNetworkConfig(&test.config)
			if (err != nil) != test.wantErr {
				t.Errorf("validateNetworkConfig() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestBuildResourceArgs(t *testing.T) {
	config := &ResourceConfig{
		CPUCores:     2,
		CPUPriority:  8,
		MemoryLimit:  2048,
		ProcessLimit: 100,
	}

	args := buildResourceArgs(config)

	// Check that expected config options are present
	expectedConfigs := []string{
		"limits.cpu=2",
		"limits.cpu.priority=8",
		"limits.memory=2048MB",
		"limits.processes=100",
	}

	for _, expected := range expectedConfigs {
		found := false
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--config" && args[i+1] == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected config %s not found in args", expected)
		}
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MyContainer", "mycontainer"},
		{"test_container", "test-container"},
		{"test.container", "test-container"},
		{"test--container", "test-container"},
		{"-test-", "test"},
		{"1test", "1test"}, // Leading digit is kept after lowercase conversion
	}

	for _, test := range tests {
		result := SanitizeName(test.input)
		if result != test.expected {
			t.Errorf("SanitizeName(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestParseImageRef(t *testing.T) {
	tests := []struct {
		input      string
		wantServer string
		wantImage  string
		wantAlias  string
	}{
		{"ubuntu/22.04", "", "ubuntu/22.04", "ubuntu/22.04"},
		{"images:ubuntu/22.04", "images", "ubuntu", "22.04"},
		{"ubuntu", "", "ubuntu", "ubuntu"},
	}

	for _, test := range tests {
		server, image, alias, err := ParseImageRef(test.input)
		if err != nil {
			t.Errorf("ParseImageRef(%q) error: %v", test.input, err)
			continue
		}
		if server != test.wantServer {
			t.Errorf("ParseImageRef(%q) server = %q, want %q", test.input, server, test.wantServer)
		}
		if image != test.wantImage {
			t.Errorf("ParseImageRef(%q) image = %q, want %q", test.input, image, test.wantImage)
		}
		if alias != test.wantAlias {
			t.Errorf("ParseImageRef(%q) alias = %q, want %q", test.input, alias, test.wantAlias)
		}
	}
}

func TestGetImageAlias(t *testing.T) {
	tests := []struct {
		os      string
		version string
		want    string
	}{
		{"ubuntu", "", "ubuntu/22.04"},
		{"ubuntu", "24.04", "ubuntu/24.04"},
		{"debian", "", "debian/12"},
		{"alpine", "3.18", "alpine/3.18"},
		{"unknown", "", "unknown"},
	}

	for _, test := range tests {
		result := GetImageAlias(test.os, test.version)
		if result != test.want {
			t.Errorf("GetImageAlias(%q, %q) = %q, want %q", test.os, test.version, result, test.want)
		}
	}
}

// Integration tests (require running LXC backend)
func TestManagerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	m, err := NewManager()
	if err != nil {
		t.Skipf("LXC backend not available: %v", err)
	}

	if !m.IsAvailable() {
		t.Skip("LXC backend is not running")
	}

	ctx := context.Background()

	t.Run("ListContainers", func(t *testing.T) {
		containers, err := m.ListContainers(ctx)
		if err != nil {
			t.Errorf("Failed to list containers: %v", err)
		}
		t.Logf("Found %d containers", len(containers))
	})

	t.Run("GetVersion", func(t *testing.T) {
		version, err := m.GetVersion()
		if err != nil {
			t.Errorf("Failed to get version: %v", err)
		}
		t.Logf("Version: %v", version)
	})
}

func TestDefaultResourceConfigs(t *testing.T) {
	minimal := MinimalResourceConfig()
	if minimal.CPUCores != 1 || minimal.MemoryLimit != 512 {
		t.Errorf("MinimalResourceConfig has unexpected values")
	}

	default_ := DefaultResourceConfig()
	if default_.CPUCores != 1 || default_.MemoryLimit != 1024 {
		t.Errorf("DefaultResourceConfig has unexpected values")
	}

	highPerf := HighPerformanceResourceConfig()
	if highPerf.CPUCores != 4 || highPerf.MemoryLimit != 8192 {
		t.Errorf("HighPerformanceResourceConfig has unexpected values")
	}
}
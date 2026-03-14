package plugin

import (
	"testing"
)

func TestSandboxCheckPermission(t *testing.T) {
	tests := []struct {
		name           string
		config         SandboxConfig
		permission     PermissionType
		expectedResult bool
	}{
		{
			name: "granted permission",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles, PermWriteFiles},
				},
			},
			permission:     PermReadFiles,
			expectedResult: true,
		},
		{
			name: "denied permission",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
					Deny:        []PermissionType{PermWriteFiles},
				},
			},
			permission:     PermWriteFiles,
			expectedResult: false,
		},
		{
			name: "admin grants all",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermAdmin},
				},
			},
			permission:     PermWriteFiles,
			expectedResult: true,
		},
		{
			name: "admin deny blocks all",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
					Deny:        []PermissionType{PermAdmin},
				},
			},
			permission:     PermReadFiles,
			expectedResult: false,
		},
		{
			name: "not granted permission",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
				},
			},
			permission:     PermWriteFiles,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox := NewSandbox("test-plugin", tt.config)
			result := sandbox.CheckPermission(tt.permission)

			if result != tt.expectedResult {
				t.Errorf("Expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

func TestSandboxCheckFileAccess(t *testing.T) {
	tests := []struct {
		name          string
		config        SandboxConfig
		path          string
		op            string
		expectError   bool
	}{
		{
			name: "allowed path read",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
				},
				AllowedPaths: []string{"/data"},
			},
			path:        "/data/file.txt",
			op:          "read",
			expectError: false,
		},
		{
			name: "denied path",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
				},
				DeniedPaths: []string{"/etc"},
			},
			path:        "/etc/passwd",
			op:          "read",
			expectError: true,
		},
		{
			name: "path not in allowed list",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
				},
				AllowedPaths: []string{"/data"},
			},
			path:        "/other/file.txt",
			op:          "read",
			expectError: true,
		},
		{
			name: "write without permission",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermReadFiles},
				},
			},
			path:        "/data/file.txt",
			op:          "write",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox := NewSandbox("test-plugin", tt.config)
			err := sandbox.CheckFileAccess(tt.path, tt.op)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestSandboxCheckNetworkAccess(t *testing.T) {
	tests := []struct {
		name        string
		config      SandboxConfig
		host        string
		port        int
		expectError bool
	}{
		{
			name: "allowed host",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermNetworkDial},
				},
				AllowedHosts: []string{"api.example.com"},
			},
			host:        "api.example.com",
			port:        443,
			expectError: false,
		},
		{
			name: "denied host",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermNetworkDial},
				},
				DeniedHosts: []string{"malicious.com"},
			},
			host:        "malicious.com",
			port:        80,
			expectError: true,
		},
		{
			name: "host not in allowed list",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermNetworkDial},
				},
				AllowedHosts: []string{"api.example.com"},
			},
			host:        "other.com",
			port:        443,
			expectError: true,
		},
		{
			name: "port not allowed",
			config: SandboxConfig{
				Permissions: PermissionSet{
					Permissions: []PermissionType{PermNetworkDial},
				},
				AllowedPorts: []int{80, 443},
			},
			host:        "example.com",
			port:        8080,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sandbox := NewSandbox("test-plugin", tt.config)
			err := sandbox.CheckNetworkAccess(tt.host, tt.port)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}

func TestSandboxManager(t *testing.T) {
	sm := NewSandboxManager()

	// Test default profiles exist
	profiles := sm.GetProfiles()
	if len(profiles) < 4 {
		t.Errorf("Expected at least 4 default profiles, got %d", len(profiles))
	}

	// Test creating sandbox
	sandbox, err := sm.CreateSandbox("test-plugin", "standard")
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	if sandbox == nil {
		t.Fatal("Expected sandbox, got nil")
	}

	// Test getting sandbox
	retrieved, exists := sm.GetSandbox("test-plugin")
	if !exists {
		t.Error("Expected sandbox to exist")
	}
	if retrieved != sandbox {
		t.Error("Expected same sandbox instance")
	}

	// Test listing sandboxes
	sandboxes := sm.ListSandboxes()
	if len(sandboxes) != 1 {
		t.Errorf("Expected 1 sandbox, got %d", len(sandboxes))
	}

	// Test removing sandbox
	sm.RemoveSandbox("test-plugin")
	_, exists = sm.GetSandbox("test-plugin")
	if exists {
		t.Error("Expected sandbox to be removed")
	}
}

func TestSandboxManagerCustomProfile(t *testing.T) {
	sm := NewSandboxManager()

	// Add custom profile
	customConfig := SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{PermReadFiles},
		},
		MaxMemoryMB: 128,
	}
	sm.AddProfile("custom", customConfig)

	// Create sandbox with custom profile
	sandbox, err := sm.CreateSandbox("custom-plugin", "custom")
	if err != nil {
		t.Fatalf("Failed to create sandbox: %v", err)
	}

	config := sandbox.GetConfig()
	if config.MaxMemoryMB != 128 {
		t.Errorf("Expected max memory 128, got %d", config.MaxMemoryMB)
	}
}

func TestSandboxViolations(t *testing.T) {
	config := SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{PermReadFiles},
		},
	}
	sandbox := NewSandbox("test-plugin", config)

	// Trigger violations
	sandbox.CheckPermission(PermWriteFiles)
	sandbox.CheckFileAccess("/etc/passwd", "write")

	violations := sandbox.GetViolations()
	if len(violations) < 2 {
		t.Errorf("Expected at least 2 violations, got %d", len(violations))
	}
}

func TestSandboxConfigJSON(t *testing.T) {
	config := SandboxConfig{
		Permissions: PermissionSet{
			Permissions: []PermissionType{PermReadFiles, PermWriteFiles},
			Deny:        []PermissionType{PermExecFiles},
		},
		AllowedPaths:  []string{"/data"},
		MaxMemoryMB:   256,
		MaxCPUPercent: 50,
	}

	// Test ToJSON
	jsonStr, err := config.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	// Test ParseSandboxConfig
	parsed, err := ParseSandboxConfig(jsonStr)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if parsed.MaxMemoryMB != config.MaxMemoryMB {
		t.Errorf("Expected max memory %d, got %d", config.MaxMemoryMB, parsed.MaxMemoryMB)
	}
}
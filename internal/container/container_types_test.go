package container

import (
	"testing"
	"time"
)

func TestPortMapping_String(t *testing.T) {
	tests := []struct {
		mapping  PortMapping
		expected string
	}{
		{
			PortMapping{HostIP: "0.0.0.0", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"},
			"0.0.0.0:8080:80/tcp",
		},
		{
			PortMapping{HostIP: "", HostPort: "3000", ContainerPort: "3000", Protocol: "tcp"},
			"3000:3000/tcp",
		},
		{
			PortMapping{HostIP: "127.0.0.1", HostPort: "5432", ContainerPort: "5432", Protocol: "tcp"},
			"127.0.0.1:5432:5432/tcp",
		},
	}

	for _, tt := range tests {
		result := tt.mapping.String()
		if result != tt.expected {
			t.Errorf("String() = %s, want %s", result, tt.expected)
		}
	}
}

func TestVolumeMount_String(t *testing.T) {
	tests := []struct {
		mount    VolumeMount
		expected string
	}{
		{
			VolumeMount{Source: "/host/path", Destination: "/container/path", Mode: "rw"},
			"/host/path:/container/path:rw",
		},
		{
			VolumeMount{Source: "/host/data", Destination: "/data", Mode: ""},
			"/host/data:/data",
		},
	}

	for _, tt := range tests {
		result := tt.mount.String()
		if result != tt.expected {
			t.Errorf("String() = %s, want %s", result, tt.expected)
		}
	}
}

func TestContainer_Fields(t *testing.T) {
	now := time.Now()
	container := &Container{
		ID:            "abc123",
		Name:          "test-container",
		Image:         "nginx:latest",
		Command:       "/bin/bash",
		Created:       now,
		Status:        "running",
		State:         "running",
		Running:       true,
		RestartPolicy: "unless-stopped",
		Ports: []PortMapping{
			{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"},
		},
		Volumes: []VolumeMount{
			{Source: "/host", Destination: "/container", Mode: "rw"},
		},
		Labels:   map[string]string{"app": "test"},
		Networks: []string{"bridge"},
	}

	if container.ID != "abc123" {
		t.Errorf("ID = %s, want abc123", container.ID)
	}

	if container.Name != "test-container" {
		t.Errorf("Name = %s, want test-container", container.Name)
	}

	if !container.Running {
		t.Error("Running should be true")
	}

	if len(container.Ports) != 1 {
		t.Errorf("Ports count = %d, want 1", len(container.Ports))
	}
}

func TestContainerConfig_Fields(t *testing.T) {
	config := &ContainerConfig{
		Name:        "my-app",
		Image:       "my-app:latest",
		Command:     []string{"npm", "start"},
		Ports:       []string{"3000:3000", "8080:80"},
		Volumes:     []string{"/host/data:/data"},
		Environment: map[string]string{"NODE_ENV": "production"},
		Network:     "bridge",
		Restart:     "always",
		CPULimit:    "0.5",
		MemLimit:    "512m",
		Labels:      map[string]string{"service": "api"},
		Detach:      true,
	}

	if config.Name != "my-app" {
		t.Errorf("Name = %s, want my-app", config.Name)
	}

	if len(config.Ports) != 2 {
		t.Errorf("Ports count = %d, want 2", len(config.Ports))
	}

	if config.Environment["NODE_ENV"] != "production" {
		t.Error("Environment variable not set correctly")
	}
}

func TestContainerLog_Fields(t *testing.T) {
	now := time.Now()
	log := ContainerLog{
		Timestamp: now,
		Line:      "Server started on port 3000",
		Source:    "stdout",
	}

	if log.Line != "Server started on port 3000" {
		t.Errorf("Line = %s, want 'Server started on port 3000'", log.Line)
	}

	if log.Source != "stdout" {
		t.Errorf("Source = %s, want stdout", log.Source)
	}
}

func TestPortMapping_Fields(t *testing.T) {
	port := PortMapping{
		HostIP:        "0.0.0.0",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	if port.HostIP != "0.0.0.0" {
		t.Errorf("HostIP = %s, want 0.0.0.0", port.HostIP)
	}

	if port.HostPort != "8080" {
		t.Errorf("HostPort = %s, want 8080", port.HostPort)
	}

	if port.ContainerPort != "80" {
		t.Errorf("ContainerPort = %s, want 80", port.ContainerPort)
	}

	if port.Protocol != "tcp" {
		t.Errorf("Protocol = %s, want tcp", port.Protocol)
	}
}

func TestVolumeMount_Fields(t *testing.T) {
	mount := VolumeMount{
		Source:      "/host/path",
		Destination: "/container/path",
		Mode:        "ro",
		RW:          false,
	}

	if mount.Source != "/host/path" {
		t.Errorf("Source = %s, want /host/path", mount.Source)
	}

	if mount.RW {
		t.Error("RW should be false")
	}
}

func TestContainerStats_Fields(t *testing.T) {
	now := time.Now()
	stats := ContainerStats{
		CPUUsage:   25.5,
		MemUsage:   512 * 1024 * 1024,
		MemLimit:   1024 * 1024 * 1024,
		MemPercent: 50.0,
		NetRX:      1024 * 1024,
		NetTX:      512 * 1024,
		BlockRead:  2048 * 1024,
		BlockWrite: 1024 * 1024,
		PIDs:       15,
		Timestamp:  now,
	}

	if stats.CPUUsage != 25.5 {
		t.Errorf("CPUUsage = %f, want 25.5", stats.CPUUsage)
	}

	if stats.MemPercent != 50.0 {
		t.Errorf("MemPercent = %f, want 50.0", stats.MemPercent)
	}

	if stats.PIDs != 15 {
		t.Errorf("PIDs = %d, want 15", stats.PIDs)
	}
}

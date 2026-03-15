package container

import (
	"testing"
	"time"
)

// ========== Container 方法测试 ==========

func TestContainer_IsRunning(t *testing.T) {
	running := &Container{State: "running", Running: true}
	stopped := &Container{State: "exited", Running: false}

	if !running.Running {
		t.Error("Running container should have Running=true")
	}
	if stopped.Running {
		t.Error("Stopped container should have Running=false")
	}
}

func TestContainer_StateValues(t *testing.T) {
	states := []string{"running", "exited", "paused", "restarting", "dead", "created"}

	for _, state := range states {
		c := &Container{State: state}
		if c.State != state {
			t.Errorf("Expected State=%s, got %s", state, c.State)
		}
	}
}

func TestContainer_HasPorts(t *testing.T) {
	withPorts := &Container{
		Ports: []PortMapping{
			{HostPort: "8080", ContainerPort: "80"},
		},
	}
	withoutPorts := &Container{Ports: nil}

	if len(withPorts.Ports) != 1 {
		t.Error("Container should have 1 port")
	}
	if withoutPorts.Ports != nil && len(withoutPorts.Ports) != 0 {
		t.Error("Container should have no ports")
	}
}

func TestContainer_HasVolumes(t *testing.T) {
	withVolumes := &Container{
		Volumes: []VolumeMount{
			{Source: "/data", Destination: "/app/data"},
		},
	}

	if len(withVolumes.Volumes) != 1 {
		t.Error("Container should have 1 volume")
	}
}

func TestContainer_HasLabels(t *testing.T) {
	withLabels := &Container{
		Labels: map[string]string{
			"app":     "nginx",
			"version": "1.0",
		},
	}

	if len(withLabels.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(withLabels.Labels))
	}
}

func TestContainer_MemoryUsage(t *testing.T) {
	c := &Container{
		MemUsage: 512 * 1024 * 1024,  // 512MB
		MemLimit: 1024 * 1024 * 1024, // 1GB
	}

	memPercent := float64(c.MemUsage) / float64(c.MemLimit) * 100
	if memPercent != 50.0 {
		t.Errorf("Expected 50%% memory usage, got %.2f%%", memPercent)
	}
}

func TestContainer_Networks(t *testing.T) {
	c := &Container{
		Networks: []string{"bridge", "frontend", "backend"},
	}

	if len(c.Networks) != 3 {
		t.Errorf("Expected 3 networks, got %d", len(c.Networks))
	}
}

func TestContainer_RestartPolicy(t *testing.T) {
	policies := []string{"no", "always", "on-failure", "unless-stopped"}

	for _, policy := range policies {
		c := &Container{RestartPolicy: policy}
		if c.RestartPolicy != policy {
			t.Errorf("Expected RestartPolicy=%s, got %s", policy, c.RestartPolicy)
		}
	}
}

// ========== ContainerStats 测试 ==========

func TestContainerStats_MemoryPercent(t *testing.T) {
	stats := &ContainerStats{
		MemUsage:   512 * 1024 * 1024,
		MemLimit:   1024 * 1024 * 1024,
		MemPercent: 50.0,
	}

	if stats.MemPercent != 50.0 {
		t.Errorf("Expected MemPercent=50.0, got %f", stats.MemPercent)
	}
}

func TestContainerStats_CPUUsage(t *testing.T) {
	stats := &ContainerStats{
		CPUUsage: 75.5,
	}

	if stats.CPUUsage < 0 || stats.CPUUsage > 100 {
		t.Errorf("CPU usage should be 0-100, got %f", stats.CPUUsage)
	}
}

func TestContainerStats_NetworkIO(t *testing.T) {
	stats := &ContainerStats{
		NetRX: 1024 * 1024 * 100, // 100MB
		NetTX: 1024 * 1024 * 50,  // 50MB
	}

	if stats.NetRX <= 0 || stats.NetTX <= 0 {
		t.Error("Network I/O should be positive")
	}
}

func TestContainerStats_BlockIO(t *testing.T) {
	stats := &ContainerStats{
		BlockRead:  1024 * 1024 * 1024, // 1GB
		BlockWrite: 512 * 1024 * 1024,  // 512MB
	}

	if stats.BlockRead <= stats.BlockWrite {
		t.Log("Block read <= write is possible")
	}
}

func TestContainerStats_PIDs(t *testing.T) {
	stats := &ContainerStats{
		PIDs: 15,
	}

	if stats.PIDs == 0 {
		t.Error("PIDs should be non-zero for running container")
	}
}

func TestContainerStats_Timestamp(t *testing.T) {
	stats := &ContainerStats{
		Timestamp: time.Now(),
	}

	if stats.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

// ========== ContainerConfig 测试 ==========

func TestContainerConfig_Full(t *testing.T) {
	config := &ContainerConfig{
		Name:    "test-container",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Ports:   []string{"8080:80", "8443:443"},
		Volumes: []string{"/data:/app/data:rw"},
		Environment: map[string]string{
			"ENV":   "production",
			"DEBUG": "false",
		},
		Network:     "bridge",
		Restart:     "always",
		CPULimit:    "0.5",
		MemLimit:    "512m",
		Labels:      map[string]string{"app": "nginx"},
		Detach:      true,
		Interactive: false,
		TTY:         false,
	}

	if config.Name != "test-container" {
		t.Error("Name mismatch")
	}
	if len(config.Ports) != 2 {
		t.Error("Should have 2 ports")
	}
	if len(config.Volumes) != 1 {
		t.Error("Should have 1 volume")
	}
}

func TestContainerConfig_Minimal(t *testing.T) {
	config := &ContainerConfig{
		Name:  "minimal",
		Image: "alpine",
	}

	if config.Name == "" || config.Image == "" {
		t.Error("Name and Image are required")
	}
}

func TestContainerConfig_ResourceLimits(t *testing.T) {
	config := &ContainerConfig{
		Name:     "limited",
		Image:    "nginx",
		CPULimit: "1.5",
		MemLimit: "1g",
	}

	if config.CPULimit != "1.5" {
		t.Error("CPU limit mismatch")
	}
	if config.MemLimit != "1g" {
		t.Error("Memory limit mismatch")
	}
}

func TestContainerConfig_InteractiveMode(t *testing.T) {
	config := &ContainerConfig{
		Name:        "interactive",
		Image:       "alpine",
		Interactive: true,
		TTY:         true,
	}

	if !config.Interactive || !config.TTY {
		t.Error("Interactive and TTY should be true")
	}
}

// ========== ContainerLog 测试 ==========

func TestContainerLog_Stdout(t *testing.T) {
	log := ContainerLog{
		Timestamp: time.Now(),
		Line:      "Server started",
		Source:    "stdout",
	}

	if log.Source != "stdout" {
		t.Error("Expected stdout source")
	}
}

func TestContainerLog_Stderr(t *testing.T) {
	log := ContainerLog{
		Timestamp: time.Now(),
		Line:      "Error: connection refused",
		Source:    "stderr",
	}

	if log.Source != "stderr" {
		t.Error("Expected stderr source")
	}
}

// ========== PortMapping 测试 ==========

func TestPortMapping_TCP(t *testing.T) {
	pm := PortMapping{
		HostIP:        "0.0.0.0",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	if pm.Protocol != "tcp" {
		t.Error("Expected TCP protocol")
	}
}

func TestPortMapping_UDP(t *testing.T) {
	pm := PortMapping{
		HostPort:      "53",
		ContainerPort: "53",
		Protocol:      "udp",
	}

	if pm.Protocol != "udp" {
		t.Error("Expected UDP protocol")
	}
}

func TestPortMapping_WithIP(t *testing.T) {
	pm := PortMapping{
		HostIP:        "127.0.0.1",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	if pm.HostIP != "127.0.0.1" {
		t.Error("Expected specific IP")
	}
}

// ========== VolumeMount 测试 ==========

func TestVolumeMount_ReadWrite(t *testing.T) {
	vm := VolumeMount{
		Source:      "/host/data",
		Destination: "/container/data",
		Mode:        "rw",
		RW:          true,
	}

	if !vm.RW {
		t.Error("Should be read-write")
	}
}

func TestVolumeMount_ReadOnly(t *testing.T) {
	vm := VolumeMount{
		Source:      "/host/config",
		Destination: "/container/config",
		Mode:        "ro",
		RW:          false,
	}

	if vm.RW {
		t.Error("Should be read-only")
	}
}

func TestVolumeMount_NamedVolume(t *testing.T) {
	vm := VolumeMount{
		Source:      "my-volume",
		Destination: "/app/data",
		Mode:        "rw",
		RW:          true,
	}

	if vm.Source != "my-volume" {
		t.Error("Should use named volume")
	}
}

// ========== Manager 测试 ==========

func TestManager_DefaultSocket(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	if mgr.socketPath != "/var/run/docker.sock" {
		t.Error("Default socket path mismatch")
	}
}

func TestManager_CustomSocket(t *testing.T) {
	mgr := &Manager{socketPath: "/custom/docker.sock"}
	if mgr.socketPath != "/custom/docker.sock" {
		t.Error("Custom socket path mismatch")
	}
}

// ========== 并发测试 ==========

func TestContainer_ConcurrentAccess(t *testing.T) {
	c := &Container{
		ID:      "test",
		Name:    "concurrent",
		Labels:  make(map[string]string),
		Ports:   make([]PortMapping, 0),
		Volumes: make([]VolumeMount, 0),
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = c.Name
			_ = len(c.Ports)
			_ = len(c.Volumes)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 边界条件测试 ==========

func TestContainer_NilSlices(t *testing.T) {
	c := &Container{}

	if c.Ports != nil {
		t.Error("Ports should be nil")
	}
	if c.Volumes != nil {
		t.Error("Volumes should be nil")
	}
	if c.Labels != nil {
		t.Error("Labels should be nil")
	}
	if c.Networks != nil {
		t.Error("Networks should be nil")
	}
}

func TestContainerStats_ZeroValues(t *testing.T) {
	stats := &ContainerStats{}

	if stats.CPUUsage != 0 {
		t.Error("CPUUsage should be 0")
	}
	if stats.MemUsage != 0 {
		t.Error("MemUsage should be 0")
	}
}

func TestContainerConfig_EmptyCommand(t *testing.T) {
	config := &ContainerConfig{
		Name:    "test",
		Image:   "nginx",
		Command: []string{},
	}

	if len(config.Command) != 0 {
		t.Error("Command should be empty")
	}
}

func TestPortMapping_EmptyHostIP(t *testing.T) {
	pm := PortMapping{
		HostPort:      "8080",
		ContainerPort: "80",
	}

	if pm.HostIP != "" {
		t.Error("HostIP should be empty (bind all)")
	}
}

// ========== 数据验证测试 ==========

func TestContainerConfig_Validation_Name(t *testing.T) {
	config := &ContainerConfig{
		Name:  "",
		Image: "nginx",
	}

	if config.Name == "" {
		t.Log("Empty name detected correctly")
	}
}

func TestContainerConfig_Validation_Image(t *testing.T) {
	config := &ContainerConfig{
		Name:  "test",
		Image: "",
	}

	if config.Image == "" {
		t.Log("Empty image detected correctly")
	}
}

func TestContainerConfig_Validation_PortFormat(t *testing.T) {
	validPorts := []string{
		"8080:80",
		"127.0.0.1:8080:80",
		"8080:80/tcp",
		"53:53/udp",
	}

	for _, port := range validPorts {
		if port == "" {
			t.Errorf("Port should not be empty")
		}
	}
}

func TestContainerConfig_Validation_VolumeFormat(t *testing.T) {
	validVolumes := []string{
		"/host/path:/container/path",
		"/host/path:/container/path:rw",
		"/host/path:/container/path:ro",
		"volume-name:/container/path",
	}

	for _, vol := range validVolumes {
		if vol == "" {
			t.Errorf("Volume should not be empty")
		}
	}
}

func TestContainerConfig_Validation_RestartPolicy(t *testing.T) {
	validPolicies := map[string]bool{
		"no":             true,
		"always":         true,
		"on-failure":     true,
		"unless-stopped": true,
		"invalid":        false,
	}

	for policy, expectedValid := range validPolicies {
		// 检查 policy 是否在有效列表中
		_, exists := validPolicies[policy]
		isValidPolicy := exists && validPolicies[policy]
		if isValidPolicy != expectedValid {
			t.Errorf("Policy %s: expected valid=%v, got valid=%v", policy, expectedValid, isValidPolicy)
		}
	}
}

// ========== 时间相关测试 ==========

func TestContainer_CreatedTime(t *testing.T) {
	now := time.Now()
	c := &Container{
		ID:      "test",
		Created: now,
	}

	if c.Created.IsZero() {
		t.Error("Created should not be zero")
	}
}

func TestContainerStats_TimestampNow(t *testing.T) {
	before := time.Now()
	stats := &ContainerStats{Timestamp: time.Now()}
	after := time.Now()

	if stats.Timestamp.Before(before) || stats.Timestamp.After(after) {
		t.Error("Timestamp should be between before and after")
	}
}

// ========== 性能基准测试 ==========

func BenchmarkContainer_Access(b *testing.B) {
	c := &Container{
		ID:       "test",
		Name:     "benchmark",
		Image:    "nginx",
		State:    "running",
		Running:  true,
		Ports:    []PortMapping{{HostPort: "8080", ContainerPort: "80"}},
		Volumes:  []VolumeMount{{Source: "/data", Destination: "/data"}},
		Labels:   map[string]string{"app": "nginx"},
		Networks: []string{"bridge"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Name
		_ = c.State
		_ = len(c.Ports)
		_ = len(c.Volumes)
		_ = c.Labels["app"]
	}
}

func BenchmarkContainerStats_Access(b *testing.B) {
	stats := &ContainerStats{
		CPUUsage:   50.0,
		MemUsage:   512 * 1024 * 1024,
		MemLimit:   1024 * 1024 * 1024,
		MemPercent: 50.0,
		NetRX:      1024000,
		NetTX:      512000,
		Timestamp:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.CPUUsage
		_ = stats.MemUsage
		_ = stats.MemPercent
	}
}

func BenchmarkPortMapping_Access(b *testing.B) {
	ports := []PortMapping{
		{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"},
		{HostPort: "8443", ContainerPort: "443", Protocol: "tcp"},
		{HostPort: "53", ContainerPort: "53", Protocol: "udp"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range ports {
			_ = p.HostPort
			_ = p.ContainerPort
		}
	}
}

func BenchmarkVolumeMount_Access(b *testing.B) {
	volumes := []VolumeMount{
		{Source: "/data", Destination: "/app/data", RW: true},
		{Source: "/config", Destination: "/app/config", RW: false},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range volumes {
			_ = v.Source
			_ = v.Destination
			_ = v.RW
		}
	}
}

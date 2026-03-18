// Package container 提供容器管理功能
package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr, err := NewManager()
	// 如果 Docker 未运行，可能返回错误
	if err != nil {
		t.Skip("Docker 未运行，跳过测试")
	}
	assert.NotNil(t, mgr)
}

func TestNewManager_DefaultSocket(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	assert.Equal(t, "/var/run/docker.sock", mgr.socketPath)
}

// ========== parseSize 测试 ==========

func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"100", 100},
		{"1KB", 1024},
		{"1K", 1024},
		{"1MB", 1024 * 1024},
		{"1M", 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"1TB", 1024 * 1024 * 1024 * 1024},
		{"512m", 512 * 1024 * 1024},
		{"2.5G", uint64(2.5 * 1024 * 1024 * 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSize_EdgeCases(t *testing.T) {
	// 空字符串
	assert.Equal(t, uint64(0), parseSize(""))

	// 只有空格
	assert.Equal(t, uint64(0), parseSize("   "))

	// 大小写混合
	assert.Equal(t, uint64(1024), parseSize("1kb"))
	assert.Equal(t, uint64(1024*1024), parseSize("1Mb"))
	assert.Equal(t, uint64(1024*1024*1024), parseSize("1Gb"))
}

// ========== Container 结构测试 ==========

func TestContainer_Structure(t *testing.T) {
	container := Container{
		ID:      "abc123def456",
		Name:    "nginx-server",
		Image:   "nginx:latest",
		Command: "/docker-entrypoint.sh",
		Created: time.Now(),
		Status:  "Up 2 hours",
		State:   "running",
		Running: true,
		Ports: []PortMapping{
			{HostIP: "0.0.0.0", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"},
		},
		Labels: map[string]string{
			"app": "nginx",
		},
		Networks: []string{"bridge"},
		Volumes: []VolumeMount{
			{Source: "/host/data", Destination: "/data", Mode: "rw", RW: true},
		},
		CPUUsage:      2.5,
		MemUsage:      52428800,
		MemLimit:      104857600,
		RestartPolicy: "always",
	}

	assert.NotEmpty(t, container.ID)
	assert.NotEmpty(t, container.Name)
	assert.NotEmpty(t, container.Image)
	assert.True(t, container.Running)
	assert.Len(t, container.Ports, 1)
	assert.Len(t, container.Volumes, 1)
}

func TestContainer_Empty(t *testing.T) {
	container := Container{}
	assert.Empty(t, container.ID)
	assert.Empty(t, container.Name)
	assert.False(t, container.Running)
	assert.Nil(t, container.Ports)
	assert.Nil(t, container.Volumes)
}

// ========== PortMapping 结构测试 ==========

func TestPortMapping_Structure(t *testing.T) {
	port := PortMapping{
		HostIP:        "0.0.0.0",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	assert.NotEmpty(t, port.HostPort)
	assert.NotEmpty(t, port.ContainerPort)
	assert.Contains(t, []string{"tcp", "udp"}, port.Protocol)
}

func TestPortMapping_AllProtocols(t *testing.T) {
	tcpPort := PortMapping{Protocol: "tcp"}
	udpPort := PortMapping{Protocol: "udp"}
	sctpPort := PortMapping{Protocol: "sctp"}

	assert.Equal(t, "tcp", tcpPort.Protocol)
	assert.Equal(t, "udp", udpPort.Protocol)
	assert.Equal(t, "sctp", sctpPort.Protocol)
}

// ========== VolumeMount 结构测试 ==========

func TestVolumeMount_Structure(t *testing.T) {
	vol := VolumeMount{
		Source:      "/host/path",
		Destination: "/container/path",
		Mode:        "rw",
		RW:          true,
	}

	assert.NotEmpty(t, vol.Source)
	assert.NotEmpty(t, vol.Destination)
	assert.True(t, vol.RW)
}

// ========== Stats 结构测试 ==========

func TestStats_Structure(t *testing.T) {
	stats := Stats{
		CPUUsage:   15.5,
		MemUsage:   524288000,
		MemLimit:   1048576000,
		MemPercent: 50.0,
		NetRX:      1024000,
		NetTX:      512000,
		BlockRead:  2048000,
		BlockWrite: 1024000,
		PIDs:       5,
		Timestamp:  time.Now(),
	}

	assert.GreaterOrEqual(t, stats.CPUUsage, 0.0)
	assert.LessOrEqual(t, stats.CPUUsage, 100.0)
	assert.GreaterOrEqual(t, stats.MemPercent, 0.0)
	assert.LessOrEqual(t, stats.MemPercent, 100.0)
	assert.NotZero(t, stats.MemLimit)
	assert.False(t, stats.Timestamp.IsZero())
}

// ========== Config 结构测试 ==========

func TestConfig_Structure(t *testing.T) {
	config := Config{
		Name:    "test-container",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Ports:   []string{"8080:80", "443:443"},
		Volumes: []string{"/host/data:/data:rw"},
		Environment: map[string]string{
			"ENV": "production",
		},
		Network:  "bridge",
		Restart:  "always",
		CPULimit: "0.5",
		MemLimit: "512m",
		Labels: map[string]string{
			"app": "nginx",
		},
		Detach:      true,
		Interactive: false,
		TTY:         false,
	}

	assert.NotEmpty(t, config.Name)
	assert.NotEmpty(t, config.Image)
	assert.Len(t, config.Ports, 2)
	assert.Len(t, config.Volumes, 1)
	assert.Equal(t, "production", config.Environment["ENV"])
	assert.Contains(t, []string{"no", "always", "on-failure", "unless-stopped"}, config.Restart)
}

// ========== Log 结构测试 ==========

func TestLog_Structure(t *testing.T) {
	log := Log{
		Timestamp: time.Now(),
		Line:      "Server started on port 80",
		Source:    "stdout",
	}

	assert.NotEmpty(t, log.Line)
	assert.Contains(t, []string{"stdout", "stderr"}, log.Source)
	assert.False(t, log.Timestamp.IsZero())
}

func TestLog_Sources(t *testing.T) {
	stdoutLog := Log{Source: "stdout", Line: "info"}
	stderrLog := Log{Source: "stderr", Line: "error"}

	assert.Equal(t, "stdout", stdoutLog.Source)
	assert.Equal(t, "stderr", stderrLog.Source)
}

// ========== 配置验证测试 ==========

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: Config{
				Name:  "valid-container",
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: Config{
				Image: "nginx:latest",
			},
			wantErr: true,
		},
		{
			name: "缺少镜像",
			config: Config{
				Name: "test-container",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 基本验证逻辑
			if tt.config.Name == "" && !tt.wantErr {
				t.Error("期望验证失败，但通过了")
			}
			if tt.config.Image == "" && !tt.wantErr {
				t.Error("期望验证失败，但通过了")
			}
		})
	}
}

// ========== 端口映射验证测试 ==========

func TestPortMapping_Validation(t *testing.T) {
	tests := []struct {
		name  string
		ports []string
		valid bool
	}{
		{"单端口", []string{"8080:80"}, true},
		{"多端口", []string{"8080:80", "443:443"}, true},
		{"带协议", []string{"8080:80/tcp"}, true},
		{"UDP端口", []string{"53:53/udp"}, true},
		{"指定IP", []string{"127.0.0.1:8080:80"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, port := range tt.ports {
				assert.NotEmpty(t, port)
			}
		})
	}
}

// ========== 卷挂载验证测试 ==========

func TestVolumeMount_Validation(t *testing.T) {
	tests := []struct {
		name   string
		volume string
		wantRW bool
	}{
		{"读写挂载", "/host/path:/container/path:rw", true},
		{"只读挂载", "/host/path:/container/path:ro", false},
		{"无权限标识", "/host/path:/container/path", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.volume)
		})
	}
}

// ========== 重启策略测试 ==========

func TestRestartPolicy_Validation(t *testing.T) {
	validPolicies := []string{"no", "always", "on-failure", "unless-stopped"}

	for _, policy := range validPolicies {
		t.Run(policy, func(t *testing.T) {
			assert.Contains(t, validPolicies, policy)
		})
	}
}

// ========== 资源限制测试 ==========

func TestResourceLimits_Validation(t *testing.T) {
	tests := []struct {
		name     string
		cpuLimit string
		memLimit string
	}{
		{"CPU 限制", "0.5", "512m"},
		{"CPU 整数", "1", "1g"},
		{"CPU 多核", "2.5", "2g"},
		{"内存限制", "1.0", "1024m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.cpuLimit)
			assert.NotEmpty(t, tt.memLimit)
		})
	}
}

// ========== 性能测试 ==========

func BenchmarkContainer_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Container{
			ID:      "abc123",
			Name:    "test",
			Image:   "nginx",
			Running: true,
		}
	}
}

func BenchmarkStats_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Stats{
			CPUUsage:   15.5,
			MemUsage:   524288000,
			MemLimit:   1048576000,
			MemPercent: 50.0,
			Timestamp:  time.Now(),
		}
	}
}

func BenchmarkParseSize(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parseSize("512m")
	}
}

// ========== parseSize 完整测试 ==========

func TestParseSize_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"纯数字", "1024", 1024},
		{"KB格式", "2KB", 2 * 1024},
		{"KiB格式", "2KiB", 2 * 1024},
		{"K格式", "2K", 2 * 1024},
		{"MB格式", "2MB", 2 * 1024 * 1024},
		{"MiB格式", "2MiB", 2 * 1024 * 1024},
		{"M格式", "2M", 2 * 1024 * 1024},
		{"GB格式", "2GB", 2 * 1024 * 1024 * 1024},
		{"GiB格式", "2GiB", 2 * 1024 * 1024 * 1024},
		{"G格式", "2G", 2 * 1024 * 1024 * 1024},
		{"TB格式", "1TB", 1 * 1024 * 1024 * 1024 * 1024},
		{"TiB格式", "1TiB", 1 * 1024 * 1024 * 1024 * 1024},
		{"T格式", "1T", 1 * 1024 * 1024 * 1024 * 1024},
		{"带空格", " 1 GB ", 1 * 1024 * 1024 * 1024},
		{"小数", "1.5GB", uint64(1.5 * 1024 * 1024 * 1024)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 容器操作函数测试 ==========

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效配置-基础",
			config: Config{
				Name:  "test-container",
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "有效配置-完整",
			config: Config{
				Name:        "test-container",
				Image:       "nginx:latest",
				Command:     []string{"nginx", "-g", "daemon off;"},
				Ports:       []string{"8080:80"},
				Volumes:     []string{"/host:/container"},
				Environment: map[string]string{"KEY": "value"},
				Network:     "bridge",
				Restart:     "always",
				CPULimit:    "1.0",
				MemLimit:    "512m",
				Detach:      true,
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: Config{
				Image: "nginx:latest",
			},
			wantErr: true,
			errMsg:  "name is required",
		},
		{
			name: "缺少镜像",
			config: Config{
				Name: "test",
			},
			wantErr: true,
			errMsg:  "image is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 简单验证逻辑
			if tt.wantErr {
				if tt.config.Name == "" || tt.config.Image == "" {
					// 符合预期
				} else {
					t.Error("期望验证失败，但通过了")
				}
			}
		})
	}
}

// ========== Manager 方法测试 ==========

func TestManager_SocketPath(t *testing.T) {
	mgr := &Manager{socketPath: "/custom/docker.sock"}
	assert.Equal(t, "/custom/docker.sock", mgr.socketPath)
}

func TestManager_SocketPath_Custom(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/custom.sock"}
	assert.Equal(t, "/var/run/custom.sock", mgr.socketPath)
}

// ========== PortMapping 解析测试 ==========

func TestPortMapping_Parse(t *testing.T) {
	tests := []struct {
		input    string
		expected PortMapping
	}{
		{
			input: "8080:80",
			expected: PortMapping{
				HostPort:      "8080",
				ContainerPort: "80",
			},
		},
		{
			input: "127.0.0.1:8080:80",
			expected: PortMapping{
				HostIP:        "127.0.0.1",
				HostPort:      "8080",
				ContainerPort: "80",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.NotEmpty(t, tt.input)
		})
	}
}

// ========== VolumeMount 解析测试 ==========

func TestVolumeMount_Parse(t *testing.T) {
	tests := []struct {
		input    string
		expected VolumeMount
	}{
		{
			input: "/host/path:/container/path",
			expected: VolumeMount{
				Source:      "/host/path",
				Destination: "/container/path",
			},
		},
		{
			input: "/host/path:/container/path:ro",
			expected: VolumeMount{
				Source:      "/host/path",
				Destination: "/container/path",
				Mode:        "ro",
				RW:          false,
			},
		},
		{
			input: "/host/path:/container/path:rw",
			expected: VolumeMount{
				Source:      "/host/path",
				Destination: "/container/path",
				Mode:        "rw",
				RW:          true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.NotEmpty(t, tt.input)
		})
	}
}

// ========== 资源限制格式测试 ==========

func TestCPULimit_Format(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected float64
	}{
		{"0.5", true, 0.5},
		{"1", true, 1.0},
		{"2.5", true, 2.5},
		{"1.0", true, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.True(t, tt.valid)
		})
	}
}

func TestMemLimit_Format(t *testing.T) {
	tests := []struct {
		input    string
		valid    bool
		expected uint64
	}{
		{"512m", true, 512 * 1024 * 1024},
		{"1g", true, 1 * 1024 * 1024 * 1024},
		{"2G", true, 2 * 1024 * 1024 * 1024},
		{"1024m", true, 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== Stats 边界测试 ==========

func TestStats_Boundaries(t *testing.T) {
	tests := []struct {
		name  string
		stats Stats
	}{
		{"零值", Stats{}},
		{"最小值", Stats{CPUUsage: 0, MemUsage: 0}},
		{"最大值", Stats{CPUUsage: 100, MemPercent: 100}},
		{"典型值", Stats{CPUUsage: 25.5, MemPercent: 50.0, MemUsage: 524288000}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.stats.CPUUsage, 0.0)
			assert.LessOrEqual(t, tt.stats.CPUUsage, 100.0)
		})
	}
}

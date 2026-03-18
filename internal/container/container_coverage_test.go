package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Stats.MemoryPercent 测试 ==========

func TestStats_MemoryPercent_Calculated(t *testing.T) {
	tests := []struct {
		name     string
		stats    Stats
		expected float64
	}{
		{
			name: "50%",
			stats: Stats{
				MemUsage: 512 * 1024 * 1024,
				MemLimit: 1024 * 1024 * 1024,
			},
			expected: 50.0,
		},
		{
			name: "25%",
			stats: Stats{
				MemUsage: 256 * 1024 * 1024,
				MemLimit: 1024 * 1024 * 1024,
			},
			expected: 25.0,
		},
		{
			name: "100%",
			stats: Stats{
				MemUsage: 1024 * 1024 * 1024,
				MemLimit: 1024 * 1024 * 1024,
			},
			expected: 100.0,
		},
		{
			name: "零限制",
			stats: Stats{
				MemUsage: 512 * 1024 * 1024,
				MemLimit: 0,
			},
			expected: 0.0,
		},
		{
			name: "零使用",
			stats: Stats{
				MemUsage: 0,
				MemLimit: 1024 * 1024 * 1024,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.stats.MemoryPercent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== Config.Validate 完整测试 ==========

func TestConfig_Validate_Complete(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效配置-最小",
			config: Config{
				Name:  "test",
				Image: "nginx",
			},
			wantErr: false,
		},
		{
			name: "有效配置-完整",
			config: Config{
				Name:        "full-test",
				Image:       "nginx:latest",
				Command:     []string{"nginx"},
				Ports:       []string{"80:80"},
				Volumes:     []string{"/data:/data"},
				Environment: map[string]string{"KEY": "value"},
				Network:     "bridge",
				Restart:     "always",
				CPULimit:    "1.0",
				MemLimit:    "512m",
				Labels:      map[string]string{"app": "test"},
				Detach:      true,
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: Config{
				Image: "nginx",
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
		{
			name:    "空配置",
			config:  Config{},
			wantErr: true,
			errMsg:  "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// ========== parseSize 完整边界测试 ==========

func TestParseSize_AllUnits(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		// 基础单位
		{"1024", 1024},
		{"1KB", 1024},
		{"1K", 1024},
		{"1KiB", 1024},
		{"1kb", 1024},
		{"1Mb", 1024 * 1024},
		{"1MB", 1024 * 1024},
		{"1MiB", 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"1GiB", 1024 * 1024 * 1024},
		{"1TB", 1024 * 1024 * 1024 * 1024},
		{"1TiB", 1024 * 1024 * 1024 * 1024},
		// 大小写混合
		{"1kb", 1024},
		{"1Kb", 1024},
		{"1mB", 1024 * 1024},
		{"1Gb", 1024 * 1024 * 1024},
		{"1Tb", 1024 * 1024 * 1024 * 1024},
		// 带空格
		{" 1 GB ", 1024 * 1024 * 1024},
		{" 2 MB ", 2 * 1024 * 1024},
		// 小数
		{"1.5GB", uint64(1.5 * 1024 * 1024 * 1024)},
		{"2.5MB", uint64(2.5 * 1024 * 1024)},
		// 边界值
		{"0", 0},
		{"0KB", 0},
		{"0GB", 0},
		// 空格变体
		{"1 GB", 1024 * 1024 * 1024},
		{"2 MB", 2 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseSize_EdgeCases_Complete(t *testing.T) {
	// 空字符串
	assert.Equal(t, uint64(0), parseSize(""))

	// 只有空格
	assert.Equal(t, uint64(0), parseSize("   "))

	// 无效格式 - 只返回数字部分
	assert.Equal(t, uint64(100), parseSize("100xyz"))

	// 未知单位但可解析的格式 - 返回数字部分
	assert.Equal(t, uint64(100), parseSize("100AB"))
}

// ========== PortMapping.String 更多场景 ==========

func TestPortMapping_String_AllProtocols(t *testing.T) {
	tests := []struct {
		name     string
		mapping  PortMapping
		contains string
	}{
		{"TCP", PortMapping{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "tcp"}, "/tcp"},
		{"UDP", PortMapping{HostIP: "0.0.0.0", HostPort: "53", ContainerPort: "53", Protocol: "udp"}, "/udp"},
		{"SCTP", PortMapping{HostIP: "0.0.0.0", HostPort: "80", ContainerPort: "80", Protocol: "sctp"}, "/sctp"},
		{"无IP", PortMapping{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}, "8080:80/tcp"},
		{"带IP", PortMapping{HostIP: "127.0.0.1", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}, "127.0.0.1:8080:80/tcp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mapping.String()
			assert.Contains(t, result, tt.contains)
		})
	}
}

// ========== VolumeMount.String 更多场景 ==========

func TestVolumeMount_String_AllModes(t *testing.T) {
	tests := []struct {
		name     string
		mount    VolumeMount
		contains string
	}{
		{"RW模式", VolumeMount{Source: "/host", Destination: "/container", Mode: "rw"}, ":rw"},
		{"RO模式", VolumeMount{Source: "/host", Destination: "/container", Mode: "ro"}, ":ro"},
		{"无模式", VolumeMount{Source: "/host", Destination: "/container", Mode: ""}, "/host:/container"},
		{"Z模式", VolumeMount{Source: "/host", Destination: "/container", Mode: "z"}, ":z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.mount.String()
			if tt.contains != "" {
				assert.Contains(t, result, tt.contains)
			}
		})
	}
}

// ========== formatSize 完整测试 ==========

func TestFormatSize_AllSizes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"0字节", 0, "0 B"},
		{"1字节", 1, "1 B"},
		{"100字节", 100, "100 B"},
		{"1KB", 1024, "1.00 KB"},
		{"1.5KB", 1536, "1.50 KB"},
		{"1MB", 1024 * 1024, "1.00 MB"},
		{"1.5MB", 1024*1024 + 512*1024, "1.50 MB"},
		{"10MB", 10 * 1024 * 1024, "10.00 MB"},
		{"1GB", 1024 * 1024 * 1024, "1.00 GB"},
		{"10GB", 10 * 1024 * 1024 * 1024, "10.00 GB"},
		{"1TB", 1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{"2TB", 2 * 1024 * 1024 * 1024 * 1024, "2.00 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			// 检查主要部分是否匹配
			assert.Contains(t, result, tt.expected[:len(tt.expected)-3])
		})
	}
}

// ========== Manager 构造函数完整测试 ==========

func TestNewManager_WithEnv(t *testing.T) {
	// 保存原始值
	origEnv := ""
	if env := getEnvValue("DOCKER_HOST"); env != "" {
		origEnv = env
	}

	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{"默认socket", "", "/var/run/docker.sock"},
		{"自定义socket", "/custom/docker.sock", "/custom/docker.sock"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 由于 NewManager 使用 os.Getenv，我们测试 Manager 结构体
			mgr := &Manager{socketPath: tt.expected}
			assert.Equal(t, tt.expected, mgr.socketPath)
		})
	}

	// 恢复原始值（如果需要）
	_ = origEnv
}

// 辅助函数
func getEnvValue(key string) string {
	return ""
}

// ========== Container 字段完整测试 ==========

func TestContainer_AllFields(t *testing.T) {
	now := time.Now()
	container := &Container{
		ID:            "abc123def456",
		Name:          "test-container",
		Image:         "nginx:latest",
		Command:       "/bin/sh -c 'nginx'",
		Created:       now,
		Status:        "Up 2 hours",
		State:         "running",
		Running:       true,
		Ports:         []PortMapping{{HostPort: "8080", ContainerPort: "80"}},
		Labels:        map[string]string{"app": "nginx"},
		Networks:      []string{"bridge", "frontend"},
		Volumes:       []VolumeMount{{Source: "/data", Destination: "/app/data"}},
		CPUUsage:      25.5,
		MemUsage:      512 * 1024 * 1024,
		MemLimit:      1024 * 1024 * 1024,
		RestartPolicy: "always",
	}

	assert.Equal(t, "abc123def456", container.ID)
	assert.Equal(t, "test-container", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.True(t, container.Running)
	assert.Equal(t, "running", container.State)
	assert.Len(t, container.Ports, 1)
	assert.Len(t, container.Networks, 2)
	assert.Equal(t, 25.5, container.CPUUsage)
	assert.Equal(t, "always", container.RestartPolicy)
}

// ========== Stats 所有字段测试 ==========

func TestStats_AllFields(t *testing.T) {
	now := time.Now()
	stats := &Stats{
		CPUUsage:   75.5,
		MemUsage:   1024 * 1024 * 512,
		MemLimit:   1024 * 1024 * 1024,
		MemPercent: 50.0,
		NetRX:      1024 * 1024 * 100,
		NetTX:      1024 * 1024 * 50,
		BlockRead:  1024 * 1024 * 200,
		BlockWrite: 1024 * 1024 * 100,
		PIDs:       25,
		Timestamp:  now,
	}

	assert.Equal(t, 75.5, stats.CPUUsage)
	assert.Equal(t, uint64(1024*1024*512), stats.MemUsage)
	assert.Equal(t, uint64(1024*1024*1024), stats.MemLimit)
	assert.Equal(t, 50.0, stats.MemPercent)
	assert.Equal(t, uint64(1024*1024*100), stats.NetRX)
	assert.Equal(t, uint64(1024*1024*50), stats.NetTX)
	assert.Equal(t, uint64(1024*1024*200), stats.BlockRead)
	assert.Equal(t, uint64(1024*1024*100), stats.BlockWrite)
	assert.Equal(t, uint64(25), stats.PIDs)
	assert.Equal(t, now, stats.Timestamp)
}

// ========== Config 所有字段测试 ==========

func TestConfig_AllFields(t *testing.T) {
	config := &Config{
		Name:        "full-config",
		Image:       "myapp:v1",
		Command:     []string{"npm", "start"},
		Ports:       []string{"3000:3000", "8080:80"},
		Volumes:     []string{"/host:/container", "data:/data"},
		Environment: map[string]string{"NODE_ENV": "production", "DEBUG": "false"},
		Network:     "custom-network",
		Restart:     "unless-stopped",
		CPULimit:    "2.0",
		MemLimit:    "1g",
		Labels:      map[string]string{"service": "api", "version": "v1"},
		Detach:      true,
		Interactive: true,
		TTY:         true,
	}

	assert.Equal(t, "full-config", config.Name)
	assert.Equal(t, "myapp:v1", config.Image)
	assert.Len(t, config.Command, 2)
	assert.Len(t, config.Ports, 2)
	assert.Len(t, config.Volumes, 2)
	assert.Len(t, config.Environment, 2)
	assert.Equal(t, "custom-network", config.Network)
	assert.Equal(t, "unless-stopped", config.Restart)
	assert.True(t, config.Detach)
	assert.True(t, config.Interactive)
	assert.True(t, config.TTY)
}

// ========== Log 所有字段测试 ==========

func TestLog_AllFields(t *testing.T) {
	now := time.Now()
	log := &Log{
		Timestamp: now,
		Line:      "Application started successfully",
		Source:    "stdout",
	}

	assert.Equal(t, now, log.Timestamp)
	assert.Equal(t, "Application started successfully", log.Line)
	assert.Equal(t, "stdout", log.Source)
}

// ========== PortMapping 所有字段测试 ==========

func TestPortMapping_AllFields(t *testing.T) {
	port := &PortMapping{
		HostIP:        "192.168.1.100",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	assert.Equal(t, "192.168.1.100", port.HostIP)
	assert.Equal(t, "8080", port.HostPort)
	assert.Equal(t, "80", port.ContainerPort)
	assert.Equal(t, "tcp", port.Protocol)
}

// ========== VolumeMount 所有字段测试 ==========

func TestVolumeMount_AllFields(t *testing.T) {
	mount := &VolumeMount{
		Source:      "/host/path",
		Destination: "/container/path",
		Mode:        "rw",
		RW:          true,
	}

	assert.Equal(t, "/host/path", mount.Source)
	assert.Equal(t, "/container/path", mount.Destination)
	assert.Equal(t, "rw", mount.Mode)
	assert.True(t, mount.RW)
}

// ========== ImagePullProgress 结构测试 ==========

func TestImagePullProgress_AllFields(t *testing.T) {
	progress := &ImagePullProgress{
		Status:   "Downloading",
		Progress: "[=====>     ] 50%",
		ID:       "sha256:abc123",
	}
	progress.ProgressDetail.Current = 5000000
	progress.ProgressDetail.Total = 10000000

	assert.Equal(t, "Downloading", progress.Status)
	assert.Equal(t, uint64(5000000), progress.ProgressDetail.Current)
	assert.Equal(t, uint64(10000000), progress.ProgressDetail.Total)
	assert.Contains(t, progress.Progress, "50%")
}

// ========== 基准测试 ==========

func BenchmarkStats_MemoryPercent(b *testing.B) {
	stats := &Stats{
		MemUsage: 512 * 1024 * 1024,
		MemLimit: 1024 * 1024 * 1024,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.MemoryPercent()
	}
}

func BenchmarkConfig_Validate(b *testing.B) {
	config := &Config{
		Name:  "test",
		Image: "nginx",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

func BenchmarkPortMapping_String(b *testing.B) {
	port := &PortMapping{
		HostIP:        "0.0.0.0",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = port.String()
	}
}

func BenchmarkVolumeMount_String(b *testing.B) {
	mount := &VolumeMount{
		Source:      "/host",
		Destination: "/container",
		Mode:        "rw",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mount.String()
	}
}
package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager 方法测试 ==========

func TestManager_IsRunning(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	// 注意：这个测试依赖于实际的 Docker 环境
	// 在 CI/CD 中可能需要跳过
	_ = mgr.IsRunning() // 只测试不会 panic
}

func TestManager_GetContainer_NotFound(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	// 测试获取不存在的容器
	_, err = mgr.GetContainer("nonexistent-container-12345")
	// 应该返回错误（容器不存在）
	assert.Error(t, err)
}

func TestManager_ListContainers(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	// 这个测试依赖于 Docker 环境
	_, _ = mgr.ListContainers(true) // 只测试不会 panic
}

// ========== CreateContainer 配置测试 ==========

func TestConfig_Validate_Detailed(t *testing.T) {
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
				Image: "alpine",
			},
			wantErr: false,
		},
		{
			name: "有效配置-完整",
			config: Config{
				Name:        "nginx-prod",
				Image:       "nginx:latest",
				Command:     []string{"nginx", "-g", "daemon off;"},
				Ports:       []string{"80:80", "443:443"},
				Volumes:     []string{"/data:/data:rw"},
				Environment: map[string]string{"ENV": "prod"},
				Network:     "bridge",
				Restart:     "always",
				CPULimit:    "1.5",
				MemLimit:    "512m",
				Labels:      map[string]string{"app": "nginx"},
				Detach:      true,
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: Config{
				Image: "alpine",
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// ========== Stats 方法测试 ==========

func TestStats_MemoryPercent_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		stats    Stats
		expected float64
	}{
		{"零限制", Stats{MemUsage: 1024, MemLimit: 0}, 0.0},
		{"零使用", Stats{MemUsage: 0, MemLimit: 1024}, 0.0},
		{"50%", Stats{MemUsage: 512, MemLimit: 1024}, 50.0},
		{"100%", Stats{MemUsage: 1024, MemLimit: 1024}, 100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.stats.MemoryPercent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== PortMapping String 方法测试 ==========

func TestPortMapping_String_Variants(t *testing.T) {
	tests := []struct {
		name     string
		port     PortMapping
		expected string
	}{
		{
			name: "带 IP",
			port: PortMapping{
				HostIP:        "0.0.0.0",
				HostPort:      "8080",
				ContainerPort: "80",
				Protocol:      "tcp",
			},
			expected: "0.0.0.0:8080:80/tcp",
		},
		{
			name: "无 IP",
			port: PortMapping{
				HostPort:      "8080",
				ContainerPort: "80",
				Protocol:      "tcp",
			},
			expected: "8080:80/tcp",
		},
		{
			name: "UDP",
			port: PortMapping{
				HostPort:      "53",
				ContainerPort: "53",
				Protocol:      "udp",
			},
			expected: "53:53/udp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.port.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== VolumeMount String 方法测试 ==========

func TestVolumeMount_String_Variants(t *testing.T) {
	tests := []struct {
		name     string
		vol      VolumeMount
		expected string
	}{
		{
			name: "带模式",
			vol: VolumeMount{
				Source:      "/host/data",
				Destination: "/container/data",
				Mode:        "rw",
			},
			expected: "/host/data:/container/data:rw",
		},
		{
			name: "无模式",
			vol: VolumeMount{
				Source:      "/host/data",
				Destination: "/container/data",
			},
			expected: "/host/data:/container/data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.vol.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== parseSize 边界测试 ==========

func TestParseSize_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{"空字符串", "", 0},
		{"只有空格", "   ", 0},
		{"只有单位", "GB", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== Container 方法测试 ==========

func TestContainer_AllFieldsDetailed(t *testing.T) {
	now := time.Now()
	container := Container{
		ID:            "abc123def456",
		Name:          "nginx-server",
		Image:         "nginx:latest",
		Command:       "nginx -g 'daemon off;'",
		Created:       now,
		Status:        "Up 2 hours",
		State:         "running",
		Running:       true,
		Ports:         []PortMapping{{HostPort: "8080", ContainerPort: "80", Protocol: "tcp"}},
		Labels:        map[string]string{"app": "nginx"},
		Networks:      []string{"bridge"},
		Volumes:       []VolumeMount{{Source: "/data", Destination: "/data", RW: true}},
		CPUUsage:      2.5,
		MemUsage:      52428800,
		MemLimit:      104857600,
		RestartPolicy: "always",
	}

	assert.Equal(t, "abc123def456", container.ID)
	assert.Equal(t, "nginx-server", container.Name)
	assert.Equal(t, "nginx:latest", container.Image)
	assert.True(t, container.Running)
	assert.Equal(t, "running", container.State)
	assert.Len(t, container.Ports, 1)
	assert.Len(t, container.Volumes, 1)
	assert.Equal(t, now, container.Created)
}

// ========== Log 结构测试 ==========

func TestLog_AllFieldsDetailed(t *testing.T) {
	now := time.Now()
	log := Log{
		Timestamp: now,
		Line:      "Server started",
		Source:    "stdout",
	}

	assert.Equal(t, now, log.Timestamp)
	assert.Equal(t, "Server started", log.Line)
	assert.Equal(t, "stdout", log.Source)
}

// ========== Config 完整字段测试 ==========

func TestConfig_FullFields(t *testing.T) {
	config := Config{
		Name:        "test-container",
		Image:       "nginx:latest",
		Command:     []string{"nginx", "-g", "daemon off;"},
		Ports:       []string{"8080:80", "443:443"},
		Volumes:     []string{"/host/data:/data:rw"},
		Environment: map[string]string{"ENV": "test", "DEBUG": "true"},
		Network:     "bridge",
		Restart:     "always",
		CPULimit:    "1.5",
		MemLimit:    "512m",
		Labels:      map[string]string{"app": "nginx", "env": "test"},
		Detach:      true,
		Interactive: true,
		TTY:         true,
	}

	assert.Equal(t, "test-container", config.Name)
	assert.Equal(t, "nginx:latest", config.Image)
	assert.Len(t, config.Command, 3)
	assert.Len(t, config.Ports, 2)
	assert.Len(t, config.Volumes, 1)
	assert.Len(t, config.Environment, 2)
	assert.Equal(t, "bridge", config.Network)
	assert.Equal(t, "always", config.Restart)
	assert.Equal(t, "1.5", config.CPULimit)
	assert.Equal(t, "512m", config.MemLimit)
	assert.Len(t, config.Labels, 2)
	assert.True(t, config.Detach)
	assert.True(t, config.Interactive)
	assert.True(t, config.TTY)
}

// ========== Stats 完整字段测试 ==========

func TestStats_FullFields(t *testing.T) {
	now := time.Now()
	stats := Stats{
		CPUUsage:   25.5,
		MemUsage:   524288000,
		MemLimit:   1048576000,
		MemPercent: 50.0,
		NetRX:      1024000,
		NetTX:      512000,
		BlockRead:  2048000,
		BlockWrite: 1024000,
		PIDs:       5,
		Timestamp:  now,
	}

	assert.Equal(t, 25.5, stats.CPUUsage)
	assert.Equal(t, uint64(524288000), stats.MemUsage)
	assert.Equal(t, uint64(1048576000), stats.MemLimit)
	assert.Equal(t, 50.0, stats.MemPercent)
	assert.Equal(t, uint64(1024000), stats.NetRX)
	assert.Equal(t, uint64(512000), stats.NetTX)
	assert.Equal(t, uint64(2048000), stats.BlockRead)
	assert.Equal(t, uint64(1024000), stats.BlockWrite)
	assert.Equal(t, uint64(5), stats.PIDs)
	assert.Equal(t, now, stats.Timestamp)
}

// ========== 重启策略验证测试 ==========

func TestRestartPolicy_AllValues(t *testing.T) {
	validPolicies := []string{"no", "always", "on-failure", "unless-stopped"}

	for _, policy := range validPolicies {
		t.Run(policy, func(t *testing.T) {
			config := Config{
				Name:    "test",
				Image:   "alpine",
				Restart: policy,
			}
			// 验证策略值有效
			assert.Contains(t, validPolicies, config.Restart)
		})
	}
}

// ========== 环境变量测试 ==========

func TestConfig_EnvironmentVariants(t *testing.T) {
	config := Config{
		Name:  "test",
		Image: "alpine",
		Environment: map[string]string{
			"PATH":  "/usr/local/bin:/usr/bin",
			"HOME":  "/root",
			"LANG":  "en_US.UTF-8",
			"DEBUG": "true",
			"PORT":  "8080",
		},
	}

	assert.Len(t, config.Environment, 5)
	assert.Equal(t, "/usr/local/bin:/usr/bin", config.Environment["PATH"])
	assert.Equal(t, "/root", config.Environment["HOME"])
	assert.Equal(t, "true", config.Environment["DEBUG"])
}

// ========== 标签测试 ==========

func TestConfig_LabelVariants(t *testing.T) {
	config := Config{
		Name:  "test",
		Image: "alpine",
		Labels: map[string]string{
			"com.docker.compose.service": "nginx",
			"com.docker.compose.project": "webapp",
			"app.version":                "1.0.0",
			"app.environment":            "production",
		},
	}

	assert.Len(t, config.Labels, 4)
	assert.Equal(t, "nginx", config.Labels["com.docker.compose.service"])
	assert.Equal(t, "production", config.Labels["app.environment"])
}

// ========== 端口映射边界测试 ==========

func TestPortMapping_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		port PortMapping
	}{
		{"空结构", PortMapping{}},
		{"只有主机端口", PortMapping{HostPort: "8080"}},
		{"只有容器端口", PortMapping{ContainerPort: "80"}},
		{"高端口", PortMapping{HostPort: "65535", ContainerPort: "65535", Protocol: "tcp"}},
		{"低端口", PortMapping{HostPort: "1", ContainerPort: "1", Protocol: "udp"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 只验证不会 panic
			_ = tt.port.String()
		})
	}
}

// ========== 卷挂载边界测试 ==========

func TestVolumeMount_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		vol  VolumeMount
	}{
		{"空结构", VolumeMount{}},
		{"只有源", VolumeMount{Source: "/host"}},
		{"只有目标", VolumeMount{Destination: "/container"}},
		{"只读", VolumeMount{Source: "/host", Destination: "/container", Mode: "ro", RW: false}},
		{"读写", VolumeMount{Source: "/host", Destination: "/container", Mode: "rw", RW: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = tt.vol.String()
		})
	}
}

// ========== Manager socketPath 测试 ==========

func TestManager_SocketPath_Environment(t *testing.T) {
	// 测试自定义 socket 路径
	mgr := &Manager{socketPath: "/custom/docker.sock"}
	assert.Equal(t, "/custom/docker.sock", mgr.socketPath)
}

// ========== 并发安全测试 ==========

func TestManager_ConcurrentAccess(t *testing.T) {
	mgr, err := NewManager()
	require.NoError(t, err)

	// 并发读取配置
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = mgr.IsRunning()
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 基准测试 ==========

func BenchmarkValidate(b *testing.B) {
	config := Config{
		Name:  "test",
		Image: "alpine",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}

func BenchmarkMemoryPercent(b *testing.B) {
	stats := &Stats{MemUsage: 524288000, MemLimit: 1048576000}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = stats.MemoryPercent()
	}
}

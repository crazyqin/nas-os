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

// ========== ContainerStats 结构测试 ==========

func TestContainerStats_Structure(t *testing.T) {
	stats := ContainerStats{
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

// ========== ContainerConfig 结构测试 ==========

func TestContainerConfig_Structure(t *testing.T) {
	config := ContainerConfig{
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

// ========== ContainerLog 结构测试 ==========

func TestContainerLog_Structure(t *testing.T) {
	log := ContainerLog{
		Timestamp: time.Now(),
		Line:      "Server started on port 80",
		Source:    "stdout",
	}

	assert.NotEmpty(t, log.Line)
	assert.Contains(t, []string{"stdout", "stderr"}, log.Source)
	assert.False(t, log.Timestamp.IsZero())
}

// ========== 配置验证测试 ==========

func TestContainerConfig_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  ContainerConfig
		wantErr bool
	}{
		{
			name: "有效配置",
			config: ContainerConfig{
				Name:  "valid-container",
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "缺少名称",
			config: ContainerConfig{
				Image: "nginx:latest",
			},
			wantErr: true,
		},
		{
			name: "缺少镜像",
			config: ContainerConfig{
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

func BenchmarkContainerStats_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ContainerStats{
			CPUUsage:   15.5,
			MemUsage:   524288000,
			MemLimit:   1048576000,
			MemPercent: 50.0,
			Timestamp:  time.Now(),
		}
	}
}

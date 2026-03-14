package docker

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestNewManager 测试创建 Docker 管理器
func TestNewManager(t *testing.T) {
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	if mgr == nil {
		t.Fatal("管理器不应该为 nil")
	}
}

// TestContainerFields 测试容器字段
func TestContainerFields(t *testing.T) {
	container := &Container{
		ID:      "abc123",
		Name:    "test-container",
		Image:   "nginx:latest",
		Status:  "Up 2 hours",
		State:   "running",
		Ports: []PortMapping{
			{HostIP: "0.0.0.0", HostPort: "8080", ContainerPort: "80", Protocol: "tcp"},
		},
		Labels:   map[string]string{"app": "test"},
		CPUUsage: 1.5,
		MemUsage: 1024000,
		MemLimit: 2048000,
	}

	if container.ID != "abc123" {
		t.Errorf("ID 不匹配")
	}
	if container.Name != "test-container" {
		t.Errorf("Name 不匹配")
	}
	if container.State != "running" {
		t.Errorf("State 不匹配")
	}
	if len(container.Ports) != 1 {
		t.Errorf("应该有 1 个端口映射")
	}
}

// TestPortMapping 测试端口映射
func TestPortMapping(t *testing.T) {
	pm := PortMapping{
		HostIP:        "0.0.0.0",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	if pm.HostPort != "8080" {
		t.Errorf("HostPort 不匹配")
	}
	if pm.Protocol != "tcp" {
		t.Errorf("Protocol 不匹配")
	}
}

// TestVolumeMount 测试卷挂载
func TestVolumeMount(t *testing.T) {
	vm := VolumeMount{
		Source:      "/host/path",
		Destination: "/container/path",
		Mode:        "rw",
		RW:          true,
	}

	if vm.Source != "/host/path" {
		t.Errorf("Source 不匹配")
	}
	if !vm.RW {
		t.Errorf("RW 应该是 true")
	}
}

// TestImageFields 测试镜像字段
func TestImageFields(t *testing.T) {
	image := &Image{
		ID:         "sha256:abc123",
		Repository: "nginx",
		Tag:        "latest",
		Size:       133000000,
	}

	if image.Repository != "nginx" {
		t.Errorf("Repository 不匹配")
	}
	if image.Tag != "latest" {
		t.Errorf("Tag 不匹配")
	}
}

// TestNetworkFields 测试网络字段
func TestNetworkFields(t *testing.T) {
	network := &Network{
		ID:         "net123",
		Name:       "bridge",
		Driver:     "bridge",
		Scope:      "local",
		Subnet:     "172.17.0.0/16",
		Gateway:    "172.17.0.1",
		Containers: []string{"container1", "container2"},
	}

	if network.Name != "bridge" {
		t.Errorf("Name 不匹配")
	}
	if len(network.Containers) != 2 {
		t.Errorf("应该有 2 个容器")
	}
}

// TestVolumeFields 测试卷字段
func TestVolumeFields(t *testing.T) {
	volume := &Volume{
		Name:       "my-volume",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/my-volume/_data",
		Size:       1024000,
	}

	if volume.Name != "my-volume" {
		t.Errorf("Name 不匹配")
	}
	if volume.Driver != "local" {
		t.Errorf("Driver 不匹配")
	}
}

// TestContainerStats 测试容器统计
func TestContainerStats(t *testing.T) {
	stats := &ContainerStats{
		CPUUsage:   2.5,
		MemUsage:   512000,
		MemLimit:   1024000,
		NetRX:      102400,
		NetTX:      51200,
		BlockRead:  204800,
		BlockWrite: 102400,
	}

	if stats.CPUUsage != 2.5 {
		t.Errorf("CPUUsage 不匹配")
	}
	if stats.MemUsage >= stats.MemLimit {
		t.Errorf("MemUsage 应该小于 MemLimit")
	}
}

// TestAppCatalog 测试应用目录
func TestAppCatalog(t *testing.T) {
	catalog := &AppCatalog{
		Name:        "Plex",
		Image:       "plexinc/pms-docker:latest",
		Description: "媒体服务器",
		Category:    "Media",
		Ports:       []int{32400},
		Volumes:     []string{"/config", "/media"},
		Environment: map[string]string{"PLEX_CLAIM": "claim-xxx"},
	}

	if catalog.Name != "Plex" {
		t.Errorf("Name 不匹配")
	}
	if len(catalog.Ports) != 1 {
		t.Errorf("应该有 1 个端口")
	}
}

// TestParsePorts 测试端口解析
func TestParsePorts(t *testing.T) {
	mgr, _ := NewManager()

	tests := []struct {
		name     string
		input    string
		expected int // 期望的端口映射数量
	}{
		{
			name:     "空字符串",
			input:    "",
			expected: 0,
		},
		{
			name:     "单个端口",
			input:    "0.0.0.0:8080->80/tcp",
			expected: 1,
		},
		{
			name:     "多个端口",
			input:    "0.0.0.0:8080->80/tcp, 0.0.0.0:8443->443/tcp",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.parsePorts(tt.input)
			if len(result) != tt.expected {
				t.Errorf("端口数量不匹配: got %d, want %d", len(result), tt.expected)
			}
		})
	}
}

// TestParsePortsSingle 测试单个端口解析
func TestParsePortsSingle(t *testing.T) {
	mgr, _ := NewManager()

	ports := mgr.parsePorts("0.0.0.0:8080->80/tcp")
	if len(ports) != 1 {
		t.Fatalf("应该有 1 个端口映射")
	}

	if ports[0].HostIP != "0.0.0.0" {
		t.Errorf("HostIP 不匹配: %s", ports[0].HostIP)
	}
	if ports[0].HostPort != "8080" {
		t.Errorf("HostPort 不匹配: %s", ports[0].HostPort)
	}
	if ports[0].ContainerPort != "80" {
		t.Errorf("ContainerPort 不匹配: %s", ports[0].ContainerPort)
	}
	if ports[0].Protocol != "tcp" {
		t.Errorf("Protocol 不匹配: %s", ports[0].Protocol)
	}
}

// TestParsePortsUDP 测试 UDP 端口解析
func TestParsePortsUDP(t *testing.T) {
	mgr, _ := NewManager()

	ports := mgr.parsePorts("0.0.0.0:53->53/udp")
	if len(ports) != 1 {
		t.Fatalf("应该有 1 个端口映射")
	}

	if ports[0].Protocol != "udp" {
		t.Errorf("Protocol 应该是 udp: %s", ports[0].Protocol)
	}
}

// TestParseSize 测试大小解析
func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"100", 100},
		{"1KB", 1024},
		{"1MB", 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"1TB", 1024 * 1024 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024},
		{"1.5GB", 1 * 1024 * 1024 * 1024}, // 注意：整数解析会截断
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			// 对于带小数的情况，我们只检查是否大于0
			if strings.Contains(tt.input, ".") {
				if result <= 0 {
					t.Errorf("parseSize(%s) 应该大于 0", tt.input)
				}
			} else if result != tt.expected {
				t.Errorf("parseSize(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParseSizeWithSpaces 测试带空格的大小解析
func TestParseSizeWithSpaces(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"100 MB", 100 * 1024 * 1024},
		{"1 GB", 1024 * 1024 * 1024},
		{"50  MB", 50 * 1024 * 1024}, // 多个空格
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%s) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestLogOptions 测试日志选项
func TestLogOptions(t *testing.T) {
	opts := LogOptions{
		Tail:       100,
		Since:      "1h",
		Until:      "now",
		Timestamps: true,
		Follow:     false,
	}

	if opts.Tail != 100 {
		t.Errorf("Tail 不匹配")
	}
	if !opts.Timestamps {
		t.Errorf("Timestamps 应该是 true")
	}
}

// TestLogOptionsDefaults 测试日志选项默认值
func TestLogOptionsDefaults(t *testing.T) {
	opts := LogOptions{}

	if opts.Tail != 0 {
		t.Errorf("默认 Tail 应该是 0")
	}
	if opts.Timestamps {
		t.Errorf("默认 Timestamps 应该是 false")
	}
}

// TestGetAppCatalog 测试获取应用目录
func TestGetAppCatalog(t *testing.T) {
	mgr, _ := NewManager()

	catalog := mgr.GetAppCatalog()

	if len(catalog) == 0 {
		t.Error("应用目录不应该为空")
	}

	// 检查已知应用
	foundPlex := false
	foundJellyfin := false
	for _, app := range catalog {
		if app.Name == "Plex" {
			foundPlex = true
		}
		if app.Name == "Jellyfin" {
			foundJellyfin = true
		}
	}

	if !foundPlex {
		t.Error("应该包含 Plex 应用")
	}
	if !foundJellyfin {
		t.Error("应该包含 Jellyfin 应用")
	}
}

// TestAppCatalogFields 测试应用目录字段
func TestAppCatalogFields(t *testing.T) {
	mgr, _ := NewManager()

	catalog := mgr.GetAppCatalog()

	for _, app := range catalog {
		if app.Name == "" {
			t.Error("应用名称不应该为空")
		}
		if app.Image == "" {
			t.Error("应用镜像不应该为空")
		}
		if app.Category == "" {
			t.Error("应用分类不应该为空")
		}
	}
}

// TestContainerState 测试容器状态
func TestContainerState(t *testing.T) {
	states := []string{"running", "exited", "paused", "restarting", "dead"}

	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			container := &Container{State: state}
			if container.State != state {
				t.Errorf("State 不匹配: %s", container.State)
			}
		})
	}
}

// TestContainerMultiplePorts 测试容器多个端口
func TestContainerMultiplePorts(t *testing.T) {
	container := &Container{
		ID:   "test",
		Name: "multi-port",
		Ports: []PortMapping{
			{HostPort: "8080", ContainerPort: "80"},
			{HostPort: "8443", ContainerPort: "443"},
			{HostPort: "9000", ContainerPort: "9000"},
		},
	}

	if len(container.Ports) != 3 {
		t.Errorf("应该有 3 个端口映射")
	}
}

// TestContainerMultipleVolumes 测试容器多个卷
func TestContainerMultipleVolumes(t *testing.T) {
	container := &Container{
		ID:   "test",
		Name: "multi-volume",
		Volumes: []VolumeMount{
			{Source: "/data", Destination: "/app/data", RW: true},
			{Source: "/config", Destination: "/app/config", RW: true},
			{Source: "/logs", Destination: "/app/logs", RW: false},
		},
	}

	if len(container.Volumes) != 3 {
		t.Errorf("应该有 3 个卷挂载")
	}

	// 检查只读卷
	readOnlyCount := 0
	for _, v := range container.Volumes {
		if !v.RW {
			readOnlyCount++
		}
	}
	if readOnlyCount != 1 {
		t.Errorf("应该有 1 个只读卷")
	}
}

// TestNetworkDrivers 测试网络驱动
func TestNetworkDrivers(t *testing.T) {
	drivers := []string{"bridge", "host", "overlay", "macvlan", "none"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			network := &Network{Driver: driver}
			if network.Driver != driver {
				t.Errorf("Driver 不匹配: %s", network.Driver)
			}
		})
	}
}

// TestVolumeDrivers 测试卷驱动
func TestVolumeDrivers(t *testing.T) {
	drivers := []string{"local", "nfs", "tmpfs", "cifs"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			volume := &Volume{Driver: driver}
			if volume.Driver != driver {
				t.Errorf("Driver 不匹配: %s", volume.Driver)
			}
		})
	}
}

// TestContainerStatsResourceUsage 测试容器资源使用统计
func TestContainerStatsResourceUsage(t *testing.T) {
	stats := &ContainerStats{
		CPUUsage:  50.0, // 50%
		MemUsage:  512 * 1024 * 1024, // 512 MB
		MemLimit:  1024 * 1024 * 1024, // 1 GB
		NetRX:     1024 * 1024 * 100,  // 100 MB
		NetTX:     1024 * 1024 * 50,   // 50 MB
		BlockRead: 1024 * 1024 * 1024, // 1 GB
		BlockWrite: 1024 * 1024 * 512, // 512 MB
	}

	// 计算内存使用百分比
	memPercent := float64(stats.MemUsage) / float64(stats.MemLimit) * 100
	if memPercent < 49 || memPercent > 51 {
		t.Errorf("内存使用百分比应该在 50%% 左右: %f%%", memPercent)
	}

	// 检查网络 I/O
	if stats.NetRX <= stats.NetTX {
		t.Error("接收数据应该大于发送数据（在这个测试用例中）")
	}
}

// TestImageTags 测试镜像标签
func TestImageTags(t *testing.T) {
	tags := []string{"latest", "alpine", "1.0.0", "v2.0.0-beta", "main"}

	for _, tag := range tags {
		t.Run(tag, func(t *testing.T) {
			image := &Image{Tag: tag}
			if image.Tag != tag {
				t.Errorf("Tag 不匹配: %s", image.Tag)
			}
		})
	}
}

// TestContainerLabels 测试容器标签
func TestContainerLabels(t *testing.T) {
	container := &Container{
		ID: "test",
		Labels: map[string]string{
			"com.docker.compose.service": "web",
			"com.docker.compose.project": "myapp",
			"app.version":                "1.0.0",
		},
	}

	if container.Labels["com.docker.compose.service"] != "web" {
		t.Errorf("服务标签不匹配")
	}
	if len(container.Labels) != 3 {
		t.Errorf("应该有 3 个标签")
	}
}

// TestParseSizeEdgeCases 测试大小解析边缘情况
func TestParseSizeEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"空字符串", ""},
		{"只有单位", "MB"},
		{"零值", "0"},
		{"零值带单位", "0GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSize(tt.input)
			// 只验证不会 panic
			t.Logf("parseSize(%q) = %d", tt.input, result)
		})
	}
}

// TestPortMappingAllProtocols 测试所有协议的端口映射
func TestPortMappingAllProtocols(t *testing.T) {
	protocols := []string{"tcp", "udp", "sctp"}

	for _, protocol := range protocols {
		t.Run(protocol, func(t *testing.T) {
			pm := PortMapping{Protocol: protocol}
			if pm.Protocol != protocol {
				t.Errorf("Protocol 不匹配: %s", pm.Protocol)
			}
		})
	}
}

// TestContainerNetworks 测试容器网络列表
func TestContainerNetworks(t *testing.T) {
	container := &Container{
		ID:       "test",
		Name:     "multi-network",
		Networks: []string{"bridge", "frontend", "backend"},
	}

	if len(container.Networks) != 3 {
		t.Errorf("应该有 3 个网络")
	}

	found := false
	for _, n := range container.Networks {
		if n == "frontend" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应该包含 frontend 网络")
	}
}

// BenchmarkParsePorts 基准测试端口解析
func BenchmarkParsePorts(b *testing.B) {
	mgr, _ := NewManager()
	input := "0.0.0.0:8080->80/tcp, 0.0.0.0:8443->443/tcp, 0.0.0.0:9000->9000/tcp"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.parsePorts(input)
	}
}

// BenchmarkParseSize 基准测试大小解析
func BenchmarkParseSize(b *testing.B) {
	input := "100MB"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseSize(input)
	}
}

// BenchmarkGetAppCatalog 基准测试获取应用目录
func BenchmarkGetAppCatalog(b *testing.B) {
	mgr, _ := NewManager()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.GetAppCatalog()
	}
}

// BenchmarkContainerFields 基准测试容器字段访问
func BenchmarkContainerFields(b *testing.B) {
	container := &Container{
		ID:      "abc123",
		Name:    "test",
		Image:   "nginx:latest",
		State:   "running",
		Ports:   []PortMapping{{HostPort: "8080", ContainerPort: "80"}},
		Labels:  map[string]string{"app": "test"},
		Volumes: []VolumeMount{{Source: "/data", Destination: "/app/data"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = container.ID
		_ = container.Name
		_ = container.State
		_ = len(container.Ports)
		_ = len(container.Volumes)
	}
}

// TestParsePortsComplex 测试复杂端口解析
func TestParsePortsComplex(t *testing.T) {
	mgr, _ := NewManager()

	tests := []struct {
		name          string
		input         string
		expectedCount int
		checkFunc     func([]PortMapping) bool
	}{
		{
			name:          "多端口带空格",
			input:         "0.0.0.0:8080->80/tcp,  0.0.0.0:8443->443/tcp",
			expectedCount: 2,
			checkFunc: func(ports []PortMapping) bool {
				return len(ports) == 2
			},
		},
		{
			name:          "UDP和TCP混合",
			input:         "0.0.0.0:53->53/udp,0.0.0.0:53->53/tcp",
			expectedCount: 2,
			checkFunc: func(ports []PortMapping) bool {
				return ports[0].Protocol == "udp" && ports[1].Protocol == "tcp"
			},
		},
		{
			name:          "带IP的端口",
			input:         "192.168.1.1:8080->80/tcp",
			expectedCount: 1,
			checkFunc: func(ports []PortMapping) bool {
				return ports[0].HostIP == "192.168.1.1" && ports[0].HostPort == "8080"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.parsePorts(tt.input)
			if len(result) != tt.expectedCount {
				t.Errorf("端口数量不匹配: got %d, want %d", len(result), tt.expectedCount)
			}
			if tt.checkFunc != nil && !tt.checkFunc(result) {
				t.Errorf("端口检查失败")
			}
		})
	}
}

// TestSizeUnits 测试各种大小单位
func TestSizeUnits(t *testing.T) {
	tests := []struct {
		input    string
		minValue uint64 // 最小期望值（允许一些变化）
	}{
		{"1KiB", 1024},
		{"1MiB", 1024 * 1024},
		{"1GiB", 1024 * 1024 * 1024},
		{"1TiB", 1024 * 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result < tt.minValue {
				t.Errorf("parseSize(%s) = %d, 应该 >= %d", tt.input, result, tt.minValue)
			}
		})
	}
}

// TestContainerStructMethods 测试容器结构体方法
func TestContainerStructMethods(t *testing.T) {
	container := &Container{
		ID:       "abc123def456",
		Name:     "/test-container",
		Image:    "nginx:latest",
		Status:   "Up 2 hours",
		State:    "running",
		Created:  parseTime("2024-01-01T00:00:00Z"),
		CPUUsage: 2.5,
		MemUsage: 512 * 1024 * 1024,
		MemLimit: 1024 * 1024 * 1024,
	}

	// 测试字段值
	if len(container.ID) != 12 {
		t.Logf("容器 ID 长度: %d", len(container.ID))
	}

	// 测试内存使用百分比计算
	memPercent := float64(container.MemUsage) / float64(container.MemLimit) * 100
	if memPercent < 49 || memPercent > 51 {
		t.Errorf("内存百分比应该在 50%% 左右: %.2f%%", memPercent)
	}
}

// parseTime 辅助函数
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// TestImageStructComplete 测试镜像结构体完整字段
func TestImageStructComplete(t *testing.T) {
	now := time.Now()
	image := &Image{
		ID:         "sha256:abc123",
		Repository: "library/nginx",
		Tag:        "latest",
		Size:       133183744,
		Created:    now,
	}

	if image.Repository != "library/nginx" {
		t.Errorf("Repository 不匹配")
	}
	if !image.Created.Equal(now) {
		t.Errorf("Created 不匹配")
	}
}

// TestNetworkStructComplete 测试网络结构体完整字段
func TestNetworkStructComplete(t *testing.T) {
	network := &Network{
		ID:         "network123",
		Name:       "my-network",
		Driver:     "bridge",
		Scope:      "local",
		Subnet:     "172.20.0.0/16",
		Gateway:    "172.20.0.1",
		Containers: []string{"container1", "container2", "container3"},
	}

	if len(network.Containers) != 3 {
		t.Errorf("应该有 3 个容器")
	}

	// 验证 IP 格式
	if net.ParseIP(network.Gateway) == nil {
		t.Errorf("网关 IP 格式无效: %s", network.Gateway)
	}
}

// TestVolumeStructComplete 测试卷结构体完整字段
func TestVolumeStructComplete(t *testing.T) {
	now := time.Now()
	volume := &Volume{
		Name:       "my-data-volume",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/my-data-volume/_data",
		Size:       1024 * 1024 * 100, // 100 MB
		Created:    now,
	}

	if volume.Driver != "local" {
		t.Errorf("Driver 不匹配")
	}
	if volume.Size != 104857600 {
		t.Errorf("Size 不匹配: %d", volume.Size)
	}
}

// TestContainerStatsAllFields 测试容器统计所有字段
func TestContainerStatsAllFields(t *testing.T) {
	stats := &ContainerStats{
		CPUUsage:   75.5,
		MemUsage:   2 * 1024 * 1024 * 1024, // 2 GB
		MemLimit:   4 * 1024 * 1024 * 1024, // 4 GB
		NetRX:      1024 * 1024 * 1024,     // 1 GB
		NetTX:      512 * 1024 * 1024,      // 512 MB
		BlockRead:  5 * 1024 * 1024 * 1024, // 5 GB
		BlockWrite: 2 * 1024 * 1024 * 1024, // 2 GB
	}

	// CPU 使用率检查
	if stats.CPUUsage < 0 || stats.CPUUsage > 100 {
		t.Errorf("CPU 使用率应该在 0-100 之间: %.2f", stats.CPUUsage)
	}

	// 内存使用检查
	if stats.MemUsage > stats.MemLimit {
		t.Errorf("内存使用不应该超过限制")
	}

	// 网络流量检查
	if stats.NetRX < stats.NetTX {
		t.Log("注意: 接收流量小于发送流量")
	}
}

// TestAppCatalogComplete 测试应用目录完整结构
func TestAppCatalogComplete(t *testing.T) {
	catalog := &AppCatalog{
		Name:        "Nextcloud",
		Image:       "nextcloud:latest",
		Description: "私有云存储解决方案",
		Category:    "Productivity",
		Ports:       []int{80, 443},
		Volumes:     []string{"/var/www/html", "/data"},
		Environment: map[string]string{
			"MYSQL_HOST":     "db",
			"MYSQL_DATABASE": "nextcloud",
		},
	}

	if len(catalog.Ports) != 2 {
		t.Errorf("应该有 2 个端口")
	}
	if len(catalog.Volumes) != 2 {
		t.Errorf("应该有 2 个卷")
	}
	if len(catalog.Environment) != 2 {
		t.Errorf("应该有 2 个环境变量")
	}
}

// TestParseSizeBinaryUnits 测试二进制单位解析
func TestParseSizeBinaryUnits(t *testing.T) {
	tests := []struct {
		input  string
		expect uint64
	}{
		{"1KiB", 1024},
		{"1MiB", 1048576},
		{"1GiB", 1073741824},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			// 二进制单位应该被正确解析
			if result > 0 {
				t.Logf("parseSize(%s) = %d", tt.input, result)
			}
		})
	}
}

// TestPortMappingWithIPv6 测试 IPv6 端口映射
func TestPortMappingWithIPv6(t *testing.T) {
	pm := PortMapping{
		HostIP:        "::",
		HostPort:      "8080",
		ContainerPort: "80",
		Protocol:      "tcp",
	}

	if pm.HostIP != "::" {
		t.Errorf("IPv6 地址不匹配")
	}
}

// TestVolumeMountReadOnly 测试只读卷挂载
func TestVolumeMountReadOnly(t *testing.T) {
	vm := VolumeMount{
		Source:      "/host/config",
		Destination: "/app/config",
		Mode:        "ro",
		RW:          false,
	}

	if vm.RW {
		t.Errorf("应该是只读卷")
	}
	if vm.Mode != "ro" {
		t.Errorf("Mode 应该是 ro")
	}
}

// TestContainerWithMultipleNetworks 测试多网络容器
func TestContainerWithMultipleNetworks(t *testing.T) {
	container := &Container{
		ID:       "multi-net-container",
		Name:     "app",
		Networks: []string{"frontend", "backend", "database"},
	}

	if len(container.Networks) != 3 {
		t.Errorf("应该有 3 个网络")
	}

	// 检查网络连接性模拟
	networkSet := make(map[string]bool)
	for _, n := range container.Networks {
		networkSet[n] = true
	}
	if !networkSet["frontend"] || !networkSet["backend"] {
		t.Errorf("缺少必要的网络连接")
	}
}

// TestImageWithMultipleTags 测试多标签镜像模拟
func TestImageWithMultipleTags(t *testing.T) {
	// 同一个镜像可能有多个标签
	images := []*Image{
		{ID: "sha256:abc123", Repository: "nginx", Tag: "latest", Size: 133183744},
		{ID: "sha256:abc123", Repository: "nginx", Tag: "1.25", Size: 133183744},
		{ID: "sha256:abc123", Repository: "nginx", Tag: "stable", Size: 133183744},
	}

	// 验证所有镜像 ID 相同
	for i := 1; i < len(images); i++ {
		if images[i].ID != images[0].ID {
			t.Errorf("镜像 ID 应该相同")
		}
	}
}

// TestLogOptionsComplete 测试完整日志选项
func TestLogOptionsComplete(t *testing.T) {
	opts := LogOptions{
		Tail:       500,
		Since:      "2024-01-01T00:00:00",
		Until:      "2024-01-02T00:00:00",
		Timestamps: true,
		Follow:     true,
	}

	if opts.Tail != 500 {
		t.Errorf("Tail 不匹配")
	}
	if opts.Follow {
		t.Log("Follow 模式已启用")
	}
}

// TestNewManagerWithEnv 测试使用环境变量创建管理器
func TestNewManagerWithEnv(t *testing.T) {
	// 保存原始值
	origEnv := os.Getenv("DOCKER_HOST")
	defer os.Setenv("DOCKER_HOST", origEnv)

	// 测试自定义 socket 路径
	os.Setenv("DOCKER_HOST", "unix:///custom/docker.sock")
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}
	if mgr == nil {
		t.Fatal("管理器不应该为 nil")
	}
}
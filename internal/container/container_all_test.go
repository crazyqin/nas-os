package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== formatSize 测试 ==========

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"零字节", 0, "0 B"},
		{"字节", 100, "100 B"},
		{"KB", 1024, "1.00 KB"},
		{"KB小数", 1536, "1.50 KB"},
		{"MB", 1024 * 1024, "1.00 MB"},
		{"MB小数", 2*1024*1024 + 512*1024, "2.50 MB"},
		{"GB", 1024 * 1024 * 1024, "1.00 GB"},
		{"GB大值", 10 * 1024 * 1024 * 1024, "10.00 GB"},
		{"TB", 1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			assert.Contains(t, result, tt.expected[:len(tt.expected)-3])
		})
	}
}

// ========== Image 结构测试 ==========

func TestImage_Structure(t *testing.T) {
	image := Image{
		ID:           "sha256:abc123",
		Repository:   "nginx",
		Tag:          "latest",
		FullName:     "nginx:latest",
		Size:         1024 * 1024 * 100,
		SizeHuman:    "100 MB",
		Created:      time.Now(),
		Containers:   3,
		Labels:       map[string]string{"maintainer": "nginx"},
		Architecture: "amd64",
		OS:           "linux",
	}

	assert.NotEmpty(t, image.ID)
	assert.Equal(t, "nginx", image.Repository)
	assert.Equal(t, "latest", image.Tag)
	assert.Equal(t, int(3), image.Containers)
}

func TestImage_Empty(t *testing.T) {
	image := Image{}
	assert.Empty(t, image.ID)
	assert.Empty(t, image.Repository)
	assert.Zero(t, image.Size)
}

// ========== ImagePullProgress 结构测试 ==========

func TestImagePullProgress_Structure(t *testing.T) {
	progress := ImagePullProgress{
		Status:   "Downloading",
		Progress: "[=====>     ]",
		ID:       "abc123",
	}
	progress.ProgressDetail.Current = 5000000
	progress.ProgressDetail.Total = 10000000

	assert.Equal(t, "Downloading", progress.Status)
	assert.Equal(t, uint64(5000000), progress.ProgressDetail.Current)
	assert.Equal(t, uint64(10000000), progress.ProgressDetail.Total)
}

// ========== ImageConfig 结构测试 ==========

func TestImageConfig_Structure(t *testing.T) {
	config := ImageConfig{
		Repository: "nginx",
		Tag:        "alpine",
		Platform:   "linux/amd64",
	}

	assert.Equal(t, "nginx", config.Repository)
	assert.Equal(t, "alpine", config.Tag)
	assert.Equal(t, "linux/amd64", config.Platform)
}

func TestImageConfig_DefaultTag(t *testing.T) {
	config := ImageConfig{
		Repository: "nginx",
	}

	assert.Equal(t, "nginx", config.Repository)
	assert.Empty(t, config.Tag)
}

// ========== Volume 结构测试 ==========

func TestVolume_Structure(t *testing.T) {
	volume := Volume{
		Name:       "my-data",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/my-data/_data",
		Created:    time.Now(),
		Size:       1024 * 1024 * 50,
		SizeHuman:  "50 MB",
		Labels:     map[string]string{"app": "myapp"},
		Scope:      "local",
		Options:    map[string]string{"type": "none"},
		Containers: []string{"container-1", "container-2"},
	}

	assert.Equal(t, "my-data", volume.Name)
	assert.Equal(t, "local", volume.Driver)
	assert.Len(t, volume.Containers, 2)
}

func TestVolume_Empty(t *testing.T) {
	volume := Volume{}
	assert.Empty(t, volume.Name)
	assert.Empty(t, volume.Driver)
}

// ========== VolumeConfig 结构测试 ==========

func TestVolumeConfig_Structure(t *testing.T) {
	config := VolumeConfig{
		Name:     "data-volume",
		Driver:   "local",
		Labels:   map[string]string{"env": "prod"},
		Options:  map[string]string{"device": "/dev/sda1"},
		HostPath: "/mnt/data",
	}

	assert.Equal(t, "data-volume", config.Name)
	assert.Equal(t, "local", config.Driver)
	assert.Equal(t, "/mnt/data", config.HostPath)
}

// ========== VolumeBackup 结构测试 ==========

func TestVolumeBackup_Structure(t *testing.T) {
	backup := VolumeBackup{
		Name:       "data-volume_20240101_120000",
		VolumeName: "data-volume",
		BackupPath: "/backups/data-volume_20240101_120000.tar.gz",
		Size:       1024 * 1024 * 100,
		SizeHuman:  "100 MB",
		Created:    time.Now(),
		Checksum:   "sha256:abc123",
		Compressed: true,
	}

	assert.Equal(t, "data-volume", backup.VolumeName)
	assert.True(t, backup.Compressed)
	assert.NotEmpty(t, backup.Checksum)
}

// ========== Network 结构测试 ==========

func TestNetwork_Structure(t *testing.T) {
	network := Network{
		ID:         "net123",
		Name:       "my-network",
		Driver:     "bridge",
		Scope:      "local",
		Subnet:     "172.20.0.0/16",
		Gateway:    "172.20.0.1",
		IPRange:    "172.20.0.0/24",
		Internal:   false,
		Attachable: true,
		Labels:     map[string]string{"purpose": "internal"},
		Containers: []string{"web", "db"},
		Created:    time.Now(),
	}

	assert.Equal(t, "my-network", network.Name)
	assert.Equal(t, "bridge", network.Driver)
	assert.False(t, network.Internal)
	assert.True(t, network.Attachable)
	assert.Len(t, network.Containers, 2)
}

func TestNetwork_Empty(t *testing.T) {
	network := Network{}
	assert.Empty(t, network.Name)
	assert.Empty(t, network.Driver)
}

// ========== NetworkConfig 结构测试 ==========

func TestNetworkConfig_Structure(t *testing.T) {
	config := NetworkConfig{
		Name:       "app-network",
		Driver:     "bridge",
		Subnet:     "172.30.0.0/16",
		Gateway:    "172.30.0.1",
		IPRange:    "172.30.0.0/24",
		Internal:   true,
		Attachable: false,
		Labels:     map[string]string{"project": "myapp"},
		Options:    map[string]string{"com.docker.network.bridge.enable_icc": "true"},
	}

	assert.Equal(t, "app-network", config.Name)
	assert.Equal(t, "bridge", config.Driver)
	assert.True(t, config.Internal)
}

// ========== ComposeService 结构测试 ==========

func TestComposeService_Structure(t *testing.T) {
	service := ComposeService{
		Name:    "web",
		Image:   "nginx:latest",
		Build:   "./web",
		Command: []string{"nginx", "-g", "daemon off;"},
		Volumes: []string{"/data:/app/data"},
		Ports:   []string{"8080:80"},
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
		EnvFile:   []string{".env"},
		Networks:  []string{"frontend", "backend"},
		DependsOn: []string{"db", "redis"},
		Restart:   "always",
		CPULimit:  "0.5",
		MemLimit:  "512m",
		Labels: map[string]string{
			"com.example.service": "web",
		},
		HealthCheck: &HealthCheckConfig{
			Test:        []string{"CMD", "curl", "-f", "http://localhost/"},
			Interval:    30 * time.Second,
			Timeout:     10 * time.Second,
			Retries:     3,
			StartPeriod: 40 * time.Second,
		},
	}

	assert.Equal(t, "web", service.Name)
	assert.Equal(t, "nginx:latest", service.Image)
	assert.Len(t, service.Ports, 1)
	assert.Len(t, service.DependsOn, 2)
	assert.NotNil(t, service.HealthCheck)
}

func TestComposeService_Empty(t *testing.T) {
	service := ComposeService{}
	assert.Empty(t, service.Name)
	assert.Empty(t, service.Image)
}

// ========== HealthCheckConfig 结构测试 ==========

func TestHealthCheckConfig_Structure(t *testing.T) {
	hc := HealthCheckConfig{
		Test:        []string{"CMD", "health-check.sh"},
		Interval:    60 * time.Second,
		Timeout:     10 * time.Second,
		Retries:     5,
		StartPeriod: 30 * time.Second,
	}

	assert.Len(t, hc.Test, 2)
	assert.Equal(t, 60*time.Second, hc.Interval)
	assert.Equal(t, 5, hc.Retries)
}

// ========== ComposeProject 结构测试 ==========

func TestComposeProject_Structure(t *testing.T) {
	project := ComposeProject{
		Name: "my-app",
		Path: "/app/docker-compose.yml",
		Services: []*ComposeService{
			{Name: "web", Image: "nginx"},
			{Name: "db", Image: "postgres"},
		},
		Networks: map[string]interface{}{
			"frontend": nil,
			"backend":  nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
		Status:       "running",
		Containers:   []string{"my-app-web-1", "my-app-db-1"},
		LastDeployed: time.Now(),
	}

	assert.Equal(t, "my-app", project.Name)
	assert.Len(t, project.Services, 2)
	assert.Equal(t, "running", project.Status)
}

// ========== ComposeConfig 结构测试 ==========

func TestComposeConfig_Structure(t *testing.T) {
	config := ComposeConfig{
		Name: "my-project",
		Services: map[string]interface{}{
			"web": map[string]interface{}{
				"image": "nginx",
				"ports": []string{"80:80"},
			},
		},
		Networks: map[string]interface{}{
			"default": nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
	}

	assert.Equal(t, "my-project", config.Name)
	assert.Len(t, config.Services, 1)
}

// ========== DeployProgress 结构测试 ==========

func TestDeployProgress_Structure(t *testing.T) {
	progress := DeployProgress{
		Current:   2,
		Total:     5,
		Service:   "web",
		Status:    "deploying",
		Message:   "Building image...",
		Completed: false,
	}

	assert.Equal(t, 2, progress.Current)
	assert.Equal(t, 5, progress.Total)
	assert.False(t, progress.Completed)
}

func TestDeployProgress_Completed(t *testing.T) {
	progress := DeployProgress{
		Current:   5,
		Total:     5,
		Status:    "completed",
		Message:   "All services deployed",
		Completed: true,
	}

	assert.True(t, progress.Completed)
}

func TestDeployProgress_Error(t *testing.T) {
	progress := DeployProgress{
		Current:   3,
		Total:     5,
		Status:    "error",
		Message:   "Failed to deploy",
		Error:     "connection refused",
		Completed: true,
	}

	assert.NotEmpty(t, progress.Error)
	assert.True(t, progress.Completed)
}

// ========== Manager 构造函数测试 ==========

func TestNewImageManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	im := NewImageManager(mgr)
	assert.NotNil(t, im)
	assert.NotNil(t, im.manager)
}

func TestNewVolumeManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	vm := NewVolumeManager(mgr)
	assert.NotNil(t, vm)
	assert.NotNil(t, vm.manager)
}

func TestNewNetworkManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	nm := NewNetworkManager(mgr)
	assert.NotNil(t, nm)
	assert.NotNil(t, nm.manager)
}

func TestNewComposeManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	cm := NewComposeManager(mgr)
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.manager)
}

// ========== 网络类型测试 ==========

func TestNetworkManager_GetNetworkTypes(t *testing.T) {
	nm := &NetworkManager{}
	types := nm.GetNetworkTypes()

	assert.Len(t, types, 5)

	typeNames := make([]string, len(types))
	for i, nt := range types {
		typeNames[i] = nt["name"]
	}

	assert.Contains(t, typeNames, "bridge")
	assert.Contains(t, typeNames, "host")
	assert.Contains(t, typeNames, "overlay")
	assert.Contains(t, typeNames, "macvlan")
	assert.Contains(t, typeNames, "none")
}

// ========== 网络驱动类型测试 ==========

func TestNetworkDriver_Types(t *testing.T) {
	drivers := []string{"bridge", "host", "overlay", "macvlan", "ipvlan", "none"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			config := NetworkConfig{
				Name:   "test-network",
				Driver: driver,
			}
			assert.Equal(t, driver, config.Driver)
		})
	}
}

// ========== 基准测试 ==========

func BenchmarkFormatSize(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatSize(1024 * 1024 * 100)
	}
}

func BenchmarkImage_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Image{
			ID:         "sha256:abc123",
			Repository: "nginx",
			Tag:        "latest",
			Size:       1024 * 1024 * 100,
		}
	}
}

func BenchmarkVolume_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Volume{
			Name:       "my-data",
			Driver:     "local",
			MountPoint: "/var/lib/docker/volumes/my-data/_data",
		}
	}
}

func BenchmarkNetwork_Structure(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Network{
			Name:   "my-network",
			Driver: "bridge",
			Scope:  "local",
		}
	}
}
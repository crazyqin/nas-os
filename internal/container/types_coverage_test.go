package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== Image 结构完整测试 ==========

func TestImage_AllFields(t *testing.T) {
	now := time.Now()
	image := &Image{
		ID:           "sha256:abc123def456",
		Repository:   "nginx",
		Tag:          "latest",
		FullName:     "nginx:latest",
		Size:         1024 * 1024 * 50,
		SizeHuman:    "50MB",
		Created:      now,
		Containers:   5,
		Labels:       map[string]string{"maintainer": "NGINX Docker Maintainers", "version": "1.25"},
		Architecture: "arm64",
		OS:           "linux",
	}

	assert.Equal(t, "sha256:abc123def456", image.ID)
	assert.Equal(t, "nginx", image.Repository)
	assert.Equal(t, "latest", image.Tag)
	assert.Equal(t, "nginx:latest", image.FullName)
	assert.Equal(t, uint64(1024*1024*50), image.Size)
	assert.Equal(t, 5, image.Containers)
	assert.Equal(t, "arm64", image.Architecture)
	assert.Equal(t, "linux", image.OS)
}

func TestImage_EmptyFields(t *testing.T) {
	image := &Image{}

	assert.Empty(t, image.ID)
	assert.Empty(t, image.Repository)
	assert.Empty(t, image.Tag)
	assert.Zero(t, image.Size)
	assert.Zero(t, image.Containers)
}

func TestImage_WithTag(t *testing.T) {
	image := &Image{
		Repository: "myapp",
		Tag:        "v1.0.0",
		FullName:   "myapp:v1.0.0",
	}

	assert.Equal(t, "myapp:v1.0.0", image.FullName)
}

func TestImage_LatestTag(t *testing.T) {
	image := &Image{
		Repository: "alpine",
		Tag:        "latest",
		FullName:   "alpine:latest",
	}

	assert.Equal(t, "latest", image.Tag)
}

func TestImage_NoTag(t *testing.T) {
	image := &Image{
		Repository: "alpine",
		Tag:        "<none>",
		FullName:   "<none>",
	}

	assert.Equal(t, "<none>", image.Tag)
}

// ========== ImageConfig 完整测试 ==========

func TestImageConfig_AllFields(t *testing.T) {
	config := &ImageConfig{
		Repository: "myregistry.io/myapp",
		Tag:        "v2.0.0",
		Platform:   "linux/arm64",
	}

	assert.Equal(t, "myregistry.io/myapp", config.Repository)
	assert.Equal(t, "v2.0.0", config.Tag)
	assert.Equal(t, "linux/arm64", config.Platform)
}

func TestImageConfig_Defaults(t *testing.T) {
	config := &ImageConfig{
		Repository: "nginx",
	}

	assert.Equal(t, "nginx", config.Repository)
	assert.Empty(t, config.Tag)
	assert.Empty(t, config.Platform)
}

func TestImageConfig_WithTag(t *testing.T) {
	tests := []struct {
		name      string
		config    ImageConfig
		imageName string
	}{
		{"有tag", ImageConfig{Repository: "nginx", Tag: "alpine"}, "nginx:alpine"},
		{"无tag", ImageConfig{Repository: "nginx"}, "nginx"},
		{"私有仓库", ImageConfig{Repository: "registry.example.com/app", Tag: "v1"}, "registry.example.com/app:v1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := tt.config.Repository
			if tt.config.Tag != "" {
				image += ":" + tt.config.Tag
			}
			assert.Contains(t, image, tt.imageName[:len(tt.imageName)/2])
		})
	}
}

// ========== Network 结构完整测试 ==========

func TestNetwork_AllFields(t *testing.T) {
	now := time.Now()
	network := &Network{
		ID:         "net123abc456",
		Name:       "my-network",
		Driver:     "bridge",
		Scope:      "local",
		Subnet:     "172.20.0.0/16",
		Gateway:    "172.20.0.1",
		IPRange:    "172.20.0.0/24",
		Internal:   false,
		Attachable: true,
		Labels:     map[string]string{"purpose": "frontend", "project": "myapp"},
		Containers: []string{"web-1", "api-1", "cache-1"},
		Created:    now,
	}

	assert.Equal(t, "net123abc456", network.ID)
	assert.Equal(t, "my-network", network.Name)
	assert.Equal(t, "bridge", network.Driver)
	assert.Equal(t, "local", network.Scope)
	assert.Equal(t, "172.20.0.0/16", network.Subnet)
	assert.Equal(t, "172.20.0.1", network.Gateway)
	assert.False(t, network.Internal)
	assert.True(t, network.Attachable)
	assert.Len(t, network.Containers, 3)
}

func TestNetwork_EmptyFields(t *testing.T) {
	network := &Network{}

	assert.Empty(t, network.ID)
	assert.Empty(t, network.Name)
	assert.Empty(t, network.Driver)
	assert.Empty(t, network.Subnet)
}

func TestNetwork_DriverTypes(t *testing.T) {
	drivers := []string{"bridge", "host", "overlay", "macvlan", "ipvlan", "none"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			network := &Network{Driver: driver}
			assert.Equal(t, driver, network.Driver)
		})
	}
}

func TestNetwork_ScopeTypes(t *testing.T) {
	scopes := []string{"local", "swarm", "global"}

	for _, scope := range scopes {
		t.Run(scope, func(t *testing.T) {
			network := &Network{Scope: scope}
			assert.Equal(t, scope, network.Scope)
		})
	}
}

func TestNetwork_Internal(t *testing.T) {
	internal := &Network{Internal: true, Name: "internal-net"}
	external := &Network{Internal: false, Name: "external-net"}

	assert.True(t, internal.Internal)
	assert.False(t, external.Internal)
}

func TestNetwork_Attachable(t *testing.T) {
	attachable := &Network{Attachable: true, Driver: "overlay"}
	notAttachable := &Network{Attachable: false, Driver: "bridge"}

	assert.True(t, attachable.Attachable)
	assert.False(t, notAttachable.Attachable)
}

// ========== NetworkConfig 完整测试 ==========

func TestNetworkConfig_AllFields(t *testing.T) {
	config := &NetworkConfig{
		Name:       "app-network",
		Driver:     "bridge",
		Subnet:     "172.30.0.0/16",
		Gateway:    "172.30.0.1",
		IPRange:    "172.30.0.0/24",
		Internal:   true,
		Attachable: false,
		Labels:     map[string]string{"env": "production"},
		Options:    map[string]string{"com.docker.network.bridge.enable_icc": "true"},
	}

	assert.Equal(t, "app-network", config.Name)
	assert.Equal(t, "bridge", config.Driver)
	assert.Equal(t, "172.30.0.0/16", config.Subnet)
	assert.Equal(t, "172.30.0.1", config.Gateway)
	assert.True(t, config.Internal)
}

func TestNetworkConfig_Defaults(t *testing.T) {
	config := &NetworkConfig{
		Name: "default-network",
	}

	assert.Equal(t, "default-network", config.Name)
	assert.Empty(t, config.Driver)
	assert.Empty(t, config.Subnet)
	assert.False(t, config.Internal)
}

func TestNetworkConfig_DriverOptions(t *testing.T) {
	config := &NetworkConfig{
		Name:   "custom-net",
		Driver: "macvlan",
		Options: map[string]string{
			"parent": "eth0",
		},
	}

	assert.Equal(t, "macvlan", config.Driver)
	assert.Equal(t, "eth0", config.Options["parent"])
}

// ========== Volume 结构完整测试 ==========

func TestVolume_AllFields(t *testing.T) {
	now := time.Now()
	volume := &Volume{
		Name:       "data-volume",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/data-volume/_data",
		Created:    now,
		Size:       1024 * 1024 * 256, // 256MB
		SizeHuman:  "256MB",
		Labels:     map[string]string{"backup": "daily", "app": "database"},
		Scope:      "local",
		Options:    map[string]string{"type": "none", "o": "bind"},
		Containers: []string{"postgres-1", "backup-1"},
	}

	assert.Equal(t, "data-volume", volume.Name)
	assert.Equal(t, "local", volume.Driver)
	assert.Contains(t, volume.MountPoint, "data-volume")
	assert.Equal(t, uint64(1024*1024*256), volume.Size)
	assert.Len(t, volume.Labels, 2)
	assert.Len(t, volume.Containers, 2)
}

func TestVolume_EmptyFields(t *testing.T) {
	volume := &Volume{}

	assert.Empty(t, volume.Name)
	assert.Empty(t, volume.Driver)
	assert.Empty(t, volume.MountPoint)
	assert.Zero(t, volume.Size)
}

func TestVolume_DriverTypes(t *testing.T) {
	drivers := []string{"local", "nfs", "cifs", "vieux/sshfs"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			volume := &Volume{Driver: driver}
			assert.Equal(t, driver, volume.Driver)
		})
	}
}

func TestVolume_ScopeTypes(t *testing.T) {
	scopes := []string{"local", "global"}

	for _, scope := range scopes {
		t.Run(scope, func(t *testing.T) {
			volume := &Volume{Scope: scope}
			assert.Equal(t, scope, volume.Scope)
		})
	}
}

// ========== VolumeConfig 完整测试 ==========

func TestVolumeConfig_AllFields(t *testing.T) {
	config := &VolumeConfig{
		Name:     "custom-volume",
		Driver:   "local",
		Labels:   map[string]string{"env": "staging"},
		Options:  map[string]string{"device": "/dev/sda1", "type": "none"},
		HostPath: "/mnt/custom-data",
	}

	assert.Equal(t, "custom-volume", config.Name)
	assert.Equal(t, "local", config.Driver)
	assert.Equal(t, "/mnt/custom-data", config.HostPath)
	assert.Len(t, config.Options, 2)
}

func TestVolumeConfig_Defaults(t *testing.T) {
	config := &VolumeConfig{
		Name: "simple-volume",
	}

	assert.Equal(t, "simple-volume", config.Name)
	assert.Empty(t, config.Driver)
	assert.Empty(t, config.HostPath)
}

func TestVolumeConfig_NFSOptions(t *testing.T) {
	config := &VolumeConfig{
		Name:   "nfs-volume",
		Driver: "local",
		Options: map[string]string{
			"type":   "nfs",
			"o":      "addr=192.168.1.100,rw",
			"device": ":/export/data",
		},
	}

	assert.Equal(t, "nfs", config.Options["type"])
	assert.Contains(t, config.Options["o"], "192.168.1.100")
}

// ========== VolumeBackup 完整测试 ==========

func TestVolumeBackup_AllFields(t *testing.T) {
	now := time.Now()
	backup := &VolumeBackup{
		Name:       "data-volume_20240115_120000",
		VolumeName: "data-volume",
		BackupPath: "/backups/data-volume_20240115_120000.tar.gz",
		Size:       1024 * 1024 * 100,
		SizeHuman:  "100MB",
		Created:    now,
		Checksum:   "sha256:abc123def456789",
		Compressed: true,
	}

	assert.Equal(t, "data-volume_20240115_120000", backup.Name)
	assert.Equal(t, "data-volume", backup.VolumeName)
	assert.True(t, backup.Compressed)
	assert.NotEmpty(t, backup.Checksum)
}

func TestVolumeBackup_Uncompressed(t *testing.T) {
	backup := &VolumeBackup{
		Name:       "volume_backup",
		Compressed: false,
	}

	assert.False(t, backup.Compressed)
}

func TestVolumeBackup_SizeCalculation(t *testing.T) {
	tests := []struct {
		name     string
		size     uint64
		expected string
	}{
		{"1MB", 1024 * 1024, "1.00 MB"},
		{"100MB", 100 * 1024 * 1024, "100.00 MB"},
		{"1GB", 1024 * 1024 * 1024, "1.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup := &VolumeBackup{Size: tt.size}
			assert.Equal(t, tt.size, backup.Size)
		})
	}
}

// ========== ComposeService 完整测试 ==========

func TestComposeService_AllFields(t *testing.T) {
	service := &ComposeService{
		Name:      "api-service",
		Image:     "myapi:v1",
		Container: "my-api-container",
		Build:     "./api",
		Command:   []string{"node", "server.js"},
		Volumes:   []string{"./data:/app/data", "logs:/var/log"},
		Ports:     []string{"3000:3000", "8080:80"},
		Environment: map[string]string{
			"NODE_ENV":  "production",
			"DB_HOST":   "postgres",
			"DB_PORT":   "5432",
			"REDIS_URL": "redis://redis:6379",
		},
		EnvFile:   []string{".env", ".env.production"},
		Networks:  []string{"frontend", "backend"},
		DependsOn: []string{"postgres", "redis"},
		Restart:   "always",
		CPULimit:  "1.0",
		MemLimit:  "512m",
		Labels: map[string]string{
			"com.example.service": "api",
			"com.example.version": "1.0",
		},
		HealthCheck: &HealthCheckConfig{
			Test:        []string{"CMD", "curl", "-f", "http://localhost:3000/health"},
			Interval:    30 * time.Second,
			Timeout:     10 * time.Second,
			Retries:     3,
			StartPeriod: 60 * time.Second,
		},
	}

	assert.Equal(t, "api-service", service.Name)
	assert.Equal(t, "myapi:v1", service.Image)
	assert.Equal(t, "my-api-container", service.Container)
	assert.Len(t, service.Ports, 2)
	assert.Len(t, service.Volumes, 2)
	assert.Len(t, service.Environment, 4)
	assert.Len(t, service.Networks, 2)
	assert.Len(t, service.DependsOn, 2)
	assert.NotNil(t, service.HealthCheck)
}

func TestComposeService_Minimal(t *testing.T) {
	service := &ComposeService{
		Name:  "minimal",
		Image: "alpine",
	}

	assert.Equal(t, "minimal", service.Name)
	assert.Equal(t, "alpine", service.Image)
	assert.Empty(t, service.Container)
	assert.Nil(t, service.Volumes)
	assert.Nil(t, service.Ports)
}

func TestComposeService_WithBuild(t *testing.T) {
	tests := []struct {
		name    string
		build   interface{}
		service *ComposeService
	}{
		{"字符串路径", "./app", &ComposeService{Name: "app", Build: "./app"}},
		{"对象配置", map[string]interface{}{"context": ".", "dockerfile": "Dockerfile.prod"}, &ComposeService{Name: "app", Build: map[string]interface{}{"context": "."}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.service.Build)
		})
	}
}

// ========== HealthCheckConfig 完整测试 ==========

func TestHealthCheckConfig_AllFields(t *testing.T) {
	hc := &HealthCheckConfig{
		Test:        []string{"CMD-SHELL", "pg_isready -U postgres"},
		Interval:    10 * time.Second,
		Timeout:     5 * time.Second,
		Retries:     5,
		StartPeriod: 30 * time.Second,
	}

	assert.Len(t, hc.Test, 2)
	assert.Equal(t, 10*time.Second, hc.Interval)
	assert.Equal(t, 5*time.Second, hc.Timeout)
	assert.Equal(t, 5, hc.Retries)
	assert.Equal(t, 30*time.Second, hc.StartPeriod)
}

func TestHealthCheckConfig_TestFormats(t *testing.T) {
	tests := []struct {
		name string
		test []string
	}{
		{"CMD", []string{"CMD", "curl", "-f", "http://localhost/"}},
		{"CMD-SHELL", []string{"CMD-SHELL", "curl -f http://localhost/ || exit 1"}},
		{"NONE", []string{"NONE"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hc := &HealthCheckConfig{Test: tt.test}
			assert.Equal(t, tt.test, hc.Test)
		})
	}
}

// ========== ComposeProject 完整测试 ==========

func TestComposeProject_AllFields(t *testing.T) {
	now := time.Now()
	project := &ComposeProject{
		Name: "full-project",
		Path: "/projects/myapp/docker-compose.yml",
		Services: []*ComposeService{
			{Name: "web", Image: "nginx"},
			{Name: "api", Image: "myapi"},
			{Name: "db", Image: "postgres"},
		},
		Networks: map[string]interface{}{
			"frontend": map[string]interface{}{"driver": "bridge"},
			"backend":  map[string]interface{}{"driver": "bridge", "internal": true},
		},
		Volumes: map[string]interface{}{
			"db-data": map[string]interface{}{"driver": "local"},
			"logs":    nil,
		},
		Status:       "running",
		Containers:   []string{"full-project-web-1", "full-project-api-1", "full-project-db-1"},
		LastDeployed: now,
	}

	assert.Equal(t, "full-project", project.Name)
	assert.Len(t, project.Services, 3)
	assert.Equal(t, "running", project.Status)
	assert.Len(t, project.Containers, 3)
}

func TestComposeProject_Status(t *testing.T) {
	statuses := []string{"running", "stopped", "partial", "error"}

	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			project := &ComposeProject{Status: status}
			assert.Equal(t, status, project.Status)
		})
	}
}

// ========== ComposeConfig 测试 ==========

func TestComposeConfig_AllFields(t *testing.T) {
	config := &ComposeConfig{
		Name: "my-compose",
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

	assert.Equal(t, "my-compose", config.Name)
	assert.Len(t, config.Services, 1)
	assert.Len(t, config.Networks, 1)
	assert.Len(t, config.Volumes, 1)
}

// ========== DeployProgress 完整测试 ==========

func TestDeployProgress_AllFields(t *testing.T) {
	tests := []struct {
		name     string
		progress DeployProgress
	}{
		{"开始", DeployProgress{Current: 0, Total: 3, Status: "starting", Message: "Starting deployment..."}},
		{"进行中", DeployProgress{Current: 1, Total: 3, Service: "web", Status: "deploying", Message: "Building web..."}},
		{"完成", DeployProgress{Current: 3, Total: 3, Status: "completed", Message: "All services deployed", Completed: true}},
		{"错误", DeployProgress{Current: 2, Total: 3, Status: "error", Error: "Build failed", Completed: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEmpty(t, tt.progress.Status)
		})
	}
}

func TestDeployProgress_Progress(t *testing.T) {
	progress := &DeployProgress{
		Current: 2,
		Total:   5,
	}

	percent := float64(progress.Current) / float64(progress.Total) * 100
	assert.Equal(t, 40.0, percent)
}

// ========== 基准测试 ==========

func BenchmarkImage_AllFields(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Image{
			ID:           "sha256:abc123",
			Repository:   "nginx",
			Tag:          "latest",
			Size:         1024 * 1024 * 50,
			Architecture: "arm64",
		}
	}
}

func BenchmarkNetwork_AllFields(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Network{
			Name:    "test-network",
			Driver:  "bridge",
			Scope:   "local",
			Subnet:  "172.20.0.0/16",
			Gateway: "172.20.0.1",
		}
	}
}

func BenchmarkVolume_AllFields(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &Volume{
			Name:       "test-volume",
			Driver:     "local",
			MountPoint: "/var/lib/docker/volumes/test/_data",
			Size:       1024 * 1024 * 100,
		}
	}
}

func BenchmarkComposeService_AllFields(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &ComposeService{
			Name:    "api",
			Image:   "myapi:latest",
			Ports:   []string{"3000:3000"},
			Volumes: []string{"./data:/app/data"},
		}
	}
}

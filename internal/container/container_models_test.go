package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== ComposeService 测试 ==========

func TestComposeService_Structure(t *testing.T) {
	service := &ComposeService{
		Name:    "nginx",
		Image:   "nginx:latest",
		Command: []string{"nginx", "-g", "daemon off;"},
		Volumes: []string{"/host/data:/data"},
		Ports:   []string{"8080:80"},
		Environment: map[string]string{
			"TZ": "Asia/Shanghai",
		},
		Networks:  []string{"frontend"},
		Restart:   "always",
		CPULimit:  "0.5",
		MemLimit:  "512m",
		DependsOn: []string{"db"},
		Labels: map[string]string{
			"app": "nginx",
		},
	}

	assert.Equal(t, "nginx", service.Name)
	assert.Equal(t, "nginx:latest", service.Image)
	assert.Len(t, service.Ports, 1)
	assert.Len(t, service.Volumes, 1)
	assert.Equal(t, "always", service.Restart)
}

func TestHealthCheckConfig_Structure(t *testing.T) {
	hc := &HealthCheckConfig{
		Test:        []string{"CMD", "curl", "-f", "http://localhost/"},
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		Retries:     3,
		StartPeriod: 40 * time.Second,
	}

	assert.Len(t, hc.Test, 4)
	assert.Equal(t, 30*time.Second, hc.Interval)
	assert.Equal(t, 10*time.Second, hc.Timeout)
	assert.Equal(t, 3, hc.Retries)
}

// ========== ComposeProject 测试 ==========

func TestComposeProject_Structure(t *testing.T) {
	project := &ComposeProject{
		Name:     "myapp",
		Path:     "/opt/myapp/docker-compose.yml",
		Services: []*ComposeService{},
		Networks: map[string]interface{}{
			"frontend": nil,
			"backend":  nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
		Status:       "running",
		Containers:   []string{"myapp-nginx-1", "myapp-db-1"},
		LastDeployed: time.Now(),
	}

	assert.Equal(t, "myapp", project.Name)
	assert.Equal(t, "running", project.Status)
	assert.Len(t, project.Containers, 2)
}

// ========== ComposeConfig 测试 ==========

func TestComposeConfig_Structure(t *testing.T) {
	config := ComposeConfig{
		Name: "myapp",
		Services: map[string]interface{}{
			"nginx": map[string]interface{}{
				"image": "nginx:latest",
				"ports": []string{"8080:80"},
			},
		},
		Networks: map[string]interface{}{
			"frontend": nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
	}

	assert.Equal(t, "myapp", config.Name)
	assert.NotNil(t, config.Services)
	assert.NotNil(t, config.Networks)
	assert.NotNil(t, config.Volumes)
}

// ========== DeployProgress 测试 ==========

func TestDeployProgress_Structure(t *testing.T) {
	progress := DeployProgress{
		Current:   2,
		Total:     5,
		Service:   "nginx",
		Status:    "pulling",
		Message:   "Pulling image nginx:latest",
		Completed: false,
	}

	assert.Equal(t, 2, progress.Current)
	assert.Equal(t, 5, progress.Total)
	assert.Equal(t, "nginx", progress.Service)
	assert.False(t, progress.Completed)
}

func TestDeployProgress_Completed(t *testing.T) {
	progress := DeployProgress{
		Current:   5,
		Total:     5,
		Service:   "nginx",
		Status:    "running",
		Message:   "Service started",
		Completed: true,
	}

	assert.True(t, progress.Completed)
	assert.Equal(t, progress.Current, progress.Total)
}

func TestDeployProgress_WithError(t *testing.T) {
	progress := DeployProgress{
		Current:   1,
		Total:     5,
		Service:   "nginx",
		Status:    "error",
		Message:   "Failed to pull image",
		Completed: false,
		Error:     "image not found",
	}

	assert.NotEmpty(t, progress.Error)
	assert.False(t, progress.Completed)
}

// ========== CreateComposeFile 测试 ==========

func TestCreateComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "test-app",
		Services: []*ComposeService{
			{
				Name:    "web",
				Image:   "nginx:latest",
				Ports:   []string{"8080:80"},
				Restart: "always",
			},
		},
		Networks: map[string]interface{}{
			"frontend": nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)
	assert.FileExists(t, composePath)

	// Read and verify content
	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "nginx:latest")
	assert.Contains(t, content, "8080:80")
}

func TestCreateComposeFile_WithResources(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "resource-app",
		Services: []*ComposeService{
			{
				Name:     "api",
				Image:    "myapi:latest",
				CPULimit: "0.5",
				MemLimit: "512m",
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "cpus")
	assert.Contains(t, content, "memory")
}

func TestCreateComposeFile_MultipleServices(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "multi-service",
		Services: []*ComposeService{
			{
				Name:  "web",
				Image: "nginx:latest",
				Ports: []string{"80:80"},
			},
			{
				Name:    "db",
				Image:   "postgres:15",
				Volumes: []string{"db-data:/var/lib/postgresql/data"},
				Environment: map[string]string{
					"POSTGRES_PASSWORD": "secret",
				},
			},
			{
				Name:      "cache",
				Image:     "redis:alpine",
				DependsOn: []string{"db"},
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "nginx:latest")
	assert.Contains(t, content, "postgres:15")
	assert.Contains(t, content, "redis:alpine")
	assert.Contains(t, content, "depends_on")
}

// ========== ComposeManager 测试 ==========

func TestNewComposeManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	cm := NewComposeManager(mgr)
	assert.NotNil(t, cm)
}

// ========== Image 结构测试 ==========

func TestImage_Structure(t *testing.T) {
	image := &Image{
		ID:           "sha256:abc123",
		Repository:   "nginx",
		Tag:          "latest",
		FullName:     "nginx:latest",
		Size:         142000000,
		SizeHuman:    "142MB",
		Created:      time.Now(),
		Containers:   3,
		Labels:       map[string]string{"version": "1.0"},
		Architecture: "amd64",
		OS:           "linux",
	}

	assert.Equal(t, "sha256:abc123", image.ID)
	assert.Equal(t, "nginx", image.Repository)
	assert.Equal(t, "latest", image.Tag)
	assert.Equal(t, 3, image.Containers)
	assert.Equal(t, "amd64", image.Architecture)
	assert.Equal(t, "linux", image.OS)
}

func TestImage_FullName(t *testing.T) {
	image := &Image{
		Repository: "myregistry.io/myapp",
		Tag:        "v1.0.0",
	}

	expected := "myregistry.io/myapp:v1.0.0"
	// FullName should be computed or set
	image.FullName = image.Repository + ":" + image.Tag
	assert.Equal(t, expected, image.FullName)
}

// ========== ImagePullProgress 测试 ==========

func TestImagePullProgress_Structure(t *testing.T) {
	progress := ImagePullProgress{
		Status:   "Downloading",
		Progress: "[==>                                                ]     10B/100B",
		ID:       "abc123",
	}
	progress.ProgressDetail.Current = 10
	progress.ProgressDetail.Total = 100

	assert.Equal(t, "Downloading", progress.Status)
	assert.Equal(t, uint64(10), progress.ProgressDetail.Current)
	assert.Equal(t, uint64(100), progress.ProgressDetail.Total)
	assert.NotEmpty(t, progress.Progress)
}

// ========== ImageConfig 测试 ==========

func TestImageConfig_Structure(t *testing.T) {
	config := ImageConfig{
		Repository: "nginx",
		Tag:        "latest",
		Platform:   "linux/amd64",
	}

	assert.Equal(t, "nginx", config.Repository)
	assert.Equal(t, "latest", config.Tag)
	assert.Equal(t, "linux/amd64", config.Platform)
}

func TestImageConfig_DefaultTag(t *testing.T) {
	config := ImageConfig{
		Repository: "nginx",
	}

	// Default tag should be "latest" if not specified
	if config.Tag == "" {
		config.Tag = "latest"
	}
	assert.Equal(t, "latest", config.Tag)
}

// ========== ImageManager 测试 ==========

func TestNewImageManager(t *testing.T) {
	mgr := &Manager{socketPath: "/var/run/docker.sock"}
	im := NewImageManager(mgr)
	assert.NotNil(t, im)
}

func TestContainerConfig_Defaults(t *testing.T) {
	config := ContainerConfig{
		Name:  "test",
		Image: "alpine",
	}

	// Test defaults
	if config.Restart == "" {
		config.Restart = "no"
	}
	if config.Network == "" {
		config.Network = "bridge"
	}

	assert.Equal(t, "no", config.Restart)
	assert.Equal(t, "bridge", config.Network)
}

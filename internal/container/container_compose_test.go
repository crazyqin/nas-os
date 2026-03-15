package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

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
				Name:  "db",
				Image: "postgres:15",
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

func TestCreateComposeFile_WithEnvironment(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "env-app",
		Services: []*ComposeService{
			{
				Name:  "app",
				Image: "myapp:latest",
				Environment: map[string]string{
					"DB_HOST":     "db",
					"DB_PORT":     "5432",
					"APP_ENV":     "production",
					"DEBUG":       "false",
					"LOG_LEVEL":   "info",
				},
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "DB_HOST")
	assert.Contains(t, content, "production")
}

func TestCreateComposeFile_WithVolumes(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "volume-app",
		Services: []*ComposeService{
			{
				Name:  "db",
				Image: "postgres:15",
				Volumes: []string{
					"db-data:/var/lib/postgresql/data",
					"./init.sql:/docker-entrypoint-initdb.d/init.sql:ro",
				},
			},
		},
		Volumes: map[string]interface{}{
			"db-data": map[string]interface{}{
				"driver": "local",
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "db-data")
	assert.Contains(t, content, "volumes")
}

func TestCreateComposeFile_WithContainerName(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "named-app",
		Services: []*ComposeService{
			{
				Name:      "web",
				Container: "my-custom-web",
				Image:     "nginx:latest",
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "container_name")
	assert.Contains(t, content, "my-custom-web")
}

func TestCreateComposeFile_WithNetworks(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "network-app",
		Services: []*ComposeService{
			{
				Name:     "web",
				Image:    "nginx:latest",
				Networks: []string{"frontend", "backend"},
			},
			{
				Name:     "api",
				Image:    "myapi:latest",
				Networks: []string{"backend"},
			},
		},
		Networks: map[string]interface{}{
			"frontend": map[string]interface{}{
				"driver": "bridge",
			},
			"backend": map[string]interface{}{
				"driver": "bridge",
				"internal": true,
			},
		},
	}

	err := cm.CreateComposeFile(composePath, project)
	require.NoError(t, err)

	data, err := os.ReadFile(composePath)
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "frontend")
	assert.Contains(t, content, "backend")
}

// ========== parseService 测试 ==========

func TestParseService_Basic(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image":   "nginx:latest",
		"command": "nginx -g 'daemon off;'",
		"restart": "always",
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "web", service.Name)
	assert.Equal(t, "nginx:latest", service.Image)
	assert.Equal(t, "always", service.Restart)
}

func TestParseService_WithPorts(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "nginx:latest",
		"ports": []interface{}{"8080:80", "443:443"},
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Len(t, service.Ports, 2)
	assert.Contains(t, service.Ports, "8080:80")
}

func TestParseService_WithVolumes(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "postgres:15",
		"volumes": []interface{}{
			"db-data:/var/lib/postgresql/data",
			"./init:/docker-entrypoint-initdb.d",
		},
	}

	service, err := cm.parseService("db", serviceData)
	require.NoError(t, err)
	assert.Len(t, service.Volumes, 2)
}

func TestParseService_WithEnvironment(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "postgres:15",
		"environment": map[string]interface{}{
			"POSTGRES_PASSWORD": "secret",
			"POSTGRES_USER":     "admin",
		},
	}

	service, err := cm.parseService("db", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "secret", service.Environment["POSTGRES_PASSWORD"])
	assert.Equal(t, "admin", service.Environment["POSTGRES_USER"])
}

func TestParseService_WithEnvironmentArray(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "myapp:latest",
		"environment": []interface{}{
			"DB_HOST=db",
			"DB_PORT=5432",
		},
	}

	service, err := cm.parseService("app", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "db", service.Environment["DB_HOST"])
	assert.Equal(t, "5432", service.Environment["DB_PORT"])
}

func TestParseService_WithLabels(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "nginx:latest",
		"labels": map[string]interface{}{
			"app":     "web",
			"version": "1.0",
		},
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "web", service.Labels["app"])
	assert.Equal(t, "1.0", service.Labels["version"])
}

func TestParseService_WithLabelsArray(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "nginx:latest",
		"labels": []interface{}{
			"app=web",
			"version=1.0",
		},
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "web", service.Labels["app"])
}

func TestParseService_WithDependsOn(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image":      "myapp:latest",
		"depends_on": []interface{}{"db", "redis"},
	}

	service, err := cm.parseService("app", serviceData)
	require.NoError(t, err)
	assert.Len(t, service.DependsOn, 2)
	assert.Contains(t, service.DependsOn, "db")
}

func TestParseService_WithHealthCheck(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "nginx:latest",
		"healthcheck": map[string]interface{}{
			"test":     []interface{}{"CMD", "curl", "-f", "http://localhost/"},
			"interval": "30s",
			"timeout":  "10s",
			"retries":  3,
		},
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	require.NotNil(t, service.HealthCheck)
	assert.Len(t, service.HealthCheck.Test, 4)
	assert.Equal(t, 30*time.Second, service.HealthCheck.Interval)
	assert.Equal(t, 10*time.Second, service.HealthCheck.Timeout)
	assert.Equal(t, 3, service.HealthCheck.Retries)
}

func TestParseService_WithDeployResources(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image": "myapp:latest",
		"deploy": map[string]interface{}{
			"resources": map[string]interface{}{
				"limits": map[string]interface{}{
					"cpus":   "0.5",
					"memory": "512m",
				},
			},
		},
	}

	service, err := cm.parseService("app", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "0.5", service.CPULimit)
	assert.Equal(t, "512m", service.MemLimit)
}

func TestParseService_WithNetworks(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image":    "nginx:latest",
		"networks": []interface{}{"frontend", "backend"},
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Len(t, service.Networks, 2)
}

func TestParseService_WithEnvFile(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image":    "myapp:latest",
		"env_file": []interface{}{".env", ".env.local"},
	}

	service, err := cm.parseService("app", serviceData)
	require.NoError(t, err)
	assert.Len(t, service.EnvFile, 2)
}

func TestParseService_WithContainerName(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := map[string]interface{}{
		"image":          "nginx:latest",
		"container_name": "my-nginx",
	}

	service, err := cm.parseService("web", serviceData)
	require.NoError(t, err)
	assert.Equal(t, "my-nginx", service.Container)
}

func TestParseService_InvalidData(t *testing.T) {
	cm := &ComposeManager{}
	
	serviceData := "invalid data"

	service, err := cm.parseService("web", serviceData)
	assert.Error(t, err)
	assert.Nil(t, service)
}

// ========== ParseComposeFile 测试 ==========

func TestParseComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	composeContent := `
name: test-app
services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    restart: always
  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - db-data:/var/lib/postgresql/data
networks:
  frontend:
    driver: bridge
volumes:
  db-data:
`
	err := os.WriteFile(composePath, []byte(composeContent), 0644)
	require.NoError(t, err)

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project, err := cm.ParseComposeFile(composePath)
	require.NoError(t, err)
	assert.Equal(t, "test-app", project.Name)
	assert.Len(t, project.Services, 2)
}

func TestParseComposeFile_FileNotFound(t *testing.T) {
	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	_, err := cm.ParseComposeFile("/nonexistent/docker-compose.yml")
	assert.Error(t, err)
}

func TestParseComposeFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	err := os.WriteFile(composePath, []byte("invalid: yaml: content: ["), 0644)
	require.NoError(t, err)

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	_, err = cm.ParseComposeFile(composePath)
	assert.Error(t, err)
}

// ========== ComposeConfig 序列化测试 ==========

func TestComposeConfig_Marshal(t *testing.T) {
	config := ComposeConfig{
		Name: "test-app",
		Services: map[string]interface{}{
			"web": map[string]interface{}{
				"image":   "nginx:latest",
				"ports":   []string{"8080:80"},
				"restart": "always",
			},
		},
		Networks: map[string]interface{}{
			"frontend": nil,
		},
		Volumes: map[string]interface{}{
			"data": nil,
		},
	}

	data, err := yaml.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test-app")
	assert.Contains(t, string(data), "nginx:latest")
}

// ========== ComposeService 默认值测试 ==========

func TestComposeService_Defaults(t *testing.T) {
	service := &ComposeService{
		Name: "test",
	}

	// 验证默认值
	assert.Empty(t, service.Image)
	assert.Nil(t, service.Volumes)
	assert.Nil(t, service.Ports)
	assert.Nil(t, service.Networks)
}

// ========== 基准测试 ==========

func BenchmarkCreateComposeFile(b *testing.B) {
	tmpDir := b.TempDir()
	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	project := &ComposeProject{
		Name: "bench-app",
		Services: []*ComposeService{
			{Name: "web", Image: "nginx:latest", Ports: []string{"80:80"}},
			{Name: "db", Image: "postgres:15"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		composePath := filepath.Join(tmpDir, "docker-compose.yml")
		_ = cm.CreateComposeFile(composePath, project)
	}
}

func BenchmarkParseComposeFile(b *testing.B) {
	tmpDir := b.TempDir()
	composePath := filepath.Join(tmpDir, "docker-compose.yml")

	composeContent := `
name: bench-app
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
`
	_ = os.WriteFile(composePath, []byte(composeContent), 0644)

	mgr := &Manager{}
	cm := NewComposeManager(mgr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.ParseComposeFile(composePath)
	}
}

func BenchmarkParseService(b *testing.B) {
	cm := &ComposeManager{}
	serviceData := map[string]interface{}{
		"image":   "nginx:latest",
		"ports":   []interface{}{"80:80"},
		"volumes": []interface{}{"/data:/app/data"},
		"environment": map[string]interface{}{
			"ENV": "production",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cm.parseService("web", serviceData)
	}
}
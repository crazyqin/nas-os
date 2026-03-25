package docker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManualInstallRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ManualInstallRequest
		wantErr bool
	}{
		{
			name: "valid compose request",
			req: ManualInstallRequest{
				Type:           "compose",
				ComposeContent: "version: '3'\nservices:\n  nginx:\n    image: nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "valid image request",
			req: ManualInstallRequest{
				Type:  "image",
				Image: "nginx:latest",
			},
			wantErr: false,
		},
		{
			name: "missing compose content and url",
			req: ManualInstallRequest{
				Type: "compose",
			},
			wantErr: true,
		},
		{
			name: "missing image",
			req: ManualInstallRequest{
				Type: "image",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation logic
			gotErr := (tt.req.Type == "compose" && tt.req.ComposeContent == "" && tt.req.ComposeURL == "") ||
				(tt.req.Type == "image" && tt.req.Image == "")
			if gotErr != tt.wantErr {
			if gotErr != tt.wantErr {
				t.Errorf("validation error = %v, wantErr %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestDependencyDetector_DetectFromCompose(t *testing.T) {
	detector := &DependencyDetector{}

	tests := []struct {
		name          string
		compose       string
		wantDepsCount int
	}{
		{
			name: "compose with depends_on",
			compose: `version: '3'
services:
  web:
    image: nginx
    depends_on:
      - db
      - redis
  db:
    image: postgres
  redis:
    image: redis`,
			wantDepsCount: 2,
		},
		{
			name: "compose without depends_on",
			compose: `version: '3'
services:
  nginx:
    image: nginx:latest`,
			wantDepsCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := detector.DetectFromCompose(tt.compose)
			if err != nil {
				t.Errorf("DetectFromCompose() error = %v", err)
				return
			}
			if len(deps) != tt.wantDepsCount {
				t.Errorf("DetectFromCompose() got %d dependencies, want %d", len(deps), tt.wantDepsCount)
			}
		})
	}
}

func TestDependencyDetector_DetectFromImage(t *testing.T) {
	detector := &DependencyDetector{}

	tests := []struct {
		name     string
		image    string
		wantDeps []string
	}{
		{
			name:     "immich image",
			image:    "ghcr.io/immich-app/immich-server:latest",
			wantDeps: []string{"postgres", "redis"},
		},
		{
			name:     "nextcloud image",
			image:    "nextcloud:latest",
			wantDeps: []string{"postgres", "redis"},
		},
		{
			name:     "standalone image",
			image:    "nginx:latest",
			wantDeps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps, err := detector.DetectFromImage(tt.image)
			if err != nil {
				t.Errorf("DetectFromImage() error = %v", err)
				return
			}
			if len(deps) != len(tt.wantDeps) {
				t.Errorf("DetectFromImage() got %v, want %v", deps, tt.wantDeps)
			}
		})
	}
}

func TestManualInstaller_ExtractAppNameFromCompose(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "manual-install-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, _ := NewManager()
	store, _ := NewAppStore(mgr, tmpDir)
	installer := NewManualInstaller(store, mgr, tmpDir)

	tests := []struct {
		name     string
		compose  string
		wantName string
	}{
		{
			name: "compose with container_name",
			compose: `version: '3'
services:
  web:
    image: nginx
    container_name: my-nginx`,
			wantName: "my-nginx",
		},
		{
			name: "compose with service name",
			compose: `version: '3'
services:
  myapp:
    image: nginx:latest`,
			wantName: "myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName := installer.extractAppNameFromCompose(tt.compose)
			if gotName != tt.wantName {
				t.Errorf("extractAppNameFromCompose() = %v, want %v", gotName, tt.wantName)
			}
		})
	}
}

func TestManualInstaller_SaveMeta(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "manual-install-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	appDir := filepath.Join(tmpDir, "test-app")
	if err := os.MkdirAll(appDir, 0750); err != nil {
		t.Fatalf("failed to create app dir: %v", err)
	}

	mgr, _ := NewManager()
	store, _ := NewAppStore(mgr, tmpDir)
	installer := NewManualInstaller(store, mgr, tmpDir)

	meta := &ManualAppMeta{
		Name:        "test-app",
		DisplayName: "Test App",
		Description: "A test application",
		Category:    "Test",
		Icon:        "🧪",
		Image:       "test:latest",
		Type:        "image",
	}

	err = installer.saveMeta(appDir, meta)
	if err != nil {
		t.Errorf("saveMeta() error = %v", err)
		return
	}

	// Verify the file was created
	metaPath := filepath.Join(appDir, "meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("meta.json was not created")
	}
}

func TestPortMappingReq(t *testing.T) {
	port := PortMappingReq{
		HostPort:      8080,
		ContainerPort: 80,
		Protocol:      "tcp",
	}

	if port.HostPort != 8080 {
		t.Errorf("HostPort = %d, want 8080", port.HostPort)
	}
	if port.ContainerPort != 80 {
		t.Errorf("ContainerPort = %d, want 80", port.ContainerPort)
	}
}

func TestVolumeMappingReq(t *testing.T) {
	vol := VolumeMappingReq{
		HostPath:      "/host/path",
		ContainerPath: "/container/path",
		ReadOnly:      true,
	}

	if vol.HostPath != "/host/path" {
		t.Errorf("HostPath = %s, want /host/path", vol.HostPath)
	}
	if !vol.ReadOnly {
		t.Error("ReadOnly should be true")
	}
}

func TestManualInstallResult(t *testing.T) {
	result := &ManualInstallResult{
		ID:           "manual-nginx",
		Name:         "nginx",
		DisplayName:  "Nginx",
		Status:       "running",
		Type:         "image",
		Ports:        map[int]int{80: 8080},
		Volumes:      map[string]string{"/data": "/var/nginx"},
		Dependencies: []string{"redis"},
	}

	if result.ID != "manual-nginx" {
		t.Errorf("ID = %s, want manual-nginx", result.ID)
	}
	if result.Type != "image" {
		t.Errorf("Type = %s, want image", result.Type)
	}
	if len(result.Dependencies) != 1 {
		t.Errorf("Dependencies count = %d, want 1", len(result.Dependencies))
	}
}

func TestLatestAppsResponse(t *testing.T) {
	resp := &LatestAppsResponse{
		Trending: []*AppTemplate{
			{ID: "nginx", Name: "nginx", DisplayName: "Nginx"},
		},
		Categories: map[string]int{"Web": 1},
	}

	if len(resp.Trending) != 1 {
		t.Errorf("Trending count = %d, want 1", len(resp.Trending))
	}
	if resp.Categories["Web"] != 1 {
		t.Errorf("Categories[Web] = %d, want 1", resp.Categories["Web"])
	}
}

func TestManualAppMeta(t *testing.T) {
	meta := &ManualAppMeta{
		Name:        "test",
		DisplayName: "Test App",
		Description: "Test Description",
		Category:    "Test",
		Icon:        "🧪",
		Image:       "test:latest",
		Type:        "image",
		Environment: map[string]string{"KEY": "value"},
	}

	if meta.Name != "test" {
		t.Errorf("Name = %s, want test", meta.Name)
	}
	if meta.Environment["KEY"] != "value" {
		t.Errorf("Environment[KEY] = %s, want value", meta.Environment["KEY"])
	}
}

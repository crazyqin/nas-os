package dockergui

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestListContainersEmpty(t *testing.T) {
	m := NewManager()
	containers := m.ListContainers(true)
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

func TestDeployCompose(t *testing.T) {
	m := NewManager()
	project, err := m.DeployCompose("webapp", "version: '3'\nservices:\n  web:\n    image: nginx")
	if err != nil {
		t.Fatalf("DeployCompose failed: %v", err)
	}
	if project.Name != "webapp" {
		t.Errorf("expected 'webapp', got '%s'", project.Name)
	}
	if project.Status != "running" {
		t.Errorf("expected 'running', got '%s'", project.Status)
	}
}

func TestDeployComposeDuplicate(t *testing.T) {
	m := NewManager()
	m.DeployCompose("webapp", "config1")
	_, err := m.DeployCompose("webapp", "config2")
	if err == nil {
		t.Fatal("expected error for duplicate project")
	}
}

func TestStopCompose(t *testing.T) {
	m := NewManager()
	m.DeployCompose("webapp", "config")
	err := m.StopCompose("webapp")
	if err != nil {
		t.Fatalf("StopCompose failed: %v", err)
	}
	project, _ := m.GetComposeProject("webapp")
	if project.Status != "stopped" {
		t.Errorf("expected stopped, got '%s'", project.Status)
	}
}

func TestRemoveCompose(t *testing.T) {
	m := NewManager()
	m.DeployCompose("webapp", "config")
	err := m.RemoveCompose("webapp")
	if err != nil {
		t.Fatalf("RemoveCompose failed: %v", err)
	}
	_, err = m.GetComposeProject("webapp")
	if err == nil {
		t.Fatal("expected error after removal")
	}
}

func TestPullImage(t *testing.T) {
	m := NewManager()
	img, err := m.PullImage("nginx", "latest")
	if err != nil {
		t.Fatalf("PullImage failed: %v", err)
	}
	if img.Repository != "nginx" {
		t.Errorf("expected 'nginx', got '%s'", img.Repository)
	}
}

func TestRemoveImage(t *testing.T) {
	m := NewManager()
	m.PullImage("nginx", "latest")
	err := m.RemoveImage("nginx:latest")
	if err != nil {
		t.Fatalf("RemoveImage failed: %v", err)
	}
	images := m.ListImages()
	if len(images) != 0 {
		t.Errorf("expected 0 images, got %d", len(images))
	}
}

func TestCreateNetwork(t *testing.T) {
	m := NewManager()
	n, err := m.CreateNetwork("mynet", "bridge", "172.20.0.0/16")
	if err != nil {
		t.Fatalf("CreateNetwork failed: %v", err)
	}
	if n.Driver != "bridge" {
		t.Errorf("expected 'bridge', got '%s'", n.Driver)
	}
}

func TestCreateNetworkDuplicate(t *testing.T) {
	m := NewManager()
	m.CreateNetwork("mynet", "bridge", "172.20.0.0/16")
	_, err := m.CreateNetwork("mynet", "overlay", "172.21.0.0/16")
	if err == nil {
		t.Fatal("expected error for duplicate network")
	}
}

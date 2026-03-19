package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContainerStruct(t *testing.T) {
	container := Container{
		ID:      "abc123",
		Name:    "test-container",
		Image:   "nginx:latest",
		Status:  "running",
		State:   "running",
		Created: time.Now(),
		Ports: []PortMapping{
			{ContainerPort: "80", HostPort: "8080", Protocol: "tcp"},
		},
		Labels: map[string]string{
			"app": "nginx",
		},
	}

	assert.Equal(t, "abc123", container.ID)
	assert.Equal(t, "test-container", container.Name)
	assert.Equal(t, "running", container.Status)
	assert.Equal(t, 1, len(container.Ports))
}

func TestContainerConfigStruct(t *testing.T) {
	// Note: ContainerConfig type does not exist in this package
	// Skipping this test as the type is not defined
	t.Skip("ContainerConfig type not defined in container package")
}

func TestImageStruct(t *testing.T) {
	image := Image{
		ID:         "sha256:abc123",
		Repository: "nginx",
		Tag:        "latest",
		Size:       133183744,
		Created:    time.Now(),
	}

	assert.Equal(t, "sha256:abc123", image.ID)
	assert.Equal(t, "nginx", image.Repository)
	assert.Equal(t, "latest", image.Tag)
	assert.Equal(t, uint64(133183744), image.Size)
}

func TestVolumeStruct(t *testing.T) {
	volume := Volume{
		Name:       "data-volume",
		Driver:     "local",
		MountPoint: "/var/lib/docker/volumes/data-volume/_data",
		Created:    time.Now(),
		Labels: map[string]string{
			"backup": "enabled",
		},
	}

	assert.Equal(t, "data-volume", volume.Name)
	assert.Equal(t, "local", volume.Driver)
	assert.NotEmpty(t, volume.MountPoint)
}

func TestNetworkConfigStruct(t *testing.T) {
	network := NetworkConfig{
		Name:     "app-network",
		Driver:   "bridge",
		Subnet:   "172.20.0.0/16",
		Gateway:  "172.20.0.1",
		Internal: false,
	}

	assert.Equal(t, "app-network", network.Name)
	assert.Equal(t, "bridge", network.Driver)
	assert.Equal(t, "172.20.0.0/16", network.Subnet)
}

func TestPortMappingStruct(t *testing.T) {
	pm := PortMapping{
		ContainerPort: "80",
		HostPort:      "8080",
		Protocol:      "tcp",
		HostIP:        "0.0.0.0",
	}

	assert.Equal(t, "80", pm.ContainerPort)
	assert.Equal(t, "8080", pm.HostPort)
	assert.Equal(t, "tcp", pm.Protocol)
	assert.Equal(t, "0.0.0.0", pm.HostIP)
}

func TestVolumeMountStruct(t *testing.T) {
	vm := VolumeMount{
		Source:      "/host/data",
		Destination: "/container/data",
		Mode:        "rw",
		RW:          true,
	}

	assert.Equal(t, "/host/data", vm.Source)
	assert.Equal(t, "/container/data", vm.Destination)
	assert.Equal(t, "rw", vm.Mode)
	assert.True(t, vm.RW)
}

func TestContainerStatsStruct(t *testing.T) {
	// Note: ContainerStats type does not exist in this package
	// Skipping this test as the type is not defined
	t.Skip("ContainerStats type not defined in container package")
}

func TestComposeServiceStruct(t *testing.T) {
	service := ComposeService{
		Name:        "web",
		Image:       "nginx:latest",
		Restart:     "always",
		Environment: map[string]string{"ENV": "production"},
		Ports:       []string{"80:80"},
		Volumes:     []string{"/data:/data"},
		Networks:    []string{"frontend", "backend"},
	}

	assert.Equal(t, "web", service.Name)
	assert.Equal(t, 2, len(service.Networks))
}

func TestComposeProjectStruct(t *testing.T) {
	project := ComposeProject{
		Name: "my-app",
		Services: []*ComposeService{
			{Name: "web", Image: "nginx"},
			{Name: "db", Image: "postgres"},
		},
		Networks: map[string]interface{}{"frontend": nil, "backend": nil},
		Volumes:  map[string]interface{}{"data": nil},
	}

	assert.Equal(t, "my-app", project.Name)
	assert.Equal(t, 2, len(project.Services))
	assert.Equal(t, 2, len(project.Networks))
}

func TestContainerStatus(t *testing.T) {
	statuses := []string{"running", "exited", "paused", "restarting", "dead"}

	for _, status := range statuses {
		container := Container{Status: status}
		assert.Equal(t, status, container.Status)
	}
}

func TestImageWithMultipleTags(t *testing.T) {
	image := Image{
		ID:         "sha256:abc123",
		Repository: "nginx",
		Tag:        "latest,1.21,stable",
	}

	assert.Contains(t, image.Tag, "latest")
	assert.Contains(t, image.Tag, "1.21")
}

func TestContainerWithMultipleLabels(t *testing.T) {
	container := Container{
		ID: "abc123",
		Labels: map[string]string{
			"app":        "nginx",
			"version":    "1.0",
			"component":  "frontend",
			"managed_by": "docker-compose",
		},
	}

	assert.Equal(t, 4, len(container.Labels))
	assert.Equal(t, "nginx", container.Labels["app"])
}

func TestContainerConfigWithAllFields(t *testing.T) {
	// Note: ContainerConfig type does not exist in this package
	// Skipping this test as the type is not defined
	t.Skip("ContainerConfig type not defined in container package")
}

func TestVolumeWithLabels(t *testing.T) {
	volume := Volume{
		Name:    "labeled-volume",
		Driver:  "local",
		Labels:  map[string]string{"backup": "daily", "tier": "fast"},
		Created: time.Now(),
	}

	assert.Equal(t, 2, len(volume.Labels))
	assert.Equal(t, "daily", volume.Labels["backup"])
}

func TestNetworkConfigWithOptions(t *testing.T) {
	network := NetworkConfig{
		Name:     "custom-network",
		Driver:   "bridge",
		Subnet:   "172.20.0.0/16",
		Gateway:  "172.20.0.1",
		Internal: false,
		Labels:   map[string]string{"environment": "production"},
		Options: map[string]string{
			"com.docker.network.bridge.enable_icc": "true",
		},
	}

	assert.Equal(t, "custom-network", network.Name)
	assert.True(t, len(network.Options) > 0)
}

func TestContainerStatsWithAllFields(t *testing.T) {
	// Note: ContainerStats type does not exist in this package
	// Skipping this test as the type is not defined
	t.Skip("ContainerStats type not defined in container package")
}

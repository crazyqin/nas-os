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
			{ContainerPort: 80, HostPort: 8080, Protocol: "tcp"},
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
	config := ContainerConfig{
		Name:       "test-container",
		Image:      "nginx:latest",
		Cmd:        []string{"nginx", "-g", "daemon off;"},
		Env:        []string{"ENV=production"},
		Ports:      []string{"80:80"},
		Volumes:    []string{"/data:/data"},
		Network:    "bridge",
		AutoStart:  true,
		Restart:    "always",
		Memory:     512,
		CPUShares:  1024,
		Privileged: false,
	}

	assert.Equal(t, "test-container", config.Name)
	assert.Equal(t, "nginx:latest", config.Image)
	assert.True(t, config.AutoStart)
	assert.Equal(t, int64(512), config.Memory)
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
	assert.Equal(t, int64(133183744), image.Size)
}

func TestVolumeStruct(t *testing.T) {
	volume := Volume{
		Name:       "data-volume",
		Driver:     "local",
		Mountpoint: "/var/lib/docker/volumes/data-volume/_data",
		Created:    time.Now(),
		Labels: map[string]string{
			"backup": "enabled",
		},
	}

	assert.Equal(t, "data-volume", volume.Name)
	assert.Equal(t, "local", volume.Driver)
	assert.NotEmpty(t, volume.Mountpoint)
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
		ContainerPort: 80,
		HostPort:      8080,
		Protocol:      "tcp",
		HostIP:        "0.0.0.0",
	}

	assert.Equal(t, 80, pm.ContainerPort)
	assert.Equal(t, 8080, pm.HostPort)
	assert.Equal(t, "tcp", pm.Protocol)
	assert.Equal(t, "0.0.0.0", pm.HostIP)
}

func TestVolumeMountStruct(t *testing.T) {
	vm := VolumeMount{
		Source:   "/host/data",
		Target:   "/container/data",
		Type:     "bind",
		ReadOnly: false,
	}

	assert.Equal(t, "/host/data", vm.Source)
	assert.Equal(t, "/container/data", vm.Target)
	assert.Equal(t, "bind", vm.Type)
	assert.False(t, vm.ReadOnly)
}

func TestContainerStatsStruct(t *testing.T) {
	stats := ContainerStats{
		ContainerID:   "abc123",
		CPUPercent:    25.5,
		MemoryUsage:   524288000,
		MemoryLimit:   1073741824,
		MemoryPercent: 48.8,
		NetworkRx:     1024000,
		NetworkTx:     512000,
		BlockRead:     2048000,
		BlockWrite:    1024000,
	}

	assert.Equal(t, "abc123", stats.ContainerID)
	assert.Equal(t, 25.5, stats.CPUPercent)
	assert.Equal(t, int64(524288000), stats.MemoryUsage)
}

func TestComposeServiceStruct(t *testing.T) {
	service := ComposeService{
		Name:          "web",
		Image:         "nginx:latest",
		Replicas:      3,
		RestartPolicy: "always",
		Environment:   map[string]string{"ENV": "production"},
		Ports:         []string{"80:80"},
		Volumes:       []string{"/data:/data"},
		Networks:      []string{"frontend", "backend"},
	}

	assert.Equal(t, "web", service.Name)
	assert.Equal(t, 3, service.Replicas)
	assert.Equal(t, 2, len(service.Networks))
}

func TestComposeProjectStruct(t *testing.T) {
	project := ComposeProject{
		Name: "my-app",
		Services: []ComposeService{
			{Name: "web", Image: "nginx"},
			{Name: "db", Image: "postgres"},
		},
		Networks: []string{"frontend", "backend"},
		Volumes:  []string{"data"},
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
	config := ContainerConfig{
		Name:            "full-config-container",
		Image:           "app:latest",
		Cmd:             []string{"./start.sh"},
		Env:             []string{"ENV=prod", "DEBUG=false"},
		Ports:           []string{"80:80", "443:443"},
		Volumes:         []string{"/data:/data", "/logs:/logs"},
		Network:         "app-network",
		AutoStart:       true,
		Restart:         "always",
		Memory:          1024,
		CPUShares:       2048,
		Privileged:      false,
		User:            "appuser",
		WorkingDir:      "/app",
		Hostname:        "app-server",
		Domainname:      "example.com",
		MacAddress:      "02:42:ac:11:00:02",
		StopSignal:      "SIGTERM",
		StopTimeout:     30,
		HealthCheckCmd:  "curl -f http://localhost/health",
		HealthInterval:  30,
		HealthTimeout:   5,
		HealthRetries:   3,
		LogDriver:       "json-file",
		LogOpts:         map[string]string{"max-size": "10m", "max-file": "3"},
		SecurityOpts:    []string{"no-new-privileges"},
		CapAdd:          []string{"NET_ADMIN"},
		CapDrop:         []string{"MKNOD"},
		DNSServers:      []string{"8.8.8.8", "8.8.4.4"},
		DNSSearch:       []string{"example.com"},
		ExtraHosts:      []string{"host1:192.168.1.1"},
		Devices:         []string{"/dev/sda:/dev/sda"},
		Ulimits:         map[string]int{"nofile": 65536},
		ReadOnlyRootFS:  false,
		StdinOnce:       false,
		Tty:             false,
		NetworkDisabled: false,
		Entrypoint:      []string{"/entrypoint.sh"},
	}

	assert.Equal(t, "full-config-container", config.Name)
	assert.Equal(t, int64(1024), config.Memory)
	assert.Equal(t, 30, config.StopTimeout)
	assert.True(t, config.AutoStart)
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
	stats := ContainerStats{
		ContainerID:     "full-stats",
		CPUPercent:      50.5,
		MemoryUsage:     536870912,
		MemoryLimit:     1073741824,
		MemoryPercent:   50.0,
		MemoryCache:     134217728,
		NetworkRx:       1048576,
		NetworkTx:       524288,
		NetworkRxPackets: 1000,
		NetworkTxPackets: 800,
		BlockRead:       2097152,
		BlockWrite:      1048576,
		PIDs:            10,
	}

	assert.Equal(t, "full-stats", stats.ContainerID)
	assert.Equal(t, 50.5, stats.CPUPercent)
	assert.Equal(t, 10, stats.PIDs)
}
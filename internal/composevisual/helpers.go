package composevisual

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GenerateUUID 生成 UUID
func GenerateUUID() string {
	return uuid.New().String()
}

// Now 返回当前时间
func Now() time.Time {
	return time.Now()
}

// CalculateNodePosition 计算节点位置（自动布局算法 - 网格布局）
func CalculateNodePosition(index int) *NodePosition {
	const (
		nodeWidth  = 280
		nodeHeight = 180
		marginX    = 70
		marginY    = 70
		startX     = 100
		startY     = 100
		cols       = 3
	)
	return &NodePosition{
		X:      startX + (index%cols)*(nodeWidth+marginX),
		Y:      startY + (index/cols)*(nodeHeight+marginY),
		Width:  nodeWidth,
		Height: nodeHeight,
	}
}

// ClassifyServices 对服务进行分类分层
func ClassifyServices(services map[string]*ServiceNode) map[string]int {
	tierMap := make(map[string]int)
	for name, svc := range services {
		img := strings.ToLower(svc.Image)
		switch {
		case IsProxyImage(img):
			tierMap[name] = 0
		case IsDatabaseImage(img) || IsCacheImage(img):
			tierMap[name] = 2
		case IsInfraImage(img):
			tierMap[name] = 3
		default:
			tierMap[name] = 1
		}
	}
	return tierMap
}

// ClassifyServiceType 判断服务类型
func ClassifyServiceType(image string) string {
	img := strings.ToLower(image)
	switch {
	case IsDatabaseImage(img):
		return "db"
	case IsCacheImage(img):
		return "cache"
	case IsProxyImage(img):
		return "proxy"
	case IsQueueImage(img):
		return "queue"
	case IsStorageImage(img):
		return "storage"
	default:
		return "app"
	}
}

// IsDatabaseImage 判断是否为数据库镜像
func IsDatabaseImage(img string) bool {
	for _, db := range []string{"mysql", "mariadb", "postgres", "postgresql", "mongo", "mssql", "clickhouse", "influxdb", "cockroach"} {
		if strings.Contains(img, db) {
			return true
		}
	}
	return false
}

// IsCacheImage 判断是否为缓存镜像
func IsCacheImage(img string) bool {
	for _, c := range []string{"redis", "memcached", "varnish", "hazelcast"} {
		if strings.Contains(img, c) {
			return true
		}
	}
	return false
}

// IsProxyImage 判断是否为代理镜像
func IsProxyImage(img string) bool {
	for _, p := range []string{"nginx", "traefik", "haproxy", "caddy", "apache", "httpd", "envoy", "kong"} {
		if strings.Contains(img, p) {
			return true
		}
	}
	return false
}

// IsQueueImage 判断是否为消息队列镜像
func IsQueueImage(img string) bool {
	for _, q := range []string{"rabbitmq", "kafka", "nats", "activemq", "zeromq", "pulsar"} {
		if strings.Contains(img, q) {
			return true
		}
	}
	return false
}

// IsStorageImage 判断是否为存储镜像
func IsStorageImage(img string) bool {
	for _, s := range []string{"minio", "nextcloud", "owncloud", "seafile"} {
		if strings.Contains(img, s) {
			return true
		}
	}
	return false
}

// IsInfraImage 判断是否为基础设
// IsInfraImage 判断是否为基础设施镜像
func IsInfraImage(img string) bool {
	for _, infra := range []string{"portainer", "watchtower", "cadvisor", "prometheus", "grafana", "loki", "fluentd", "elasticsearch", "kibana", "consul", "etcd", "vault"} {
		if strings.Contains(img, infra) {
			return true
		}
	}
	return false
}

// CalculateStartOrder 计算服务启动顺序（拓扑排序 - BFS分层法）
func CalculateStartOrder(services map[string]*ServiceNode) [][]string {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)
	for name := range services {
		inDegree[name] = 0
	}
	for name, svc := range services {
		for _, dep := range svc.DependsOn {
			if _, ok := services[dep]; ok {
				graph[dep] = append(graph[dep], name)
				inDegree[name]++
			}
		}
	}
	result := make([][]string, 0)
	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	sort.Strings(queue)
	for len(queue) > 0 {
		current := make([]string, len(queue))
		copy(current, queue)
		result = append(result, current)
		nextQueue := make([]string, 0)
		for _, node := range queue {
			for _, neighbor := range graph[node] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					nextQueue = append(nextQueue, neighbor)
				}
			}
		}
		sort.Strings(nextQueue)
		queue = nextQueue
	}
	return result
}

// SuggestResources 根据镜像类型推荐资源限制
func SuggestResources(image string) *ResourceLimits {
	img := strings.ToLower(image)
	switch {
	case IsDatabaseImage(img):
		return &ResourceLimits{CPUs: "2.0", Memory: "2G", Reservations: &ResourceReservation{CPUs: "0.5", Memory: "512M"}}
	case IsCacheImage(img):
		return &ResourceLimits{CPUs: "1.0", Memory: "512M", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "128M"}}
	case IsProxyImage(img):
		return &ResourceLimits{CPUs: "1.0", Memory: "256M", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "64M"}}
	case IsQueueImage(img):
		return &ResourceLimits{CPUs: "1.5", Memory: "1G", Reservations: &ResourceReservation{CPUs: "0.5", Memory: "256M"}}
	case IsStorageImage(img):
		return &ResourceLimits{CPUs: "1.0", Memory: "1G", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "128M"}}
	default:
		return &ResourceLimits{CPUs: "1.0", Memory: "512M", Reservations: &ResourceReservation{CPUs: "0.25", Memory: "64M"}}
	}
}

// ParsePortMapping 解析端口映射 "8080:80/tcp"
func ParsePortMapping(s string) *PortMapping {
	protocol := "tcp"
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 2 {
		protocol = parts[1]
	}
	addr := parts[0]
	colonParts := strings.Split(addr, ":")
	var hostPort, containerPort int
	var ip string
	switch len(colonParts) {
	case 3: // ip:host:container
		ip = colonParts[0]
		fmt.Sscanf(colonParts[1], "%d", &hostPort)
		fmt.Sscanf(colonParts[2], "%d", &containerPort)
	case 2: // host:container
		fmt.Sscanf(colonParts[0], "%d", &hostPort)
		fmt.Sscanf(colonParts[1], "%d", &containerPort)
	case 1: // container only
		fmt.Sscanf(colonParts[0], "%d", &containerPort)
		hostPort = containerPort
	default:
		return nil
	}
	return &PortMapping{HostPort: hostPort, ContainerPort: containerPort, Protocol: protocol, IP: ip}
}

// ParseVolumeMapping 解析卷映射 "source:target:ro"
func ParseVolumeMapping(s string) *VolumeMapping {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return nil
	}
	vm := &VolumeMapping{Source: parts[0], Target: parts[1], Type: "volume"}
	if len(parts) > 2 && parts[2] == "ro" {
		vm.ReadOnly = true
	}
	// 判断是否为 bind mount（绝对路径）
	if strings.HasPrefix(parts[0], "/") || strings.HasPrefix(parts[0], "./") || strings.HasPrefix(parts[0], "~") {
		vm.Type = "bind"
	}
	return vm
}

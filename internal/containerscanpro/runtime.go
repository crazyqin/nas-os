package containerscanpro

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RuntimeScanner 运行时监控器
type RuntimeScanner struct {
	mu          sync.RWMutex
	config      *ScanConfig
	anomalies   []RuntimeAnomaly
	monitors    map[string]*ContainerMonitor
	stopCh      chan struct{}
	anomalyChan chan RuntimeAnomaly
}

// ContainerMonitor 单容器监控器
type ContainerMonitor struct {
	ContainerID   string
	ContainerName string
	StartTime     time.Time
	LastCheck     time.Time
	AnomalyCount  int
	stopCh        chan struct{}
}

// NewRuntimeScanner 创建运行时监控器
func NewRuntimeScanner(config *ScanConfig) *RuntimeScanner {
	return &RuntimeScanner{
		config:      config,
		anomalies:   make([]RuntimeAnomaly, 0),
		monitors:    make(map[string]*ContainerMonitor),
		stopCh:      make(chan struct{}),
		anomalyChan: make(chan RuntimeAnomaly, 100),
	}
}

// Start 启动监控
func (rs *RuntimeScanner) Start(ctx context.Context) error {
	log.Println("[RuntimeScanner] Starting runtime monitoring...")

	go rs.monitorLoop(ctx)
	go rs.anomalyProcessor(ctx)

	return nil
}

// Stop 停止监控
func (rs *RuntimeScanner) Stop() {
	log.Println("[RuntimeScanner] Stopping runtime monitoring...")
	close(rs.stopCh)
}

// monitorLoop 监控循环
func (rs *RuntimeScanner) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rs.stopCh:
			return
		case <-ticker.C:
			rs.checkAllContainers(ctx)
		}
	}
}

// anomalyProcessor 异常处理器
func (rs *RuntimeScanner) anomalyProcessor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-rs.stopCh:
			return
		case anomaly := <-rs.anomalyChan:
			rs.mu.Lock()
			rs.anomalies = append(rs.anomalies, anomaly)
			rs.mu.Unlock()

			log.Printf("[RuntimeScanner] Anomaly detected: %s in container %s",
				anomaly.Type, anomaly.ContainerID)
		}
	}
}

// checkAllContainers 检查所有容器
func (rs *RuntimeScanner) checkAllContainers(ctx context.Context) {
	containers, err := rs.listRunningContainers(ctx)
	if err != nil {
		log.Printf("[RuntimeScanner] Failed to list containers: %v", err)
		return
	}

	for _, container := range containers {
		select {
		case <-ctx.Done():
			return
		case <-rs.stopCh:
			return
		default:
			rs.checkContainer(ctx, container)
		}
	}
}

// listRunningContainers 列出运行中的容器
func (rs *RuntimeScanner) listRunningContainers(ctx context.Context) ([]map[string]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}|{{.Names}}|{{.Image}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var containers []map[string]string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) >= 2 {
			containers = append(containers, map[string]string{
				"id":    parts[0],
				"name":  parts[1],
				"image": parts[2],
			})
		}
	}

	return containers, nil
}

// checkContainer 检查单个容器
func (rs *RuntimeScanner) checkContainer(ctx context.Context, container map[string]string) {
	containerID := container["id"]
	containerName := container["name"]

	// 检查是否在排除列表中
	if rs.isExcluded(containerID, containerName) {
		return
	}

	// 并发执行多项检查
	var wg sync.WaitGroup
	checks := []func(context.Context, string, string){
		rs.checkSuspiciousProcesses,
		rs.checkFileModifications,
		rs.checkNetworkConnections,
		rs.checkPrivilegeEscalation,
		rs.checkResourceUsage,
	}

	for _, check := range checks {
		wg.Add(1)
		go func(fn func(context.Context, string, string)) {
			defer wg.Done()
			fn(ctx, containerID, containerName)
		}(check)
	}

	wg.Wait()
}

// checkSuspiciousProcesses 检查可疑进程
func (rs *RuntimeScanner) checkSuspiciousProcesses(ctx context.Context, containerID, containerName string) {
	suspiciousCommands := []string{
		"nc", "ncat", "netcat", "nmap", "tcpdump", "wireshark",
		"curl", "wget", "ssh", "telnet", "ftp", "tftp",
		"python", "perl", "ruby", "php",
		"base64", "openssl",
	}

	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return // 容器可能不支持 ps
	}

	processList := string(output)
	for _, suspicious := range suspiciousCommands {
		if strings.Contains(processList, suspicious) {
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalySuspiciousProcess,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("Suspicious process '%s' detected in container %s", suspicious, containerName),
				Details: map[string]string{
					"process":        suspicious,
					"container_name": containerName,
				},
				Severity: SeverityMedium,
			}
		}
	}
}

// checkFileModifications 检查文件修改
func (rs *RuntimeScanner) checkFileModifications(ctx context.Context, containerID, containerName string) {
	// 检查关键目录的修改
	criticalPaths := []string{
		"/etc/passwd",
		"/etc/shadow",
		"/etc/sudoers",
		"/root/.ssh",
		"/usr/bin",
		"/usr/sbin",
	}

	for _, path := range criticalPaths {
		cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "stat", "-c", "%Y", path)
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		modTime := strings.TrimSpace(string(output))
		if modTime != "" {
			// 检查是否最近修改（1小时内）
			// 这里简化处理，实际应该记录基线并对比
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalyFileModification,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("Critical file '%s' was modified in container %s", path, containerName),
				Details: map[string]string{
					"file_path":      path,
					"container_name": containerName,
				},
				Severity: SeverityHigh,
			}
		}
	}
}

// checkNetworkConnections 检查网络连接
func (rs *RuntimeScanner) checkNetworkConnections(ctx context.Context, containerID, containerName string) {
	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "netstat", "-tlnp")
	output, err := cmd.Output()
	if err != nil {
		// 尝试使用 ss
		cmd = exec.CommandContext(ctx, "docker", "exec", containerID, "ss", "-tlnp")
		output, err = cmd.Output()
		if err != nil {
			return
		}
	}

	connectionList := string(output)

	// 检查异常端口
	suspiciousPorts := []string{":4444", ":5555", ":6666", ":7777", ":8888", ":9999", ":1234", ":31337"}
	for _, port := range suspiciousPorts {
		if strings.Contains(connectionList, port) {
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalyNetworkConnection,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("Suspicious port %s listening in container %s", port, containerName),
				Details: map[string]string{
					"port":           port,
					"container_name": containerName,
				},
				Severity: SeverityHigh,
			}
		}
	}

	// 检查外部连接
	cmd = exec.CommandContext(ctx, "docker", "exec", containerID, "netstat", "-tn")
	output, err = cmd.Output()
	if err != nil {
		return
	}

	externalConnections := string(output)
	lines := strings.Split(externalConnections, "\n")
	externalCount := 0
	for _, line := range lines {
		if strings.Contains(line, "ESTABLISHED") && !strings.Contains(line, "127.0.0.1") {
			externalCount++
		}
	}

	if externalCount > 10 {
		rs.anomalyChan <- RuntimeAnomaly{
			Type:        AnomalyNetworkConnection,
			ContainerID: containerID,
			Timestamp:   time.Now(),
			Description: fmt.Sprintf("High number of external connections (%d) in container %s", externalCount, containerName),
			Details: map[string]string{
				"connection_count": fmt.Sprintf("%d", externalCount),
				"container_name":   containerName,
			},
			Severity: SeverityMedium,
		}
	}
}

// checkPrivilegeEscalation 检查权限提升
func (rs *RuntimeScanner) checkPrivilegeEscalation(ctx context.Context, containerID, containerName string) {
	// 检查是否有 suid 文件被创建
	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "find", "/", "-perm", "-4000", "-type", "f", "2>/dev/null")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	suidFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
	knownSUID := map[string]bool{
		"/usr/bin/passwd":  true,
		"/usr/bin/su":      true,
		"/usr/bin/sudo":    true,
		"/usr/bin/newgrp":  true,
		"/usr/bin/chsh":    true,
		"/usr/bin/chfn":    true,
		"/usr/bin/gpasswd": true,
		"/usr/bin/mount":   true,
		"/usr/bin/umount":  true,
	}

	for _, file := range suidFiles {
		file = strings.TrimSpace(file)
		if file != "" && !knownSUID[file] {
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalyPrivilegeEscalation,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("Unknown SUID file '%s' found in container %s", file, containerName),
				Details: map[string]string{
					"suid_file":      file,
					"container_name": containerName,
				},
				Severity: SeverityCritical,
			}
		}
	}

	// 检查 capabilities
	cmd = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.HostConfig.Privileged}}", containerID)
	output, err = cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) == "true" {
		rs.anomalyChan <- RuntimeAnomaly{
			Type:        AnomalyPrivilegeEscalation,
			ContainerID: containerID,
			Timestamp:   time.Now(),
			Description: fmt.Sprintf("Container %s is running in privileged mode", containerName),
			Details: map[string]string{
				"container_name": containerName,
				"privileged":     "true",
			},
			Severity: SeverityCritical,
		}
	}
}

// checkResourceUsage 检查资源使用
func (rs *RuntimeScanner) checkResourceUsage(ctx context.Context, containerID, containerName string) {
	cmd := exec.CommandContext(ctx, "docker", "stats", "--no-stream", "--format",
		"{{.CPUPerc}}|{{.MemPerc}}|{{.NetIO}}|{{.BlockIO}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	stats := strings.Split(strings.TrimSpace(string(output)), "|")
	if len(stats) < 2 {
		return
	}

	// 检查 CPU 使用率
	cpuStr := strings.TrimSuffix(stats[0], "%")
	var cpu float64
	if _, err := fmt.Sscanf(cpuStr, "%f", &cpu); err == nil {
		if cpu > 90 {
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalyResourceAbuse,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("High CPU usage (%.1f%%) in container %s", cpu, containerName),
				Details: map[string]string{
					"cpu_usage":      fmt.Sprintf("%.1f%%", cpu),
					"container_name": containerName,
				},
				Severity: SeverityMedium,
			}
		}
	}

	// 检查内存使用率
	memStr := strings.TrimSuffix(stats[1], "%")
	var mem float64
	if _, err := fmt.Sscanf(memStr, "%f", &mem); err == nil {
		if mem > 90 {
			rs.anomalyChan <- RuntimeAnomaly{
				Type:        AnomalyResourceAbuse,
				ContainerID: containerID,
				Timestamp:   time.Now(),
				Description: fmt.Sprintf("High memory usage (%.1f%%) in container %s", mem, containerName),
				Details: map[string]string{
					"mem_usage":      fmt.Sprintf("%.1f%%", mem),
					"container_name": containerName,
				},
				Severity: SeverityMedium,
			}
		}
	}
}

// isExcluded 检查容器是否在排除列表中
func (rs *RuntimeScanner) isExcluded(containerID, containerName string) bool {
	for _, excluded := range rs.config.ExcludedContainers {
		if excluded == containerID || excluded == containerName {
			return true
		}
	}
	return false
}

// GetAnomalies 获取所有异常
func (rs *RuntimeScanner) GetAnomalies() []RuntimeAnomaly {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	anomalies := make([]RuntimeAnomaly, len(rs.anomalies))
	copy(anomalies, rs.anomalies)
	return anomalies
}

// GetAnomaliesByContainer 获取指定容器的异常
func (rs *RuntimeScanner) GetAnomaliesByContainer(containerID string) []RuntimeAnomaly {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	var anomalies []RuntimeAnomaly
	for _, a := range rs.anomalies {
		if a.ContainerID == containerID {
			anomalies = append(anomalies, a)
		}
	}
	return anomalies
}

// ClearAnomalies 清除异常记录
func (rs *RuntimeScanner) ClearAnomalies() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.anomalies = make([]RuntimeAnomaly, 0)
}

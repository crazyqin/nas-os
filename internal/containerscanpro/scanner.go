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

// Scanner 容器安全扫描器
type Scanner struct {
	mu          sync.RWMutex
	config      *ScanConfig
	cveDB       *CVEDatabase
	runtime     *RuntimeScanner
	alerter     *Alerter
	stats       *ScanStats
	scanResults map[string]*ScanResult
	scanChan    chan string
	stopCh      chan struct{}
}

// NewScanner 创建扫描器
func NewScanner(config *ScanConfig, cveDB *CVEDatabase, runtime *RuntimeScanner, alerter *Alerter) *Scanner {
	return &Scanner{
		config:      config,
		cveDB:       cveDB,
		runtime:     runtime,
		alerter:     alerter,
		stats:       &ScanStats{},
		scanResults: make(map[string]*ScanResult),
		scanChan:    make(chan string, 100),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动扫描器
func (s *Scanner) Start(ctx context.Context) error {
	log.Println("[Scanner] Starting scanner...")

	// 启动扫描处理器
	for i := 0; i < s.config.MaxConcurrent; i++ {
		go s.scanWorker(ctx)
	}

	// 启动定期扫描
	if s.config.ScanInterval > 0 {
		go s.scheduledScan(ctx)
	}

	return nil
}

// Stop 停止扫描器
func (s *Scanner) Stop() {
	close(s.stopCh)
}

// scanWorker 扫描工作器
func (s *Scanner) scanWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case containerID := <-s.scanChan:
			result := s.scanContainer(ctx, containerID)
			s.storeResult(result)
		}
	}
}

// scheduledScan 定期扫描
func (s *Scanner) scheduledScan(ctx context.Context) {
	ticker := time.NewTicker(s.config.ScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			log.Println("[Scanner] Starting scheduled scan...")
			s.ScanAllContainers(ctx)
		}
	}
}

// ScanContainer 扫描单个容器
func (s *Scanner) ScanContainer(ctx context.Context, containerID string) *ScanResult {
	return s.scanContainer(ctx, containerID)
}

// ScanAllContainers 扫描所有容器
func (s *Scanner) ScanAllContainers(ctx context.Context) ([]*ScanResult, error) {
	containers, err := s.listContainers(ctx)
	if err != nil {
		return nil, err
	}

	var results []*ScanResult
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, container := range containers {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		case <-s.stopCh:
			return results, nil
		default:
			wg.Add(1)
			go func(containerID string) {
				defer wg.Done()
				result := s.scanContainer(ctx, containerID)
				mu.Lock()
				results = append(results, result)
				mu.Unlock()
			}(container["id"])
		}
	}

	wg.Wait()
	return results, nil
}

// ScanImage 扫描镜像
func (s *Scanner) ScanImage(ctx context.Context, imageName string) *ScanResult {
	scanID := fmt.Sprintf("scan-img-%d", time.Now().UnixNano())
	startTime := time.Now()

	result := &ScanResult{
		ScanID:    scanID,
		ImageName: imageName,
		Status:    StatusRunning,
		StartTime: startTime,
	}

	s.stats.IncrementScans()

	// 获取镜像信息
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("Failed to inspect image: %v", err)
		endTime := time.Now()
		result.EndTime = &endTime
		s.stats.IncrementFailed()
		return result
	}
	result.ImageID = strings.TrimSpace(string(output))

	// 扫描镜像层
	packages := s.scanImageLayers(ctx, imageName)

	// 匹配 CVE
	vulns := s.cveDB.MatchPackages(packages)
	for i := range vulns {
		vulns[i].ImageName = imageName
	}
	result.Vulnerabilities = vulns

	// 计算安全评分
	result.Score = s.calculateScore(vulns, nil)

	// 生成修复建议
	result.Recommendations = s.generateRecommendations(vulns, nil)

	// 发送告警
	s.sendAlerts(result)

	endTime := time.Now()
	result.EndTime = &endTime
	result.Status = StatusCompleted

	s.stats.IncrementCompleted()
	s.stats.UpdateLastScan()

	// 统计漏洞
	critical, high, medium, low := countVulnsBySeverity(vulns)
	s.stats.AddVulns(critical, high, medium, low)

	return result
}

// scanContainer 扫描容器内部
func (s *Scanner) scanContainer(ctx context.Context, containerID string) *ScanResult {
	scanID := fmt.Sprintf("scan-%s-%d", containerID[:8], time.Now().UnixNano())
	startTime := time.Now()

	result := &ScanResult{
		ScanID:      scanID,
		ContainerID: containerID,
		Status:      StatusRunning,
		StartTime:   startTime,
	}

	s.stats.IncrementScans()

	// 检查容器是否在排除列表
	if s.isExcluded(containerID) {
		result.Status = StatusCompleted
		endTime := time.Now()
		result.EndTime = &endTime
		return result
	}

	// 获取容器信息
	containerInfo, err := s.getContainerInfo(ctx, containerID)
	if err != nil {
		result.Status = StatusFailed
		result.Error = fmt.Sprintf("Failed to get container info: %v", err)
		endTime := time.Now()
		result.EndTime = &endTime
		s.stats.IncrementFailed()
		return result
	}

	result.ContainerName = containerInfo["name"]
	result.ImageName = containerInfo["image"]
	result.ImageID = containerInfo["image_id"]

	// CVE 扫描
	if s.config.EnableCVEScan {
		packages := s.scanContainerPackages(ctx, containerID)
		vulns := s.cveDB.MatchPackages(packages)
		for i := range vulns {
			vulns[i].ContainerID = containerID
			vulns[i].ImageName = result.ImageName
		}
		result.Vulnerabilities = vulns
	}

	// 运行时异常检查
	if s.config.EnableRuntimeMonitor {
		anomalies := s.runtime.GetAnomaliesByContainer(containerID)
		result.Anomalies = anomalies
	}

	// 计算安全评分
	result.Score = s.calculateScore(result.Vulnerabilities, result.Anomalies)

	// 生成修复建议
	result.Recommendations = s.generateRecommendations(result.Vulnerabilities, result.Anomalies)

	// 发送告警
	s.sendAlerts(result)

	endTime := time.Now()
	result.EndTime = &endTime
	result.Status = StatusCompleted

	s.stats.IncrementCompleted()
	s.stats.UpdateLastScan()

	// 统计漏洞
	critical, high, medium, low := countVulnsBySeverity(result.Vulnerabilities)
	s.stats.AddVulns(critical, high, medium, low)
	s.stats.AddAnomalies(len(result.Anomalies))

	return result
}

// listContainers 列出所有容器
func (s *Scanner) listContainers(ctx context.Context) ([]map[string]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a", "--format", "{{.ID}}|{{.Names}}|{{.Image}}")
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
		if len(parts) >= 3 {
			containers = append(containers, map[string]string{
				"id":    parts[0],
				"name":  parts[1],
				"image": parts[2],
			})
		}
	}

	return containers, nil
}

// getContainerInfo 获取容器详细信息
func (s *Scanner) getContainerInfo(ctx context.Context, containerID string) (map[string]string, error) {
	info := map[string]string{
		"id":       containerID,
		"name":     "",
		"image":    "",
		"image_id": "",
	}

	// 获取容器名称
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Name}}", containerID)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	info["name"] = strings.TrimPrefix(strings.TrimSpace(string(output)), "/")

	// 获取镜像信息
	cmd = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Config.Image}}", containerID)
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}
	info["image"] = strings.TrimSpace(string(output))

	// 获取镜像 ID
	cmd = exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Image}}", containerID)
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}
	info["image_id"] = strings.TrimSpace(string(output))

	return info, nil
}

// scanContainerPackages 扫描容器内安装的软件包
func (s *Scanner) scanContainerPackages(ctx context.Context, containerID string) []PackageInfo {
	var packages []PackageInfo

	// 尝试使用 dpkg (Debian/Ubuntu)
	cmd := exec.CommandContext(ctx, "docker", "exec", containerID, "dpkg-query", "-W", "-f", "${Package}|${Version}\n")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 2)
			if len(parts) == 2 {
				packages = append(packages, PackageInfo{
					Name:             parts[0],
					InstalledVersion: parts[1],
					Source:           "dpkg",
				})
			}
		}
		return packages
	}

	// 尝试使用 rpm (RHEL/CentOS)
	cmd = exec.CommandContext(ctx, "docker", "exec", containerID, "rpm", "-qa", "--queryformat", "%{NAME}|%{VERSION}-%{RELEASE}\n")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 2)
			if len(parts) == 2 {
				packages = append(packages, PackageInfo{
					Name:             parts[0],
					InstalledVersion: parts[1],
					Source:           "rpm",
				})
			}
		}
		return packages
	}

	// 尝试使用 apk (Alpine)
	cmd = exec.CommandContext(ctx, "docker", "exec", containerID, "apk", "list", "--installed")
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			// 格式: package-name version description
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				packages = append(packages, PackageInfo{
					Name:             parts[0],
					InstalledVersion: parts[1],
					Source:           "apk",
				})
			}
		}
		return packages
	}

	return packages
}

// scanImageLayers 扫描镜像层
func (s *Scanner) scanImageLayers(ctx context.Context, imageName string) []PackageInfo {
	var packages []PackageInfo

	// 使用 docker history 获取层信息
	cmd := exec.CommandContext(ctx, "docker", "history", "--no-trunc", "--format", "{{.CreatedBy}}", imageName)
	output, err := cmd.Output()
	if err != nil {
		return packages
	}

	layers := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i, layer := range layers {
		layer = strings.TrimSpace(layer)
		if layer == "" || layer == "<missing>" {
			continue
		}

		// 解析安装命令
		if strings.Contains(layer, "apt-get install") || strings.Contains(layer, "apk add") || strings.Contains(layer, "yum install") {
			// 提取包名（简化版本）
			packages = append(packages, PackageInfo{
				Name:   fmt.Sprintf("layer-%d", i),
				Source: layer,
			})
		}
	}

	return packages
}

// calculateScore 计算安全评分
func (s *Scanner) calculateScore(vulns []VulnerabilityCVE, anomalies []RuntimeAnomaly) *SecurityScore {
	score := &SecurityScore{
		Breakdown: make(map[string]float64),
	}

	// 基础分 100
	vulnScore := 100.0
	runtimeScore := 100.0
	configScore := 100.0

	// CVE 评分扣分
	for _, vuln := range vulns {
		switch vuln.Severity {
		case SeverityCritical:
			vulnScore -= 15
		case SeverityHigh:
			vulnScore -= 10
		case SeverityMedium:
			vulnScore -= 5
		case SeverityLow:
			vulnScore -= 2
		case SeverityInfo:
			vulnScore -= 1
		}
	}

	// 运行时异常扣分
	for _, anomaly := range anomalies {
		switch anomaly.Severity {
		case SeverityCritical:
			runtimeScore -= 20
		case SeverityHigh:
			runtimeScore -= 15
		case SeverityMedium:
			runtimeScore -= 10
		case SeverityLow:
			runtimeScore -= 5
		case SeverityInfo:
			runtimeScore -= 2
		}
	}

	// 确保分数不小于 0
	if vulnScore < 0 {
		vulnScore = 0
	}
	if runtimeScore < 0 {
		runtimeScore = 0
	}

	score.VulnScore = vulnScore
	score.RuntimeScore = runtimeScore
	score.ConfigScore = configScore

	// 综合评分 (加权平均)
	score.Overall = (vulnScore*0.5 + runtimeScore*0.3 + configScore*0.2)

	score.Breakdown["vulnerability"] = vulnScore
	score.Breakdown["runtime"] = runtimeScore
	score.Breakdown["config"] = configScore

	// 评级
	switch {
	case score.Overall >= 90:
		score.Grade = "A"
	case score.Overall >= 80:
		score.Grade = "B"
	case score.Overall >= 70:
		score.Grade = "C"
	case score.Overall >= 60:
		score.Grade = "D"
	default:
		score.Grade = "F"
	}

	return score
}

// generateRecommendations 生成修复建议
func (s *Scanner) generateRecommendations(vulns []VulnerabilityCVE, anomalies []RuntimeAnomaly) []string {
	var recommendations []string
	recommendationSet := make(map[string]bool)

	// 基于漏洞的建议
	for _, vuln := range vulns {
		// 通用建议
		rec := fmt.Sprintf("Update %s to fix %s", vuln.Package, vuln.CVEID)
		if !recommendationSet[rec] {
			recommendations = append(recommendations, rec)
			recommendationSet[rec] = true
		}

		// 特定漏洞建议
		if vuln.FixVersion != "" {
			rec := fmt.Sprintf("Upgrade %s to version %s or later", vuln.Package, vuln.FixVersion)
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		}
	}

	// 基于异常的建议
	for _, anomaly := range anomalies {
		switch anomaly.Type {
		case AnomalySuspiciousProcess:
			rec := "Review and remove suspicious processes from container"
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		case AnomalyNetworkConnection:
			rec := "Restrict container network access and review exposed ports"
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		case AnomalyPrivilegeEscalation:
			rec := "Remove SUID binaries and run container with minimal privileges"
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		case AnomalyFileModification:
			rec := "Use read-only filesystem and restrict file modifications"
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		case AnomalyResourceAbuse:
			rec := "Set resource limits (CPU/memory) for container"
			if !recommendationSet[rec] {
				recommendations = append(recommendations, rec)
				recommendationSet[rec] = true
			}
		}
	}

	// 通用安全建议
	generalRecs := []string{
		"Use minimal base images (Alpine, distroless)",
		"Enable container scanning in CI/CD pipeline",
		"Regularly update base images and dependencies",
		"Implement network policies to restrict container communication",
		"Use security profiles (AppArmor, SELinux, Seccomp)",
	}

	for _, rec := range generalRecs {
		if len(recommendations) < 15 && !recommendationSet[rec] {
			recommendations = append(recommendations, rec)
			recommendationSet[rec] = true
		}
	}

	return recommendations
}

// sendAlerts 发送告警
func (s *Scanner) sendAlerts(result *ScanResult) {
	// 漏洞告警
	for _, vuln := range result.Vulnerabilities {
		if vuln.Severity == SeverityCritical || vuln.Severity == SeverityHigh {
			alert := Alert{
				Level:   AlertLevelCritical,
				Title:   fmt.Sprintf("CVE Vulnerability: %s", vuln.CVEID),
				Message: fmt.Sprintf("Container %s has vulnerability %s (Score: %.1f)", result.ContainerID, vuln.CVEID, vuln.CVSS),
				Source:  "cve-scanner",
				Details: map[string]string{
					"cve_id":       vuln.CVEID,
					"severity":     vuln.Severity,
					"score":        fmt.Sprintf("%.1f", vuln.CVSS),
					"package":      vuln.Package,
					"container_id": result.ContainerID,
					"image_name":   result.ImageName,
				},
			}
			s.alerter.SendAlert(alert)
		}
	}

	// 异常告警
	for _, anomaly := range result.Anomalies {
		level := AlertLevelWarning
		if anomaly.Severity == SeverityCritical {
			level = AlertLevelCritical
		}
		alert := Alert{
			Level:   level,
			Title:   fmt.Sprintf("Runtime Anomaly: %s", anomaly.Type),
			Message: anomaly.Description,
			Source:  "runtime-monitor",
			Details: map[string]string{
				"anomaly_type": anomaly.Type,
				"container_id": anomaly.ContainerID,
				"severity":     anomaly.Severity,
			},
		}
		s.alerter.SendAlert(alert)
	}

	// 评分告警
	if result.Score != nil && result.Score.Overall < 60 {
		level := AlertLevelWarning
		if result.Score.Overall < 30 {
			level = AlertLevelCritical
		}
		alert := Alert{
			Level:   level,
			Title:   fmt.Sprintf("Security Score Alert: %s", result.ContainerID),
			Message: fmt.Sprintf("Container %s has security score %.1f (Grade: %s)", result.ContainerID, result.Score.Overall, result.Score.Grade),
			Source:  "security-scorer",
			Details: map[string]string{
				"container_id": result.ContainerID,
				"score":        fmt.Sprintf("%.1f", result.Score.Overall),
				"grade":        result.Score.Grade,
			},
		}
		s.alerter.SendAlert(alert)
	}
}

// isExcluded 检查容器是否在排除列表
func (s *Scanner) isExcluded(containerID string) bool {
	for _, excluded := range s.config.ExcludedContainers {
		if excluded == containerID {
			return true
		}
	}
	return false
}

// storeResult 存储扫描结果
func (s *Scanner) storeResult(result *ScanResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanResults[result.ScanID] = result
}

// GetResult 获取扫描结果
func (s *Scanner) GetResult(scanID string) (*ScanResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.scanResults[scanID]
	return result, ok
}

// GetResultsByContainer 获取容器扫描结果
func (s *Scanner) GetResultsByContainer(containerID string) []*ScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*ScanResult
	for _, r := range s.scanResults {
		if r.ContainerID == containerID {
			results = append(results, r)
		}
	}
	return results
}

// GetStats 获取统计信息
func (s *Scanner) GetStats() *ScanStats {
	return s.stats.GetStats()
}

// countVulnsBySeverity 统计各严重程度漏洞数量
func countVulnsBySeverity(vulns []VulnerabilityCVE) (critical, high, medium, low int) {
	for _, v := range vulns {
		switch v.Severity {
		case SeverityCritical:
			critical++
		case SeverityHigh:
			high++
		case SeverityMedium:
			medium++
		case SeverityLow:
			low++
		}
	}
	return
}

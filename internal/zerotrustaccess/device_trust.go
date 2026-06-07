// Package zerotrustaccess 提供设备指纹识别和信任评估
package zerotrustaccess

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ========== 设备指纹 ==========

// DeviceFingerprint 设备指纹
type DeviceFingerprint struct {
	DeviceID      string                `json:"device_id"`
	HardwareHash  string                `json:"hardware_hash"`
	OSHash        string                `json:"os_hash"`
	NetworkHash   string                `json:"network_hash"`
	BrowserHash   string                `json:"browser_hash,omitempty"`
	CompositeHash string                `json:"composite_hash"`
	Components    FingerprintComponents `json:"components"`
	CollectedAt   time.Time             `json:"collected_at"`
	ExpiresAt     time.Time             `json:"expires_at"`
	TrustScore    float64               `json:"trust_score"` // 0-100
	IsStable      bool                  `json:"is_stable"`   // 指纹是否稳定
}

// FingerprintComponents 指纹组件
type FingerprintComponents struct {
	Hardware HardwareFingerprint `json:"hardware"`
	OS       OSFingerprint       `json:"os"`
	Network  NetworkFingerprint  `json:"network"`
	Browser  BrowserFingerprint  `json:"browser,omitempty"`
}

// HardwareFingerprint 硬件指纹
type HardwareFingerprint struct {
	CPUModel    string `json:"cpu_model"`
	CPUCores    int    `json:"cpu_cores"`
	MemoryMB    int64  `json:"memory_mb"`
	DiskSerial  string `json:"disk_serial"`
	MACAddress  string `json:"mac_address"`
	BIOSVersion string `json:"bios_version"`
	BoardSerial string `json:"board_serial"`
	GPUModel    string `json:"gpu_model,omitempty"`
}

// OSFingerprint 操作系统指纹
type OSFingerprint struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	Build         string `json:"build"`
	Arch          string `json:"arch"`
	Hostname      string `json:"hostname"`
	KernelVersion string `json:"kernel_version"`
}

// NetworkFingerprint 网络指纹
type NetworkFingerprint struct {
	ExternalIP  string   `json:"external_ip"`
	InternalIP  string   `json:"internal_ip"`
	SubnetMask  string   `json:"subnet_mask"`
	Gateway     string   `json:"gateway"`
	DNSServers  []string `json:"dns_servers"`
	DHCPEnabled bool     `json:"dhcp_enabled"`
	VPNActive   bool     `json:"vpn_active"`
}

// BrowserFingerprint 浏览器指纹
type BrowserFingerprint struct {
	UserAgent string   `json:"user_agent"`
	Language  string   `json:"language"`
	Timezone  string   `json:"timezone"`
	ScreenRes string   `json:"screen_res"`
	Fonts     []string `json:"fonts,omitempty"`
	Plugins   []string `json:"plugins,omitempty"`
	WebGL     string   `json:"webgl,omitempty"`
}

// ========== 信任评估 ==========

// TrustAssessment 信任评估
type TrustAssessment struct {
	DeviceID        string        `json:"device_id"`
	OverallScore    float64       `json:"overall_score"` // 0-100
	TrustLevel      TrustLevel    `json:"trust_level"`
	Factors         []TrustFactor `json:"factors"`
	Anomalies       []Anomaly     `json:"anomalies"`
	Recommendations []string      `json:"recommendations"`
	AssessedAt      time.Time     `json:"assessed_at"`
	ValidUntil      time.Time     `json:"valid_until"`
}

// TrustFactor 信任因素
type TrustFactor struct {
	Name     string  `json:"name"`
	Category string  `json:"category"` // "hardware", "software", "behavior", "compliance"
	Score    float64 `json:"score"`    // 0-100
	Weight   float64 `json:"weight"`   // 权重 0-1
	Details  string  `json:"details"`
}

// Anomaly 异常
type Anomaly struct {
	Type        string    `json:"type"`     // "fingerprint_change", "location_anomaly", "behavior_anomaly"
	Severity    string    `json:"severity"` // "low", "medium", "high", "critical"
	Description string    `json:"description"`
	DetectedAt  time.Time `json:"detected_at"`
}

// ========== 设备信任管理器 ==========

// DeviceTrustManager 设备信任管理器
type DeviceTrustManager struct {
	mu           sync.RWMutex
	fingerprints map[string]*DeviceFingerprint
	assessments  map[string]*TrustAssessment
	anomalies    map[string][]Anomaly
	config       *TrustConfig
}

// TrustConfig 信任配置
type TrustConfig struct {
	FingerprintExpiry time.Duration `json:"fingerprint_expiry"`
	AssessmentExpiry  time.Duration `json:"assessment_expiry"`
	MaxAnomalyAge     time.Duration `json:"max_anomaly_age"`
	MinTrustScore     float64       `json:"min_trust_score"`
	AnomalyThreshold  int           `json:"anomaly_threshold"`
	ComplianceWeight  float64       `json:"compliance_weight"`
	HardwareWeight    float64       `json:"hardware_weight"`
	BehaviorWeight    float64       `json:"behavior_weight"`
	NetworkWeight     float64       `json:"network_weight"`
}

// NewDeviceTrustManager 创建设备信任管理器
func NewDeviceTrustManager(config *TrustConfig) *DeviceTrustManager {
	if config == nil {
		config = &TrustConfig{
			FingerprintExpiry: 30 * 24 * time.Hour,
			AssessmentExpiry:  24 * time.Hour,
			MaxAnomalyAge:     7 * 24 * time.Hour,
			MinTrustScore:     50.0,
			AnomalyThreshold:  5,
			ComplianceWeight:  0.3,
			HardwareWeight:    0.3,
			BehaviorWeight:    0.25,
			NetworkWeight:     0.15,
		}
	}

	return &DeviceTrustManager{
		fingerprints: make(map[string]*DeviceFingerprint),
		assessments:  make(map[string]*TrustAssessment),
		anomalies:    make(map[string][]Anomaly),
		config:       config,
	}
}

// CollectFingerprint 收集设备指纹
func (m *DeviceTrustManager) CollectFingerprint(deviceID string, components FingerprintComponents) (*DeviceFingerprint, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算各组件哈希
	hardwareHash := computeHash(fmt.Sprintf("%+v", components.Hardware))
	osHash := computeHash(fmt.Sprintf("%+v", components.OS))
	networkHash := computeHash(fmt.Sprintf("%+v", components.Network))

	var browserHash string
	if components.Browser.UserAgent != "" {
		browserHash = computeHash(fmt.Sprintf("%+v", components.Browser))
	}

	// 组合哈希
	compositeHash := computeHash(hardwareHash + osHash + browserHash)

	// 检查指纹变化
	oldFingerprint, exists := m.fingerprints[deviceID]
	isStable := true
	if exists {
		if oldFingerprint.CompositeHash != compositeHash {
			// 指纹变化，记录异常
			anomaly := Anomaly{
				Type:        "fingerprint_change",
				Severity:    "medium",
				Description: fmt.Sprintf("设备指纹发生变化: %s -> %s", oldFingerprint.CompositeHash[:8], compositeHash[:8]),
				DetectedAt:  time.Now(),
			}
			m.anomalies[deviceID] = append(m.anomalies[deviceID], anomaly)
			isStable = false
		}
	}

	fingerprint := &DeviceFingerprint{
		DeviceID:      deviceID,
		HardwareHash:  hardwareHash,
		OSHash:        osHash,
		NetworkHash:   networkHash,
		BrowserHash:   browserHash,
		CompositeHash: compositeHash,
		Components:    components,
		CollectedAt:   time.Now(),
		ExpiresAt:     time.Now().Add(m.config.FingerprintExpiry),
		IsStable:      isStable,
	}

	// 计算信任分数
	trustScore := m.calculateFingerprintTrustScore(fingerprint, exists)
	fingerprint.TrustScore = trustScore

	m.fingerprints[deviceID] = fingerprint
	return fingerprint, nil
}

// AssessTrust 评估设备信任
func (m *DeviceTrustManager) AssessTrust(deviceID string, device *Device) *TrustAssessment {
	m.mu.Lock()
	defer m.mu.Unlock()

	assessment := &TrustAssessment{
		DeviceID:        deviceID,
		AssessedAt:      time.Now(),
		ValidUntil:      time.Now().Add(m.config.AssessmentExpiry),
		Factors:         make([]TrustFactor, 0),
		Anomalies:       make([]Anomaly, 0),
		Recommendations: make([]string, 0),
	}

	// 获取指纹
	fingerprint, hasFingerprint := m.fingerprints[deviceID]

	// 计算各因素得分
	var totalScore float64
	var totalWeight float64

	// 1. 硬件信任
	if hasFingerprint {
		hardwareScore := m.assessHardwareTrust(fingerprint.Components.Hardware)
		assessment.Factors = append(assessment.Factors, TrustFactor{
			Name:     "hardware_trust",
			Category: "hardware",
			Score:    hardwareScore,
			Weight:   m.config.HardwareWeight,
			Details:  "硬件组件信任评估",
		})
		totalScore += hardwareScore * m.config.HardwareWeight
		totalWeight += m.config.HardwareWeight
	}

	// 2. 软件/OS信任
	if hasFingerprint {
		osScore := m.assessOSTrust(fingerprint.Components.OS, device)
		assessment.Factors = append(assessment.Factors, TrustFactor{
			Name:     "os_trust",
			Category: "software",
			Score:    osScore,
			Weight:   0.2,
			Details:  "操作系统信任评估",
		})
		totalScore += osScore * 0.2
		totalWeight += 0.2
	}

	// 3. 合规性信任
	complianceScore := m.assessComplianceTrust(device)
	assessment.Factors = append(assessment.Factors, TrustFactor{
		Name:     "compliance_trust",
		Category: "compliance",
		Score:    complianceScore,
		Weight:   m.config.ComplianceWeight,
		Details:  "合规性评估",
	})
	totalScore += complianceScore * m.config.ComplianceWeight
	totalWeight += m.config.ComplianceWeight

	// 4. 网络信任
	if hasFingerprint {
		networkScore := m.assessNetworkTrust(fingerprint.Components.Network)
		assessment.Factors = append(assessment.Factors, TrustFactor{
			Name:     "network_trust",
			Category: "network",
			Score:    networkScore,
			Weight:   m.config.NetworkWeight,
			Details:  "网络环境信任评估",
		})
		totalScore += networkScore * m.config.NetworkWeight
		totalWeight += m.config.NetworkWeight
	}

	// 5. 行为信任
	behaviorScore := m.assessBehaviorTrust(deviceID)
	assessment.Factors = append(assessment.Factors, TrustFactor{
		Name:     "behavior_trust",
		Category: "behavior",
		Score:    behaviorScore,
		Weight:   m.config.BehaviorWeight,
		Details:  "历史行为信任评估",
	})
	totalScore += behaviorScore * m.config.BehaviorWeight
	totalWeight += m.config.BehaviorWeight

	// 计算总体分数
	if totalWeight > 0 {
		assessment.OverallScore = totalScore / totalWeight
	}

	// 获取异常
	assessment.Anomalies = m.getRecentAnomalies(deviceID)

	// 确定信任级别
	assessment.TrustLevel = m.scoreToTrustLevel(assessment.OverallScore)

	// 生成建议
	assessment.Recommendations = m.generateRecommendations(assessment)

	// 存储评估
	m.assessments[deviceID] = assessment

	return assessment
}

// calculateFingerprintTrustScore 计算指纹信任分数
func (m *DeviceTrustManager) calculateFingerprintTrustScore(fp *DeviceFingerprint, existed bool) float64 {
	score := 70.0 // 基础分

	if existed {
		// 已存在设备加分
		score += 10
	}

	if fp.IsStable {
		// 指纹稳定加分
		score += 15
	}

	// 硬件完整性
	if fp.Components.Hardware.CPUCores > 0 {
		score += 5
	}

	return minFloat(score, 100)
}

// assessHardwareTrust 评估硬件信任
func (m *DeviceTrustManager) assessHardwareTrust(hw HardwareFingerprint) float64 {
	score := 80.0

	if hw.CPUCores == 0 || hw.MemoryMB == 0 {
		score -= 20
	}

	if hw.MACAddress == "" {
		score -= 15
	}

	if hw.DiskSerial == "" {
		score -= 10
	}

	return maxFloat(score, 0)
}

// assessOSTrust 评估OS信任
func (m *DeviceTrustManager) assessOSTrust(os OSFingerprint, device *Device) float64 {
	score := 85.0

	// 检查OS版本
	if os.Version == "" {
		score -= 20
	}

	// 检查设备状态
	if device != nil {
		switch device.Status {
		case DeviceStatusCompromised:
			score = 10
		case DeviceStatusNonCompliant:
			score -= 40
		case DeviceStatusBlocked:
			score = 0
		}
	}

	return maxFloat(score, 0)
}

// assessComplianceTrust 评估合规性信任
func (m *DeviceTrustManager) assessComplianceTrust(device *Device) float64 {
	if device == nil {
		return 50.0
	}

	// 基于设备合规分数
	compliance := device.Compliance

	// 健康检查
	if len(device.HealthChecks) > 0 {
		passCount := 0
		for _, check := range device.HealthChecks {
			if check.Status == "pass" {
				passCount++
			}
		}
		checkScore := float64(passCount) / float64(len(device.HealthChecks)) * 100
		compliance = (compliance + checkScore) / 2
	}

	return compliance
}

// assessNetworkTrust 评估网络信任
func (m *DeviceTrustManager) assessNetworkTrust(network NetworkFingerprint) float64 {
	score := 75.0

	// VPN激活可能表示远程访问，中等信任
	if network.VPNActive {
		score -= 5
	}

	// 检查IP
	if network.ExternalIP == "" {
		score -= 10
	}

	return maxFloat(score, 0)
}

// assessBehaviorTrust 评估行为信任
func (m *DeviceTrustManager) assessBehaviorTrust(deviceID string) float64 {
	score := 80.0

	// 检查异常记录
	anomalies := m.anomalies[deviceID]
	if len(anomalies) > m.config.AnomalyThreshold {
		score -= float64(len(anomalies)-m.config.AnomalyThreshold) * 10
	}

	return maxFloat(score, 0)
}

// scoreToTrustLevel 分数转信任级别
func (m *DeviceTrustManager) scoreToTrustLevel(score float64) TrustLevel {
	switch {
	case score >= 90:
		return TrustVerified
	case score >= 70:
		return TrustHigh
	case score >= 50:
		return TrustMedium
	case score >= 30:
		return TrustLow
	default:
		return TrustNone
	}
}

// getRecentAnomalies 获取最近的异常
func (m *DeviceTrustManager) getRecentAnomalies(deviceID string) []Anomaly {
	anomalies := m.anomalies[deviceID]
	cutoff := time.Now().Add(-m.config.MaxAnomalyAge)

	recent := make([]Anomaly, 0)
	for _, a := range anomalies {
		if a.DetectedAt.After(cutoff) {
			recent = append(recent, a)
		}
	}

	return recent
}

// generateRecommendations 生成建议
func (m *DeviceTrustManager) generateRecommendations(assessment *TrustAssessment) []string {
	var recommendations []string

	if assessment.OverallScore < 50 {
		recommendations = append(recommendations, "设备信任分数过低，建议阻止访问或要求MFA")
	}

	if assessment.TrustLevel < TrustMedium {
		recommendations = append(recommendations, "建议提升设备安全配置")
	}

	for _, anomaly := range assessment.Anomalies {
		if anomaly.Severity == "high" || anomaly.Severity == "critical" {
			recommendations = append(recommendations, fmt.Sprintf("检测到高风险异常: %s", anomaly.Type))
		}
	}

	return recommendations
}

// GetFingerprint 获取设备指纹
func (m *DeviceTrustManager) GetFingerprint(deviceID string) (*DeviceFingerprint, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fp, exists := m.fingerprints[deviceID]
	return fp, exists
}

// GetAssessment 获取信任评估
func (m *DeviceTrustManager) GetAssessment(deviceID string) (*TrustAssessment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assessment, exists := m.assessments[deviceID]
	return assessment, exists
}

// RecordAnomaly 记录异常
func (m *DeviceTrustManager) RecordAnomaly(deviceID string, anomaly Anomaly) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.anomalies[deviceID] = append(m.anomalies[deviceID], anomaly)
}

// 辅助函数
func computeHash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16])
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// IsHardwareStable 检查硬件是否稳定（辅助方法）
func IsHardwareStable(old, new HardwareFingerprint) bool {
	return old.CPUModel == new.CPUModel &&
		old.CPUCores == new.CPUCores &&
		old.DiskSerial == new.DiskSerial &&
		old.MACAddress == new.MACAddress
}

// ContainsAnomalyType 检查是否包含特定类型的异常
func ContainsAnomalyType(anomalies []Anomaly, anomalyType string) bool {
	for _, a := range anomalies {
		if a.Type == anomalyType {
			return true
		}
	}
	return false
}

// FilterAnomaliesBySeverity 按严重程度过滤异常
func FilterAnomaliesBySeverity(anomalies []Anomaly, severity string) []Anomaly {
	var filtered []Anomaly
	for _, a := range anomalies {
		if strings.EqualFold(a.Severity, severity) {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

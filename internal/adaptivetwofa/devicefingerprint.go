package adaptivetwofa

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// DeviceFingerprint 设备指纹信息
type DeviceFingerprint struct {
	// Fingerprint 指纹哈希
	Fingerprint string `json:"fingerprint"`
	// Components 指纹组件
	Components map[string]string `json:"components"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// Confidence 置信度 (0-100)
	Confidence int `json:"confidence"`
}

// FingerprintGenerator 设备指纹生成器
type FingerprintGenerator struct {
	mu         sync.RWMutex
	knownDevices map[string]*DeviceFingerprint // fingerprint -> info
	salt       string
}

// NewFingerprintGenerator 创建设备指纹生成器
func NewFingerprintGenerator(salt string) *FingerprintGenerator {
	if salt == "" {
		salt = "nas-os-default-salt"
	}
	return &FingerprintGenerator{
		knownDevices: make(map[string]*DeviceFingerprint),
		salt:       salt,
	}
}

// FingerprintComponents 指纹组件输入
type FingerprintComponents struct {
	// UserAgent 用户代理字符串
	UserAgent string `json:"user_agent"`
	// IP IP地址
	IP string `json:"ip"`
	// ScreenResolution 屏幕分辨率 (可选)
	ScreenResolution string `json:"screen_resolution,omitempty"`
	// Timezone 时区 (可选)
	Timezone string `json:"timezone,omitempty"`
	// Language 语言 (可选)
	Language string `json:"language,omitempty"`
	// Platform 平台 (可选)
	Platform string `json:"platform,omitempty"`
	// HardwareConcurrency 硬件并发数 (可选)
	HardwareConcurrency string `json:"hardware_concurrency,omitempty"`
	// DeviceMemory 设备内存 (可选)
	DeviceMemory string `json:"device_memory,omitempty"`
	// Fonts 字体列表哈希 (可选)
	Fonts string `json:"fonts,omitempty"`
	// WebGL WebGL渲染器 (可选)
	WebGL string `json:"webgl,omitempty"`
	// Canvas Canvas指纹 (可选)
	Canvas string `json:"canvas,omitempty"`
}

// Generate 生成设备指纹
func (fg *FingerprintGenerator) Generate(components *FingerprintComponents) *DeviceFingerprint {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	// 构建指纹数据
	data := fg.buildFingerprintData(components)

	// 生成哈希
	hash := sha256.Sum256([]byte(data))
	fingerprint := hex.EncodeToString(hash[:])

	// 计算置信度
	confidence := fg.calculateConfidence(components)

	// 检查是否已知设备
	if existing, exists := fg.knownDevices[fingerprint]; exists {
		// 更新最后访问时间
		existing.CreatedAt = time.Now()
		return existing
	}

	// 创建新的设备指纹
	deviceFP := &DeviceFingerprint{
		Fingerprint: fingerprint,
		Components: map[string]string{
			"user_agent":            components.UserAgent,
			"ip":                    components.IP,
			"screen_resolution":     components.ScreenResolution,
			"timezone":              components.Timezone,
			"language":              components.Language,
			"platform":              components.Platform,
			"hardware_concurrency":  components.HardwareConcurrency,
			"device_memory":         components.DeviceMemory,
			"fonts":                 components.Fonts,
			"webgl":                 components.WebGL,
			"canvas":                components.Canvas,
		},
		CreatedAt:  time.Now(),
		Confidence: confidence,
	}

	// 保存到已知设备
	fg.knownDevices[fingerprint] = deviceFP

	return deviceFP
}

// buildFingerprintData 构建指纹数据字符串
func (fg *FingerprintGenerator) buildFingerprintData(components *FingerprintComponents) string {
	var parts []string

	// 添加salt
	parts = append(parts, fg.salt)

	// 添加各组件 (使用管道符分隔)
	if components.UserAgent != "" {
		parts = append(parts, "ua:"+components.UserAgent)
	}
	if components.ScreenResolution != "" {
		parts = append(parts, "sr:"+components.ScreenResolution)
	}
	if components.Timezone != "" {
		parts = append(parts, "tz:"+components.Timezone)
	}
	if components.Language != "" {
		parts = append(parts, "lang:"+components.Language)
	}
	if components.Platform != "" {
		parts = append(parts, "pl:"+components.Platform)
	}
	if components.HardwareConcurrency != "" {
		parts = append(parts, "hc:"+components.HardwareConcurrency)
	}
	if components.DeviceMemory != "" {
		parts = append(parts, "dm:"+components.DeviceMemory)
	}
	if components.Fonts != "" {
		parts = append(parts, "ft:"+components.Fonts)
	}
	if components.WebGL != "" {
		parts = append(parts, "wg:"+components.WebGL)
	}
	if components.Canvas != "" {
		parts = append(parts, "cv:"+components.Canvas)
	}

	return strings.Join(parts, "|")
}

// calculateConfidence 计算指纹置信度
func (fg *FingerprintGenerator) calculateConfidence(components *FingerprintComponents) int {
	score := 0

	// UserAgent 必需，基础分
	if components.UserAgent != "" {
		score += 20
	}

	// 可选组件加分
	if components.ScreenResolution != "" {
		score += 15
	}
	if components.Timezone != "" {
		score += 10
	}
	if components.Language != "" {
		score += 10
	}
	if components.Platform != "" {
		score += 10
	}
	if components.HardwareConcurrency != "" {
		score += 10
	}
	if components.DeviceMemory != "" {
		score += 5
	}
	if components.Fonts != "" {
		score += 10
	}
	if components.WebGL != "" {
		score += 5
	}
	if components.Canvas != "" {
		score += 5
	}

	// 限制在100以内
	if score > 100 {
		score = 100
	}

	return score
}

// IsKnownDevice 检查是否是已知设备
func (fg *FingerprintGenerator) IsKnownDevice(fingerprint string) bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	_, exists := fg.knownDevices[fingerprint]
	return exists
}

// GetDevice 获取设备信息
func (fg *FingerprintGenerator) GetDevice(fingerprint string) *DeviceFingerprint {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	return fg.knownDevices[fingerprint]
}

// GetDeviceCount 获取已知设备数量
func (fg *FingerprintGenerator) GetDeviceCount() int {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	return len(fg.knownDevices)
}

// RemoveDevice 移除已知设备
func (fg *FingerprintGenerator) RemoveDevice(fingerprint string) bool {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	if _, exists := fg.knownDevices[fingerprint]; exists {
		delete(fg.knownDevices, fingerprint)
		return true
	}
	return false
}

// CleanupExpired 清理过期设备
func (fg *FingerprintGenerator) CleanupExpired(maxAge time.Duration) int {
	fg.mu.Lock()
	defer fg.mu.Unlock()

	now := time.Now()
	removed := 0

	for fp, device := range fg.knownDevices {
		if now.Sub(device.CreatedAt) > maxAge {
			delete(fg.knownDevices, fp)
			removed++
		}
	}

	return removed
}

// GenerateSimpleFingerprint 简单指纹生成 (仅使用IP和UserAgent)
func GenerateSimpleFingerprint(ip, userAgent string) string {
	data := fmt.Sprintf("%s|%s", ip, userAgent)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CompareFingerprints 比较两个指纹是否相似
func CompareFingerprints(fp1, fp2 *DeviceFingerprint) float64 {
	if fp1.Fingerprint == fp2.Fingerprint {
		return 1.0
	}

	// 计算组件相似度
	matches := 0
	total := 0

	for key, val1 := range fp1.Components {
		if val1 == "" {
			continue
		}
		total++
		if val2, exists := fp2.Components[key]; exists && val1 == val2 {
			matches++
		}
	}

	if total == 0 {
		return 0.0
	}

	return float64(matches) / float64(total)
}

// GetFingerprintComponents 从请求中提取指纹组件
func GetFingerprintComponents(ip, userAgent string, extra map[string]string) *FingerprintComponents {
	components := &FingerprintComponents{
		IP:        ip,
		UserAgent: userAgent,
	}

	if extra == nil {
		return components
	}

	// 从extra map中提取可选组件
	if v, ok := extra["screen_resolution"]; ok {
		components.ScreenResolution = v
	}
	if v, ok := extra["timezone"]; ok {
		components.Timezone = v
	}
	if v, ok := extra["language"]; ok {
		components.Language = v
	}
	if v, ok := extra["platform"]; ok {
		components.Platform = v
	}
	if v, ok := extra["hardware_concurrency"]; ok {
		components.HardwareConcurrency = v
	}
	if v, ok := extra["device_memory"]; ok {
		components.DeviceMemory = v
	}
	if v, ok := extra["fonts"]; ok {
		components.Fonts = v
	}
	if v, ok := extra["webgl"]; ok {
		components.WebGL = v
	}
	if v, ok := extra["canvas"]; ok {
		components.Canvas = v
	}

	return components
}

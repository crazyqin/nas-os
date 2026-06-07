package adaptivetwofa

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AdaptiveManager 自适应MFA管理器
type AdaptiveManager struct {
	mu             sync.RWMutex
	config         *AdaptiveConfig
	riskEngine     *RiskEngine
	fingerprintGen *FingerprintGenerator
	trustedDevices map[string][]*TrustedDevice // userID -> 信任设备列表
	challenges     map[string]*AuthChallenge   // challengeID -> 挑战
	configPath     string
}

// NewAdaptiveManager 创建自适应MFA管理器
func NewAdaptiveManager(configPath string, config *AdaptiveConfig) (*AdaptiveManager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	mgr := &AdaptiveManager{
		config:         config,
		riskEngine:     NewRiskEngine(config),
		fingerprintGen: NewFingerprintGenerator(""),
		trustedDevices: make(map[string][]*TrustedDevice),
		challenges:     make(map[string]*AuthChallenge),
		configPath:     configPath,
	}

	// 加载配置
	if configPath != "" {
		if err := mgr.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配置失败: %w", err)
		}
	}

	return mgr, nil
}

// loadConfig 加载配置
func (am *AdaptiveManager) loadConfig() error {
	if _, err := os.Stat(am.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(am.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var stored struct {
		TrustedDevices map[string][]*TrustedDevice `json:"trusted_devices"`
		Challenges     map[string]*AuthChallenge   `json:"challenges"`
	}

	if err := json.Unmarshal(data, &stored); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	am.trustedDevices = stored.TrustedDevices
	am.challenges = stored.Challenges

	if am.trustedDevices == nil {
		am.trustedDevices = make(map[string][]*TrustedDevice)
	}
	if am.challenges == nil {
		am.challenges = make(map[string]*AuthChallenge)
	}

	return nil
}

// saveConfig 保存配置
func (am *AdaptiveManager) saveConfig() error {
	if am.configPath == "" {
		return nil
	}

	am.mu.RLock()
	stored := struct {
		TrustedDevices map[string][]*TrustedDevice `json:"trusted_devices"`
		Challenges     map[string]*AuthChallenge   `json:"challenges"`
	}{
		TrustedDevices: am.trustedDevices,
		Challenges:     am.challenges,
	}
	am.mu.RUnlock()

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(am.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(am.configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// EvaluateLogin 评估登录请求
func (am *AdaptiveManager) EvaluateLogin(ctx *LoginContext) *AdaptiveAuthResult {
	am.mu.RLock()
	defer am.mu.RUnlock()

	// 获取设备指纹
	var fingerprint string
	if ctx.DeviceFingerprint != "" {
		// 使用提供的指纹
		fingerprint = ctx.DeviceFingerprint
	} else {
		// 生成设备指纹
		components := &FingerprintComponents{
			IP:        ctx.IP,
			UserAgent: ctx.UserAgent,
		}
		deviceFP := am.fingerprintGen.Generate(components)
		fingerprint = deviceFP.Fingerprint
	}

	// 检查是否是信任设备
	trustedDevice := am.findTrustedDevice(ctx.UserID, fingerprint)

	// 评估风险
	riskScore := am.riskEngine.EvaluateRisk(ctx, trustedDevice)

	// 根据风险等级决定认证要求
	result := &AdaptiveAuthResult{
		RiskScore: riskScore,
	}

	switch riskScore.Level {
	case RiskLow:
		// 低风险，允许登录，提示信任设备
		result.Allowed = true
		if trustedDevice == nil {
			result.TrustDevicePrompt = true
			result.Message = "登录成功。是否信任此设备以跳过未来的双因素认证？"
		} else {
			result.Message = "信任设备，已跳过双因素认证"
		}

	case RiskMedium:
		// 中风险，需要简单验证
		result.Allowed = true
		result.TrustDevicePrompt = true
		challenge := am.createChallenge(ctx.UserID, "totp")
		result.Challenges = []AuthChallenge{*challenge}
		result.Message = "检测到中等风险，请完成验证"

	case RiskHigh:
		// 高风险，需要完整2FA
		result.Allowed = true
		challenge := am.createChallenge(ctx.UserID, "totp")
		result.Challenges = []AuthChallenge{*challenge}
		result.Message = "检测到高风险，请完成双因素认证"

	case RiskCritical:
		// 极高风险，需要额外验证或阻止
		result.Allowed = false
		result.Message = "检测到极高风险登录，请联系管理员或使用已验证的设备登录"
	}

	// 记录登录历史
	go am.riskEngine.RecordLogin(ctx, result.Allowed, riskScore.Score)

	return result
}

// findTrustedDevice 查找信任设备
func (am *AdaptiveManager) findTrustedDevice(userID, fingerprint string) *TrustedDevice {
	devices, exists := am.trustedDevices[userID]
	if !exists {
		return nil
	}

	now := time.Now()
	for _, device := range devices {
		if device.Fingerprint == fingerprint && now.Before(device.ExpiresAt) {
			// 更新最后访问时间
			device.LastSeenAt = now
			return device
		}
	}

	return nil
}

// createChallenge 创建认证挑战
func (am *AdaptiveManager) createChallenge(userID, challengeType string) *AuthChallenge {
	challengeID, _ := generateID()

	challenge := &AuthChallenge{
		ChallengeID: challengeID,
		UserID:      userID,
		Type:        challengeType,
		Required:    true,
		ExpiresAt:   time.Now().Add(am.config.ChallengeTTL),
		CreatedAt:   time.Now(),
	}

	am.challenges[challengeID] = challenge
	return challenge
}

// TrustDevice 信任设备
func (am *AdaptiveManager) TrustDevice(userID, deviceFingerprint, ip, userAgent string, geoLocation *GeoLocation) (*TrustedDevice, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 检查用户信任设备数量
	devices := am.trustedDevices[userID]
	if len(devices) >= am.config.MaxTrustedDevices {
		// 移除最旧的设备
		oldestIdx := 0
		oldestTime := devices[0].CreatedAt
		for i, d := range devices {
			if d.CreatedAt.Before(oldestTime) {
				oldestTime = d.CreatedAt
				oldestIdx = i
			}
		}
		devices = append(devices[:oldestIdx], devices[oldestIdx+1:]...)
	}

	// 生成设备ID
	deviceID, _ := generateID()

	// 创建信任设备
	device := &TrustedDevice{
		DeviceID:    deviceID,
		UserID:      userID,
		Fingerprint: deviceFingerprint,
		IP:          ip,
		UserAgent:   userAgent,
		GeoLocation: geoLocation,
		CreatedAt:   time.Now(),
		LastSeenAt:  time.Now(),
		ExpiresAt:   time.Now().Add(am.config.TrustedDeviceTTL),
		TrustLevel:  5,
	}

	am.trustedDevices[userID] = append(devices, device)

	// 保存配置
	if err := am.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return device, nil
}

// RevokeTrust 撤销设备信任
func (am *AdaptiveManager) RevokeTrust(userID, deviceID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	devices, exists := am.trustedDevices[userID]
	if !exists {
		return fmt.Errorf("用户 %s 没有信任设备", userID)
	}

	for i, device := range devices {
		if device.DeviceID == deviceID {
			am.trustedDevices[userID] = append(devices[:i], devices[i+1:]...)
			return am.saveConfig()
		}
	}

	return fmt.Errorf("未找到设备 %s", deviceID)
}

// RevokeAllTrust 撤销用户所有信任设备
func (am *AdaptiveManager) RevokeAllTrust(userID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.trustedDevices, userID)
	return am.saveConfig()
}

// GetTrustedDevices 获取用户信任设备列表
func (am *AdaptiveManager) GetTrustedDevices(userID string) []*TrustedDevice {
	am.mu.RLock()
	defer am.mu.RUnlock()

	devices, exists := am.trustedDevices[userID]
	if !exists {
		return nil
	}

	// 过滤过期设备
	now := time.Now()
	validDevices := make([]*TrustedDevice, 0, len(devices))
	for _, device := range devices {
		if now.Before(device.ExpiresAt) {
			validDevices = append(validDevices, device)
		}
	}

	return validDevices
}

// VerifyChallenge 验证挑战
func (am *AdaptiveManager) VerifyChallenge(challengeID string) (*AuthChallenge, error) {
	am.mu.Lock()
	defer am.mu.Unlock()

	challenge, exists := am.challenges[challengeID]
	if !exists {
		return nil, fmt.Errorf("挑战 %s 不存在", challengeID)
	}

	if challenge.IsExpired() {
		delete(am.challenges, challengeID)
		return nil, fmt.Errorf("挑战 %s 已过期", challengeID)
	}

	return challenge, nil
}

// CompleteChallenge 完成挑战
func (am *AdaptiveManager) CompleteChallenge(challengeID string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	challenge, exists := am.challenges[challengeID]
	if !exists {
		return fmt.Errorf("挑战 %s 不存在", challengeID)
	}

	if challenge.IsExpired() {
		delete(am.challenges, challengeID)
		return fmt.Errorf("挑战 %s 已过期", challengeID)
	}

	// 删除已完成的挑战
	delete(am.challenges, challengeID)

	return am.saveConfig()
}

// CleanupExpired 清理过期数据
func (am *AdaptiveManager) CleanupExpired() (devicesRemoved, challengesRemoved int) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()

	// 清理过期信任设备
	for userID, devices := range am.trustedDevices {
		validDevices := make([]*TrustedDevice, 0, len(devices))
		for _, device := range devices {
			if now.Before(device.ExpiresAt) {
				validDevices = append(validDevices, device)
			} else {
				devicesRemoved++
			}
		}
		am.trustedDevices[userID] = validDevices
	}

	// 清理过期挑战
	for challengeID, challenge := range am.challenges {
		if challenge.IsExpired() {
			delete(am.challenges, challengeID)
			challengesRemoved++
		}
	}

	// 清理过期指纹 (30天)
	am.fingerprintGen.CleanupExpired(30 * 24 * time.Hour)

	return devicesRemoved, challengesRemoved
}

// GetStats 获取统计信息
func (am *AdaptiveManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	totalTrustedDevices := 0
	for _, devices := range am.trustedDevices {
		totalTrustedDevices += len(devices)
	}

	return map[string]interface{}{
		"total_users_with_trust": len(am.trustedDevices),
		"total_trusted_devices":  totalTrustedDevices,
		"active_challenges":      len(am.challenges),
		"known_fingerprints":     am.fingerprintGen.GetDeviceCount(),
	}
}

// GetConfig 获取配置
func (am *AdaptiveManager) GetConfig() *AdaptiveConfig {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.config
}

// UpdateConfig 更新配置
func (am *AdaptiveManager) UpdateConfig(config *AdaptiveConfig) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.config = config
}

// GetRiskEngine 获取风险引擎
func (am *AdaptiveManager) GetRiskEngine() *RiskEngine {
	return am.riskEngine
}

// GetFingerprintGenerator 获取指纹生成器
func (am *AdaptiveManager) GetFingerprintGenerator() *FingerprintGenerator {
	return am.fingerprintGen
}

// generateID 生成随机ID
func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

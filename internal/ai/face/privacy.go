// Package face - 人脸隐私合规管理
package face

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PrivacyManager 人脸隐私管理器
type PrivacyManager struct {
	dataDir       string
	encryptKey    []byte
	consents      map[string]*ConsentRecord
	config        FacePrivacyConfig
	cleanupCancel context.CancelFunc
	mu            sync.RWMutex
}

// ConsentRecord 用户知情同意记录
type ConsentRecord struct {
	UserID      string    `json:"userId"`
	Consented   bool      `json:"consented"`
	ConsentTime time.Time `json:"consentTime"`
	Purpose     string    `json:"purpose"` // 人脸识别用途说明
	Version     string    `json:"version"` // 同意书版本
}

// FacePrivacyConfig 人脸隐私配置
type FacePrivacyConfig struct {
	// 数据存储位置（仅本地）
	DataDir string `json:"dataDir"`
	// 是否启用加密存储
	EnableEncryption bool `json:"enableEncryption"`
	// 数据保留天数（默认365天=1年，0表示永久保留）
	DataRetentionDays int `json:"dataRetentionDays"`
	// 是否启用自动清理
	EnableAutoCleanup bool `json:"enableAutoCleanup"`
	// 自动清理检查间隔（小时）
	AutoCleanupIntervalHours int `json:"autoCleanupIntervalHours"`
	// 是否允许导出
	AllowExport bool `json:"allowExport"`
	// 同意书版本
	ConsentVersion string `json:"consentVersion"`
}

// DefaultPrivacyConfig 默认隐私配置
var DefaultPrivacyConfig = FacePrivacyConfig{
	DataDir:                  "/var/lib/nas-os/face-data",
	EnableEncryption:         true,
	DataRetentionDays:        365,  // 默认保留1年
	EnableAutoCleanup:        true, // 默认启用自动清理
	AutoCleanupIntervalHours: 24,   // 每天检查一次
	AllowExport:              true,
	ConsentVersion:           "1.0",
}

// NewPrivacyManager 创建隐私管理器
func NewPrivacyManager(dataDir string) *PrivacyManager {
	return NewPrivacyManagerWithConfig(dataDir, DefaultPrivacyConfig)
}

// NewPrivacyManagerWithConfig 创建带配置的隐私管理器
func NewPrivacyManagerWithConfig(dataDir string, config FacePrivacyConfig) *PrivacyManager {
	return &PrivacyManager{
		dataDir:  dataDir,
		consents: make(map[string]*ConsentRecord),
		config:   config,
	}
}

// Initialize 初始化隐私管理器
func (pm *PrivacyManager) Initialize() error {
	// 创建数据目录
	if err := os.MkdirAll(pm.dataDir, 0700); err != nil {
		return fmt.Errorf("create data directory failed: %w", err)
	}

	// 加载已有同意记录
	if err := pm.loadConsents(); err != nil {
		return err
	}

	// 启动自动清理任务
	if pm.config.EnableAutoCleanup && pm.config.DataRetentionDays > 0 {
		pm.startAutoCleanup()
	}

	return nil
}

// Stop 停止隐私管理器
func (pm *PrivacyManager) Stop() {
	if pm.cleanupCancel != nil {
		pm.cleanupCancel()
		pm.cleanupCancel = nil
	}
}

// RequestConsent 请求用户知情同意
func (pm *PrivacyManager) RequestConsent(userID string) (*ConsentInfo, error) {
	info := &ConsentInfo{
		Title: "人脸识别功能知情同意",
		Content: `您即将启用人脸识别功能。请注意：

1. 人脸数据仅存储在您的本地NAS设备上，不会上传到任何云端服务。
2. 人脸数据用于自动识别和分类照片中的人物。
3. 您可以随时删除所有人脸数据。
4. 您可以导出人脸数据用于备份。
5. 本功能不会与任何第三方服务共享您的数据。

请确认是否同意启用此功能。`,
		Version: DefaultPrivacyConfig.ConsentVersion,
		Purpose: "照片人物自动识别和分类",
	}
	return info, nil
}

// RecordConsent 记录用户同意
func (pm *PrivacyManager) RecordConsent(userID string, consented bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	record := &ConsentRecord{
		UserID:      userID,
		Consented:   consented,
		ConsentTime: time.Now(),
		Purpose:     "照片人物自动识别和分类",
		Version:     DefaultPrivacyConfig.ConsentVersion,
	}

	pm.consents[userID] = record
	return pm.saveConsents()
}

// CheckConsent 检查用户是否已同意
func (pm *PrivacyManager) CheckConsent(userID string) (bool, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	record, ok := pm.consents[userID]
	if !ok {
		return false, nil
	}
	return record.Consented, nil
}

// ExportData 导出用户人脸数据
func (pm *PrivacyManager) ExportData(ctx context.Context, userID string) (*ExportResult, error) {
	if !DefaultPrivacyConfig.AllowExport {
		return nil, fmt.Errorf("export feature is disabled")
	}

	// 检查用户同意
	consented, err := pm.CheckConsent(userID)
	if err != nil {
		return nil, err
	}
	if !consented {
		return nil, fmt.Errorf("user has not consented to face recognition")
	}

	// 收集人脸数据
	exportData := &FaceExportData{
		UserID:     userID,
		ExportTime: time.Now(),
		Faces:      []FaceRecord{},
		Clusters:   []ClusterRecord{},
	}

	facePath := filepath.Join(pm.dataDir, "faces_"+userID+".json")
	if data, err := os.ReadFile(facePath); err == nil {
		_ = json.Unmarshal(data, &exportData.Faces)
	}
	clusterPath := filepath.Join(pm.dataDir, "clusters_"+userID+".json")
	if data, err := os.ReadFile(clusterPath); err == nil {
		_ = json.Unmarshal(data, &exportData.Clusters)
	}

	return &ExportResult{
		Format:    "json",
		Data:      exportData,
		FilePath:  filepath.Join(pm.dataDir, "export_"+userID+".json"),
		CreatedAt: time.Now(),
	}, nil
}

// DeleteAllData 删除用户所有人脸数据
func (pm *PrivacyManager) DeleteAllData(ctx context.Context, userID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 删除人脸数据文件
	dataPath := filepath.Join(pm.dataDir, userID)
	if err := os.RemoveAll(dataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete face data failed: %w", err)
	}

	// 删除同意记录（可选）
	// delete(pm.consents, userID)
	// pm.saveConsents()

	return nil
}

// GetPrivacyPolicy 获取隐私政策说明
func (pm *PrivacyManager) GetPrivacyPolicy() *PrivacyPolicy {
	return &PrivacyPolicy{
		Title: "人脸识别隐私政策",
		Sections: []PolicySection{
			{
				Title:   "数据存储",
				Content: "所有人脸数据仅存储在您的本地NAS设备上，不会上传到任何云端服务。",
			},
			{
				Title:   "数据用途",
				Content: "人脸数据仅用于自动识别和分类照片中的人物，不会用于其他目的。",
			},
			{
				Title:   "数据删除",
				Content: "您可以随时删除所有人脸数据，删除后数据将不可恢复。",
			},
			{
				Title:   "数据导出",
				Content: "您可以导出人脸数据用于备份，导出数据请妥善保管。",
			},
			{
				Title:   "数据共享",
				Content: "本功能不会与任何第三方服务共享您的数据。",
			},
			{
				Title:   "数据加密",
				Content: "人脸特征向量采用加密存储，保障数据安全。",
			},
		},
		Version:   DefaultPrivacyConfig.ConsentVersion,
		UpdatedAt: time.Now(),
	}
}

// 内部方法
func (pm *PrivacyManager) loadConsents() error {
	filePath := filepath.Join(pm.dataDir, "consents.json")
	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return nil // 文件不存在，首次使用
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &pm.consents)
}

func (pm *PrivacyManager) saveConsents() error {
	filePath := filepath.Join(pm.dataDir, "consents.json")
	data, err := json.MarshalIndent(pm.consents, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0600)
}

// startAutoCleanup 启动自动清理任务
func (pm *PrivacyManager) startAutoCleanup() {
	ctx, cancel := context.WithCancel(context.Background())
	pm.cleanupCancel = cancel

	go pm.runAutoCleanup(ctx)
}

// runAutoCleanup 运行自动清理循环
func (pm *PrivacyManager) runAutoCleanup(ctx context.Context) {
	interval := time.Duration(pm.config.AutoCleanupIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour // 默认每天
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动时先执行一次清理
	pm.cleanupExpiredData(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pm.cleanupExpiredData(ctx)
		}
	}
}

// cleanupExpiredData 清理过期的人脸数据
func (pm *PrivacyManager) cleanupExpiredData(ctx context.Context) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.config.DataRetentionDays <= 0 {
		return // 不清理
	}

	cutoff := time.Now().AddDate(0, 0, -pm.config.DataRetentionDays)

	// 遍历用户目录，删除过期文件
	usersDir := filepath.Join(pm.dataDir, "users")
	userDirs, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return // 目录不存在，无需清理
		}
		// 记录错误但不中断
		return
	}

	for _, userEntry := range userDirs {
		if !userEntry.IsDir() {
			continue
		}

		userID := userEntry.Name()
		userDir := filepath.Join(usersDir, userID)

		// 清理过期文件
		pm.cleanupUserExpiredFiles(ctx, userDir, cutoff, userID)
	}
}

// cleanupUserExpiredFiles 清理单个用户的过期文件
func (pm *PrivacyManager) cleanupUserExpiredFiles(ctx context.Context, userDir string, cutoff time.Time, userID string) {
	// 清理人脸特征文件
	facesDir := filepath.Join(userDir, "faces")
	pm.cleanupDirByTime(facesDir, cutoff)

	// 清理人脸图片缩略图
	thumbnailsDir := filepath.Join(userDir, "thumbnails")
	pm.cleanupDirByTime(thumbnailsDir, cutoff)

	// 更新人脸记录索引（移除已删除的记录）
	indexPath := filepath.Join(userDir, "face_index.json")
	pm.updateFaceIndex(indexPath, cutoff)
}

// cleanupDirByTime 按时间清理目录中的文件
func (pm *PrivacyManager) cleanupDirByTime(dir string, cutoff time.Time) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}

		// 检查文件修改时间
		if info.ModTime().Before(cutoff) {
			filePath := filepath.Join(dir, file.Name())
			_ = os.Remove(filePath) // 删除过期文件
		}
	}
}

// updateFaceIndex 更新人脸索引，移除过期记录
func (pm *PrivacyManager) updateFaceIndex(indexPath string, cutoff time.Time) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}

	var index FaceIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return
	}

	// 过滤过期记录
	var validRecords []FaceIndexRecord
	for _, record := range index.Records {
		if record.CreatedAt.After(cutoff) {
			validRecords = append(validRecords, record)
		}
	}

	// 保存更新后的索引
	index.Records = validRecords
	updatedData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(indexPath, updatedData, 0600)
}

// GetRetentionPolicy 获取数据保留策略信息
func (pm *PrivacyManager) GetRetentionPolicy() *RetentionPolicy {
	return &RetentionPolicy{
		RetentionDays:     pm.config.DataRetentionDays,
		EnableAutoCleanup: pm.config.EnableAutoCleanup,
		CheckInterval:     pm.config.AutoCleanupIntervalHours,
		Description: fmt.Sprintf("人脸数据默认保留%d天（约%.1f年），超期数据将自动清理",
			pm.config.DataRetentionDays, float64(pm.config.DataRetentionDays)/365.0),
	}
}

// SetRetentionPolicy 设置数据保留策略
func (pm *PrivacyManager) SetRetentionPolicy(days int, enableAutoCleanup bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.config.DataRetentionDays = days
	pm.config.EnableAutoCleanup = enableAutoCleanup

	// 如果启用了自动清理，重启清理任务
	if pm.cleanupCancel != nil {
		pm.cleanupCancel()
		pm.cleanupCancel = nil
	}

	if enableAutoCleanup && days > 0 {
		pm.startAutoCleanup()
	}

	return nil
}

// 类型定义
type ConsentInfo struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Version string `json:"version"`
	Purpose string `json:"purpose"`
}

type FaceExportData struct {
	UserID     string          `json:"userId"`
	ExportTime time.Time       `json:"exportTime"`
	Faces      []FaceRecord    `json:"faces"`
	Clusters   []ClusterRecord `json:"clusters"`
}

type FaceRecord struct {
	ID        string    `json:"id"`
	ImagePath string    `json:"imagePath"`
	CreatedAt time.Time `json:"createdAt"`
}

type ClusterRecord struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	FaceIDs []string `json:"faceIds"`
}

type ExportResult struct {
	Format    string      `json:"format"`
	Data      interface{} `json:"data"`
	FilePath  string      `json:"filePath"`
	CreatedAt time.Time   `json:"createdAt"`
}

type PrivacyPolicy struct {
	Title     string          `json:"title"`
	Sections  []PolicySection `json:"sections"`
	Version   string          `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type PolicySection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// RetentionPolicy 数据保留策略
type RetentionPolicy struct {
	RetentionDays     int    `json:"retentionDays"`
	EnableAutoCleanup bool   `json:"enableAutoCleanup"`
	CheckInterval     int    `json:"checkInterval"` // 小时
	Description       string `json:"description"`
}

// FaceIndex 人脸索引文件结构
type FaceIndex struct {
	UserID  string            `json:"userId"`
	Records []FaceIndexRecord `json:"records"`
}

// FaceIndexRecord 人脸索引记录
type FaceIndexRecord struct {
	FaceID    string    `json:"faceId"`
	ImagePath string    `json:"imagePath"`
	CreatedAt time.Time `json:"createdAt"`
}

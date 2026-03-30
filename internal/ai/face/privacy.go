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
	dataDir    string
	encryptKey []byte
	consents   map[string]*ConsentRecord
	mu         sync.RWMutex
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
	// 数据保留天数（0表示永久保留）
	DataRetentionDays int `json:"dataRetentionDays"`
	// 是否允许导出
	AllowExport bool `json:"allowExport"`
	// 同意书版本
	ConsentVersion string `json:"consentVersion"`
}

// DefaultPrivacyConfig 默认隐私配置
var DefaultPrivacyConfig = FacePrivacyConfig{
	DataDir:           "/var/lib/nas-os/face-data",
	EnableEncryption:  true,
	DataRetentionDays: 0, // 永久保留（用户主动删除）
	AllowExport:       true,
	ConsentVersion:    "1.0",
}

// NewPrivacyManager 创建隐私管理器
func NewPrivacyManager(dataDir string) *PrivacyManager {
	return &PrivacyManager{
		dataDir:  dataDir,
		consents: make(map[string]*ConsentRecord),
	}
}

// Initialize 初始化隐私管理器
func (pm *PrivacyManager) Initialize() error {
	// 创建数据目录
	if err := os.MkdirAll(pm.dataDir, 0700); err != nil {
		return fmt.Errorf("create data directory failed: %w", err)
	}

	// 加载已有同意记录
	return pm.loadConsents()
}

// RequestConsent 请求用户知情同意
func (pm *PrivacyManager) RequestConsent(userID string) (*ConsentInfo, error) {
	info := &ConsentInfo{
		Title:    "人脸识别功能知情同意",
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
		UserID:    userID,
		ExportTime: time.Now(),
		Faces:     []FaceRecord{},
		Clusters:  []ClusterRecord{},
	}

	// TODO: 实际从存储中读取人脸数据

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
				Title: "数据存储",
				Content: "所有人脸数据仅存储在您的本地NAS设备上，不会上传到任何云端服务。",
			},
			{
				Title: "数据用途",
				Content: "人脸数据仅用于自动识别和分类照片中的人物，不会用于其他目的。",
			},
			{
				Title: "数据删除",
				Content: "您可以随时删除所有人脸数据，删除后数据将不可恢复。",
			},
			{
				Title: "数据导出",
				Content: "您可以导出人脸数据用于备份，导出数据请妥善保管。",
			},
			{
				Title: "数据共享",
				Content: "本功能不会与任何第三方服务共享您的数据。",
			},
			{
				Title: "数据加密",
				Content: "人脸特征向量采用加密存储，保障数据安全。",
			},
		},
		Version: DefaultPrivacyConfig.ConsentVersion,
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

// 类型定义
type ConsentInfo struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	Version  string `json:"version"`
	Purpose  string `json:"purpose"`
}

type FaceExportData struct {
	UserID     string         `json:"userId"`
	ExportTime time.Time      `json:"exportTime"`
	Faces      []FaceRecord   `json:"faces"`
	Clusters   []ClusterRecord `json:"clusters"`
}

type FaceRecord struct {
	ID        string    `json:"id"`
	ImagePath string    `json:"imagePath"`
	CreatedAt time.Time `json:"createdAt"`
}

type ClusterRecord struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	FaceIDs   []string `json:"faceIds"`
}

type ExportResult struct {
	Format    string      `json:"format"`
	Data      interface{} `json:"data"`
	FilePath  string      `json:"filePath"`
	CreatedAt time.Time   `json:"createdAt"`
}

type PrivacyPolicy struct {
	Title     string         `json:"title"`
	Sections  []PolicySection `json:"sections"`
	Version   string         `json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

type PolicySection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}
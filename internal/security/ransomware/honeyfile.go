// Package ransomware 提供蜜罐文件检测功能
// 参考TrueNAS 26的勒索检测设计，通过部署诱饵文件检测勒索软件攻击
package ransomware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HoneyFileConfig 蜜罐文件配置
type HoneyFileConfig struct {
	// 是否启用蜜罐检测
	Enabled bool `json:"enabled"`

	// 蜜罐文件部署路径列表
	DeployPaths []string `json:"deployPaths"`

	// 每个路径部署的蜜罐文件数量
	FilesPerPath int `json:"filesPerPath"`

	// 蜜罐文件类型（扩展名）
	FileTypes []string `json:"fileTypes"`

	// 文件名模式（用于生成随机文件名）
	NamePatterns []string `json:"namePatterns"`

	// 文件大小范围（字节）
	MinFileSize int64 `json:"minFileSize"`
	MaxFileSize int64 `json:"maxFileSize"`

	// 检测间隔
	CheckInterval time.Duration `json:"checkInterval"`

	// 文件内容生成模式
	ContentPattern ContentPattern `json:"contentPattern"`

	// 告警配置
	AlertOnAccess bool `json:"alertOnAccess"` // 文件被访问时告警
	AlertOnModify bool `json:"alertOnModify"` // 文件被修改时告警
	AlertOnDelete bool `json:"alertOnDelete"` // 文件被删除时告警
	AlertOnRename bool `json:"alertOnRename"` // 文件被重命名时告警
}

// ContentPattern 蜜罐文件内容模式
type ContentPattern struct {
	// 内容类型：random（随机数据）、structured（结构化）、realistic（仿真）
	Type string `json:"type"`

	// 结构化内容的模板（如JSON、XML等）
	Template string `json:"template"`

	// 是否嵌入追踪标记
	EmbedTrackingMarker bool `json:"embedTrackingMarker"`

	// 追踪标记前缀
	TrackingMarkerPrefix string `json:"trackingMarkerPrefix"`
}

// HoneyFile 蜜罐文件记录
type HoneyFile struct {
	// 文件ID
	ID string `json:"id"`

	// 文件路径
	Path string `json:"path"`

	// 文件名
	Name string `json:"name"`

	// 文件大小
	Size int64 `json:"size"`

	// 文件扩展名
	Extension string `json:"extension"`

	// 文件哈希（SHA256）
	Hash string `json:"hash"`

	// 追踪标记
	TrackingMarker string `json:"trackingMarker"`

	// 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// 最后检查时间
	LastChecked time.Time `json:"lastChecked"`

	// 文件状态
	Status HoneyFileStatus `json:"status"`

	// 部署路径ID
	DeployPathID string `json:"deployPathId"`

	// 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// HoneyFileStatus 蜜罐文件状态
type HoneyFileStatus string

const (
	// HoneyFileStatusActive 正常状态
	HoneyFileStatusActive HoneyFileStatus = "active"

	// HoneyFileStatusAccessed 被访问
	HoneyFileStatusAccessed HoneyFileStatus = "accessed"

	// HoneyFileStatusModified 被修改
	HoneyFileStatusModified HoneyFileStatus = "modified"

	// HoneyFileStatusDeleted 被删除
	HoneyFileStatusDeleted HoneyFileStatus = "deleted"

	// HoneyFileStatusRenamed 被重命名
	HoneyFileStatusRenamed HoneyFileStatus = "renamed"

	// HoneyFileStatusEncrypted 被加密
	HoneyFileStatusEncrypted HoneyFileStatus = "encrypted"
)

// HoneyFileEvent 蜜罐文件事件
type HoneyFileEvent struct {
	// 事件ID
	ID string `json:"id"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 文件ID
	FileID string `json:"fileId"`

	// 文件路径
	FilePath string `json:"filePath"`

	// 事件类型
	EventType HoneyFileEventType `json:"eventType"`

	// 原始状态
	OldStatus HoneyFileStatus `json:"oldStatus"`

	// 新状态
	NewStatus HoneyFileStatus `json:"newStatus"`

	// 触发进程信息
	ProcessInfo *ProcessInfo `json:"processInfo,omitempty"`

	// 用户信息
	UserID string `json:"userId,omitempty"`

	// 详细信息
	Details map[string]interface{} `json:"details,omitempty"`

	// 威胁等级（基于事件判断）
	ThreatLevel ThreatLevel `json:"threatLevel"`
}

// HoneyFileEventType 蜜罐文件事件类型
type HoneyFileEventType string

const (
	// HoneyFileEventAccess 访问事件
	HoneyFileEventAccess HoneyFileEventType = "access"

	// HoneyFileEventModify 修改事件
	HoneyFileEventModify HoneyFileEventType = "modify"

	// HoneyFileEventDelete 删除事件
	HoneyFileEventDelete HoneyFileEventType = "delete"

	// HoneyFileEventRename 重命名事件
	HoneyFileEventRename HoneyFileEventType = "rename"

	// HoneyFileEventEncrypt 加密事件
	HoneyFileEventEncrypt HoneyFileEventType = "encrypt"

	// HoneyFileEventCreate 创建事件
	HoneyFileEventCreate HoneyFileEventType = "create"

	// HoneyFileEventCheck 检查事件
	HoneyFileEventCheck HoneyFileEventType = "check"
)

// HoneyFileAlert 蜜罐文件告警
type HoneyFileAlert struct {
	// 告警ID
	ID string `json:"id"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`

	// 威胁等级
	Severity ThreatLevel `json:"severity"`

	// 告警类型
	Type string `json:"type"`

	// 标题
	Title string `json:"title"`

	// 消息
	Message string `json:"message"`

	// 触发的事件列表
	Events []HoneyFileEvent `json:"events"`

	// 受影响的蜜罐文件数量
	AffectedFiles int `json:"affectedFiles"`

	// 可能的攻击来源
	AttackSource *AttackSource `json:"attackSource,omitempty"`

	// 推荐行动
	Recommendations []string `json:"recommendations"`

	// 已采取的行动
	ActionsTaken []string `json:"actionsTaken"`

	// 告警状态
	Status AlertStatus `json:"status"`
}

// AttackSource 攻击来源信息
type AttackSource struct {
	// 进程信息
	Process *ProcessInfo `json:"process,omitempty"`

	// 用户信息
	User string `json:"user,omitempty"`

	// IP地址（远程访问时）
	IPAddress string `json:"ipAddress,omitempty"`

	// 连接信息
	ConnectionType string `json:"connectionType,omitempty"` // smb, nfs, ftp, webdav等
}

// HoneyFileManager 蜜罐文件管理器
type HoneyFileManager struct {
	config HoneyFileConfig

	// 蜜罐文件记录
	files map[string]*HoneyFile

	// 按路径索引
	pathIndex map[string][]string // path -> file IDs

	// 事件记录
	events []HoneyFileEvent

	// 告警通道
	alerts chan HoneyFileAlert

	// 统计信息
	stats HoneyFileStats

	// 运行状态
	running bool

	mu sync.RWMutex
}

// HoneyFileStats 蜜罐文件统计
type HoneyFileStats struct {
	// 总蜜罐文件数
	TotalFiles int `json:"totalFiles"`

	// 活跃文件数
	ActiveFiles int `json:"activeFiles"`

	// 被触发文件数
	TriggeredFiles int `json:"triggeredFiles"`

	// 总事件数
	TotalEvents int64 `json:"totalEvents"`

	// 总告警数
	TotalAlerts int64 `json:"totalAlerts"`

	// 最后部署时间
	LastDeployed time.Time `json:"lastDeployed,omitempty"`

	// 最后检查时间
	LastChecked time.Time `json:"lastChecked,omitempty"`

	// 最后触发时间
	LastTriggered time.Time `json:"lastTriggered,omitempty"`
}

// DefaultHoneyFileConfig 返回默认蜜罐配置
func DefaultHoneyFileConfig() HoneyFileConfig {
	return HoneyFileConfig{
		Enabled:       true,
		DeployPaths:   []string{"/data/shares", "/home", "/mnt"},
		FilesPerPath:  5,
		FileTypes:     []string{".doc", ".docx", ".xls", ".xlsx", ".pdf", ".jpg", ".zip"},
		NamePatterns:  []string{"financial_report", "project_plan", "backup_data", "confidential", "important"},
		MinFileSize:   1024,    // 1KB
		MaxFileSize:   102400,  // 100KB
		CheckInterval: 30 * time.Second,
		ContentPattern: ContentPattern{
			Type:                 "realistic",
			EmbedTrackingMarker:  true,
			TrackingMarkerPrefix: "HONEYTRACK_",
		},
		AlertOnAccess: false,
		AlertOnModify: true,
		AlertOnDelete: true,
		AlertOnRename: true,
	}
}

// NewHoneyFileManager 创建蜜罐文件管理器
func NewHoneyFileManager(config HoneyFileConfig) (*HoneyFileManager, error) {
	if config.FilesPerPath <= 0 {
		config.FilesPerPath = 5
	}
	if len(config.FileTypes) == 0 {
		config.FileTypes = DefaultHoneyFileConfig().FileTypes
	}
	if config.MinFileSize <= 0 {
		config.MinFileSize = 1024
	}
	if config.MaxFileSize <= config.MinFileSize {
		config.MaxFileSize = config.MinFileSize * 100
	}

	m := &HoneyFileManager{
		config:     config,
		files:      make(map[string]*HoneyFile),
		pathIndex:  make(map[string][]string),
		events:     make([]HoneyFileEvent, 0),
		alerts:     make(chan HoneyFileAlert, 100),
	}

	// 初始部署蜜罐文件
	if config.Enabled {
		if err := m.DeployAll(); err != nil {
			return nil, fmt.Errorf("failed to deploy honey files: %w", err)
		}
	}

	return m, nil
}

// DeployAll 在所有配置路径部署蜜罐文件
func (m *HoneyFileManager) DeployAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, path := range m.config.DeployPaths {
		if err := m.deployInPath(path); err != nil {
			return fmt.Errorf("failed to deploy in %s: %w", path, err)
		}
	}

	m.stats.LastDeployed = time.Now()
	return nil
}

// deployInPath 在指定路径部署蜜罐文件
func (m *HoneyFileManager) deployInPath(basePath string) error {
	// 确保路径存在
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return err
	}

	for i := 0; i < m.config.FilesPerPath; i++ {
		file, err := m.createHoneyFile(basePath)
		if err != nil {
			continue // 单个文件失败不中断
		}

		m.files[file.ID] = file
		m.pathIndex[basePath] = append(m.pathIndex[basePath], file.ID)
		m.stats.TotalFiles++
		m.stats.ActiveFiles++
	}

	return nil
}

// createHoneyFile 创建单个蜜罐文件
func (m *HoneyFileManager) createHoneyFile(basePath string) (*HoneyFile, error) {
	// 随机选择扩展名
	extIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(m.config.FileTypes))))
	ext := m.config.FileTypes[extIndex.Int64()]

	// 生成文件名
	nameIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(m.config.NamePatterns))))
	namePrefix := m.config.NamePatterns[nameIndex.Int64()]
	randomSuffix := generateRandomString(4)
	filename := fmt.Sprintf("%s_%s%s", namePrefix, randomSuffix, ext)

	// 生成文件路径（可能在子目录中）
	subdirs := []string{"", "documents", "backup", "archive", "important"}
	subdirIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(subdirs))))
	subdir := subdirs[subdirIndex.Int64()]

	fullPath := filepath.Join(basePath, subdir, filename)
	if subdir != "" {
		if err := os.MkdirAll(filepath.Join(basePath, subdir), 0755); err != nil {
			return nil, err
		}
	}

	// 生成文件大小
	sizeRange := m.config.MaxFileSize - m.config.MinFileSize
	sizeOffset, _ := rand.Int(rand.Reader, big.NewInt(sizeRange))
	size := m.config.MinFileSize + sizeOffset.Int64()

	// 生成文件内容
	content := m.generateContent(size, ext)

	// 计算哈希
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	// 生成追踪标记
	trackingMarker := m.config.ContentPattern.TrackingMarkerPrefix + generateRandomString(16)

	// 写入文件
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return nil, err
	}

	file := &HoneyFile{
		ID:            generateHoneyFileID(),
		Path:          fullPath,
		Name:          filename,
		Size:          size,
		Extension:     ext,
		Hash:          hashStr,
		TrackingMarker: trackingMarker,
		CreatedAt:     time.Now(),
		LastChecked:   time.Now(),
		Status:        HoneyFileStatusActive,
		DeployPathID:  basePath,
		Metadata: map[string]interface{}{
			"created_by": "honeyfile_manager",
			"purpose":    "ransomware_detection",
		},
	}

	// 记录创建事件
	m.recordEvent(HoneyFileEvent{
		ID:         generateEventID(),
		Timestamp:  time.Now(),
		FileID:     file.ID,
		FilePath:   file.Path,
		EventType:  HoneyFileEventCreate,
		OldStatus:  "",
		NewStatus:  HoneyFileStatusActive,
		ThreatLevel: ThreatLevelNone,
	})

	return file, nil
}

// generateContent 生成蜜罐文件内容
func (m *HoneyFileManager) generateContent(size int64, ext string) []byte {
	content := make([]byte, size)

	switch m.config.ContentPattern.Type {
	case "random":
		// 纯随机数据
		rand.Read(content)

	case "structured":
		// 结构化数据（根据扩展名）
		switch ext {
		case ".json":
			m.generateJSONContent(content)
		case ".xml":
			m.generateXMLContent(content)
		case ".txt":
			m.generateTextContent(content)
		default:
			m.generateBinaryContent(content)
		}

	case "realistic":
		// 仿真内容（看起来像真实文件）
		m.generateRealisticContent(content, ext)

	default:
		rand.Read(content)
	}

	// 嵌入追踪标记
	if m.config.ContentPattern.EmbedTrackingMarker {
		m.embedMarker(content)
	}

	return content
}

// generateJSONContent 生成JSON格式内容
func (m *HoneyFileManager) generateJSONContent(content []byte) {
	template := `{"data": {"records": [], "metadata": {"created": "%s", "tracking": "%s"}}}`
	data := fmt.Sprintf(template, time.Now().Format(time.RFC3339), generateRandomString(8))
	copy(content, data)
	if len(data) < len(content) {
		rand.Read(content[len(data):])
	}
}

// generateXMLContent 生成XML格式内容
func (m *HoneyFileManager) generateXMLContent(content []byte) {
	template := `<document><metadata created="%s"/></document>`
	data := fmt.Sprintf(template, time.Now().Format(time.RFC3339))
	copy(content, data)
	if len(data) < len(content) {
		rand.Read(content[len(data):])
	}
}

// generateTextContent 生成文本内容
func (m *HoneyFileManager) generateTextContent(content []byte) {
	template := "Report generated: %s\nConfidential data - do not distribute\n\n"
	data := fmt.Sprintf(template, time.Now().Format(time.RFC3339))
	copy(content, data)
	if len(data) < len(content) {
		// 填充随机可打印字符
		for i := len(data); i < len(content); i++ {
			char, _ := rand.Int(rand.Reader, big.NewInt(94))
			content[i] = byte(32 + char.Int64()) // ASCII 32-126
		}
	}
}

// generateBinaryContent 生成二进制内容
func (m *HoneyFileManager) generateBinaryContent(content []byte) {
	rand.Read(content)
}

// generateRealisticContent 生成仿真内容
func (m *HoneyFileManager) generateRealisticContent(content []byte, ext string) {
	ext = strings.ToLower(ext)

	switch {
	case strings.Contains(ext, "doc"):
		m.generateDocumentContent(content)
	case strings.Contains(ext, "xls"):
		m.generateSpreadsheetContent(content)
	case strings.Contains(ext, "pdf"):
		m.generatePDFLikeContent(content)
	case strings.Contains(ext, "jpg") || strings.Contains(ext, "png"):
		m.generateImageLikeContent(content)
	case strings.Contains(ext, "zip"):
		m.generateArchiveLikeContent(content)
	default:
		rand.Read(content)
	}
}

// generateDocumentContent 生成文档仿真内容
func (m *HoneyFileManager) generateDocumentContent(content []byte) {
	header := "IMPORTANT DOCUMENT\n\nThis file contains confidential information.\n"
	copy(content, header)
	if len(header) < len(content) {
		for i := len(header); i < len(content); i++ {
			char, _ := rand.Int(rand.Reader, big.NewInt(94))
			content[i] = byte(32 + char.Int64())
		}
	}
}

// generateSpreadsheetContent 生成表格仿真内容
func (m *HoneyFileManager) generateSpreadsheetContent(content []byte) {
	header := "Financial Report\nDate,Amount,Category\n"
	copy(content, header)
	if len(header) < len(content) {
		rand.Read(content[len(header):])
	}
}

// generatePDFLikeContent 生成PDF仿真内容
func (m *HoneyFileManager) generatePDFLikeContent(content []byte) {
	// PDF文件头
	pdfHeader := "%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"
	copy(content, pdfHeader)
	if len(pdfHeader) < len(content) {
		rand.Read(content[len(pdfHeader):])
	}
}

// generateImageLikeContent 生成图像仿真内容
func (m *HoneyFileManager) generateImageLikeContent(content []byte) {
	// JPEG文件头
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	copy(content, jpegHeader)
	if len(jpegHeader) < len(content) {
		rand.Read(content[len(jpegHeader):])
	}
}

// generateArchiveLikeContent 生成压缩包仿真内容
func (m *HoneyFileManager) generateArchiveLikeContent(content []byte) {
	// ZIP文件头
	zipHeader := []byte{0x50, 0x4B, 0x03, 0x04}
	copy(content, zipHeader)
	if len(zipHeader) < len(content) {
		rand.Read(content[len(zipHeader):])
	}
}

// embedMarker 嵌入追踪标记
func (m *HoneyFileManager) embedMarker(content []byte) {
	if len(content) < 100 {
		return
	}

	marker := m.config.ContentPattern.TrackingMarkerPrefix + generateRandomString(16)
	// 在文件开头嵌入标记
	startPos := 50
	copy(content[startPos:startPos+len(marker)], []byte(marker))
}

// CheckAll 检查所有蜜罐文件状态
func (m *HoneyFileManager) CheckAll() []HoneyFileEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	var events []HoneyFileEvent

	for _, file := range m.files {
		event := m.checkFile(file)
		if event != nil {
			events = append(events, *event)
		}
	}

	m.stats.LastChecked = time.Now()
	m.stats.TotalEvents += int64(len(events))

	return events
}

// checkFile 检查单个蜜罐文件
func (m *HoneyFileManager) checkFile(file *HoneyFile) *HoneyFileEvent {
	oldStatus := file.Status

	// 检查文件是否存在
	info, err := os.Stat(file.Path)
	if os.IsNotExist(err) {
		file.Status = HoneyFileStatusDeleted
		file.LastChecked = time.Now()

		event := &HoneyFileEvent{
			ID:         generateEventID(),
			Timestamp:  time.Now(),
			FileID:     file.ID,
			FilePath:   file.Path,
			EventType:  HoneyFileEventDelete,
			OldStatus:  oldStatus,
			NewStatus:  HoneyFileStatusDeleted,
			ThreatLevel: ThreatLevelHigh,
		}

		m.handleThreat(event)
		return event
	}

	if err != nil {
		return nil // 其他错误忽略
	}

	// 检查文件大小变化
	if info.Size() != file.Size {
		file.Status = HoneyFileStatusModified
		file.LastChecked = time.Now()

		event := &HoneyFileEvent{
			ID:         generateEventID(),
			Timestamp:  time.Now(),
			FileID:     file.ID,
			FilePath:   file.Path,
			EventType:  HoneyFileEventModify,
			OldStatus:  oldStatus,
			NewStatus:  HoneyFileStatusModified,
			Details: map[string]interface{}{
				"old_size": file.Size,
				"new_size": info.Size(),
			},
			ThreatLevel: ThreatLevelHigh,
		}

		file.Size = info.Size()
		m.handleThreat(event)
		return event
	}

	// 检查文件名变化（重命名）
	if info.Name() != file.Name {
		file.Status = HoneyFileStatusRenamed
		file.Name = info.Name()
		file.LastChecked = time.Now()

		event := &HoneyFileEvent{
			ID:         generateEventID(),
			Timestamp:  time.Now(),
			FileID:     file.ID,
			FilePath:   file.Path,
			EventType:  HoneyFileEventRename,
			OldStatus:  oldStatus,
			NewStatus:  HoneyFileStatusRenamed,
			Details: map[string]interface{}{
				"old_name": filepath.Base(file.Path),
				"new_name": info.Name(),
				"old_ext":  file.Extension,
				"new_ext":  filepath.Ext(info.Name()),
			},
			ThreatLevel: ThreatLevelCritical,
		}

		m.handleThreat(event)
		return event
	}

	// 检查文件内容是否被加密
	content, err := os.ReadFile(file.Path)
	if err == nil && len(content) > 0 {
		if m.isContentEncrypted(content) {
			file.Status = HoneyFileStatusEncrypted
			file.LastChecked = time.Now()

			event := &HoneyFileEvent{
				ID:         generateEventID(),
				Timestamp:  time.Now(),
				FileID:     file.ID,
				FilePath:   file.Path,
				EventType:  HoneyFileEventEncrypt,
				OldStatus:  oldStatus,
				NewStatus:  HoneyFileStatusEncrypted,
				ThreatLevel: ThreatLevelCritical,
			}

			m.handleThreat(event)
			return event
		}
	}

	// 记录检查事件
	m.recordEvent(HoneyFileEvent{
		ID:         generateEventID(),
		Timestamp:  time.Now(),
		FileID:     file.ID,
		FilePath:   file.Path,
		EventType:  HoneyFileEventCheck,
		OldStatus:  oldStatus,
		NewStatus:  file.Status,
		ThreatLevel: ThreatLevelNone,
	})

	file.LastChecked = time.Now()
	return nil
}

// isContentEncrypted 检查内容是否被加密
func (m *HoneyFileManager) isContentEncrypted(content []byte) bool {
	// 检查熵值（加密文件通常高熵）
	entropy := calculateEntropy(content)
	if entropy > 7.5 {
		return true
	}

	// 检查追踪标记是否丢失（加密可能破坏标记）
	if m.config.ContentPattern.EmbedTrackingMarker {
		marker := m.config.ContentPattern.TrackingMarkerPrefix
		if !strings.Contains(string(content), marker) {
			return true
		}
	}

	return false
}

// handleThreat 处理威胁事件
func (m *HoneyFileManager) handleThreat(event *HoneyFileEvent) {
	m.stats.TriggeredFiles++
	m.stats.LastTriggered = time.Now()

	m.recordEvent(*event)

	// 生成告警
	alert := m.generateAlert(event)
	m.alerts <- alert
}

// generateAlert 生成告警
func (m *HoneyFileManager) generateAlert(event *HoneyFileEvent) HoneyFileAlert {
	severity := event.ThreatLevel

	var title, message string
	var recommendations []string

	switch event.EventType {
	case HoneyFileEventDelete:
		title = "蜜罐文件被删除 - 勒索软件攻击警告"
		message = fmt.Sprintf("蜜罐文件 %s 被删除，可能是勒索软件攻击", event.FilePath)
		recommendations = []string{
			"立即隔离受影响的共享目录",
			"检查最近登录的用户和进程",
			"启用只读模式保护其他文件",
			"准备从备份恢复",
		}

	case HoneyFileEventModify:
		title = "蜜罐文件被修改 - 勒索软件攻击警告"
		message = fmt.Sprintf("蜜罐文件 %s 被修改，可能是勒索软件加密活动", event.FilePath)
		recommendations = []string{
			"检查文件是否被加密",
			"追踪修改进程",
			"阻止可疑进程",
		}

	case HoneyFileEventRename:
		title = "蜜罐文件被重命名 - 勒索软件攻击警告"
		message = fmt.Sprintf("蜜罐文件 %s 被重命名，可能是勒索软件加密标记", event.FilePath)
		recommendations = []string{
			"检查新扩展名是否为勒索软件特征",
			"追踪重命名进程",
			"立即隔离系统",
		}

	case HoneyFileEventEncrypt:
		title = "蜜罐文件被加密 - 勒索软件攻击确认"
		message = fmt.Sprintf("蜜罐文件 %s 被加密，勒索软件攻击正在进行", event.FilePath)
		recommendations = []string{
			"立即断开网络连接",
			"终止所有可疑进程",
			"保护剩余文件",
			"联系安全团队",
			"准备恢复备份",
		}
		severity = ThreatLevelCritical

	default:
		title = "蜜罐文件异常事件"
		message = fmt.Sprintf("蜜罐文件 %s 发生异常事件", event.FilePath)
		recommendations = []string{"检查文件状态"}
	}

	return HoneyFileAlert{
		ID:            generateAlertID(),
		Timestamp:     time.Time{},
		Severity:      severity,
		Type:          "honeyfile_trigger",
		Title:         title,
		Message:       message,
		Events:        []HoneyFileEvent{*event},
		AffectedFiles: 1,
		Recommendations: recommendations,
		Status:        AlertStatusNew,
	}
}

// recordEvent 记录事件
func (m *HoneyFileManager) recordEvent(event HoneyFileEvent) {
	m.events = append(m.events, event)
	// 保持事件列表在合理大小
	if len(m.events) > 10000 {
		m.events = m.events[len(m.events)-5000:]
	}
}

// Alerts 返回告警通道
func (m *HoneyFileManager) Alerts() <-chan HoneyFileAlert {
	return m.alerts
}

// GetFiles 获取所有蜜罐文件
func (m *HoneyFileManager) GetFiles() []*HoneyFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]*HoneyFile, 0, len(m.files))
	for _, file := range m.files {
		files = append(files, file)
	}
	return files
}

// GetFilesByPath 获取指定路径的蜜罐文件
func (m *HoneyFileManager) GetFilesByPath(path string) []*HoneyFile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := m.pathIndex[path]
	files := make([]*HoneyFile, 0, len(ids))
	for _, id := range ids {
		if file, ok := m.files[id]; ok {
			files = append(files, file)
		}
	}
	return files
}

// GetStats 获取统计信息
func (m *HoneyFileManager) GetStats() HoneyFileStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// GetEvents 获取事件记录
func (m *HoneyFileManager) GetEvents(limit int) []HoneyFileEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	return m.events[start:]
}

// Redeploy 重新部署被触发的蜜罐文件
func (m *HoneyFileManager) Redeploy() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	triggeredCount := 0
	for _, file := range m.files {
		if file.Status != HoneyFileStatusActive {
			triggeredCount++

			// 删除旧文件（如果还存在）
			os.Remove(file.Path)

			// 创建新蜜罐文件
			newFile, err := m.createHoneyFile(file.DeployPathID)
			if err != nil {
				continue
			}

			// 更新索引
			m.files[newFile.ID] = newFile

			// 更新路径索引
			pathFiles := m.pathIndex[file.DeployPathID]
			for i, id := range pathFiles {
				if id == file.ID {
					pathFiles[i] = newFile.ID
					break
				}
			}
			m.pathIndex[file.DeployPathID] = pathFiles

			// 删除旧记录
			delete(m.files, file.ID)
		}
	}

	m.stats.TriggeredFiles = 0
	m.stats.ActiveFiles = m.stats.TotalFiles
	m.stats.LastDeployed = time.Now()

	return nil
}

// StartMonitoring 启动持续监控
func (m *HoneyFileManager) StartMonitoring(ctx interface{ Done() <-chan struct{} }) {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	go m.monitorLoop(ctx)
}

// StopMonitoring 停止监控
func (m *HoneyFileManager) StopMonitoring() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
}

// monitorLoop 监控循环
func (m *HoneyFileManager) monitorLoop(ctx interface{ Done() <-chan struct{} }) {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.running {
				return
			}
			m.CheckAll()
		}
	}
}

// 辅助函数

func generateHoneyFileID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "hf_" + hex.EncodeToString(b)
}

func generateEventID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "he_" + hex.EncodeToString(b)
}

func generateAlertID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "ha_" + hex.EncodeToString(b)
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// AddToDetector 将蜜罐检测集成到勒索软件检测器
func (d *Detector) EnableHoneyFileDetection(config HoneyFileConfig) error {
	manager, err := NewHoneyFileManager(config)
	if err != nil {
		return err
	}

	d.mu.Lock()
	// 存储蜜罐管理器引用（需要添加到Detector结构）
	d.mu.Unlock()

	// 监听蜜罐告警
	go func() {
		for alert := range manager.Alerts() {
			d.handleAlert(DetectorAlert{
				ID:          alert.ID,
				Type:        alert.Type,
				Title:       alert.Title,
				Message:     alert.Message,
				RiskScore:   int(threatLevelToScore(alert.Severity)),
				ThreatType:  "ransomware_honeyfile",
				Recommendations: alert.Recommendations,
				Timestamp:   alert.Timestamp,
			})
		}
	}()

	return nil
}

func threatLevelToScore(level ThreatLevel) int {
	switch level {
	case ThreatLevelLow:
		return 25
	case ThreatLevelMedium:
		return 50
	case ThreatLevelHigh:
		return 75
	case ThreatLevelCritical:
		return 100
	default:
		return 0
	}
}
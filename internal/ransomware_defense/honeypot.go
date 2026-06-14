// Package ransomware_defense 提供勒索软件防护模块
// honeypot.go - 蜜罐文件部署与监控
package ransomware_defense

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// 蜜罐文件模板
// =============================================================================

// honeypotTemplate 蜜罐文件模板
type honeypotTemplate struct {
	// FileType 文件类型
	FileType string

	// Extensions 常见扩展名
	Extensions []string

	// MinSize 最小文件大小
	MinSize int64

	// MaxSize 最大文件大小
	MaxSize int64

	// Names 常见文件名前缀
	Names []string
}

// getHoneypotTemplates 获取蜜罐文件模板列表
func getHoneypotTemplates() []honeypotTemplate {
	return []honeypotTemplate{
		{
			FileType:   "document",
			Extensions: []string{".docx", ".xlsx", ".pdf", ".doc", ".xls"},
			MinSize:    10240,
			MaxSize:    204800,
			Names: []string{
				"财务报表", "工资单", "合同", "税务", "发票",
				"Budget_Report", "Invoice", "Contract", "Payroll", "Tax_Return",
			},
		},
		{
			FileType:   "image",
			Extensions: []string{".jpg", ".png", ".bmp", ".tiff"},
			MinSize:    51200,
			MaxSize:    5242880,
			Names: []string{
				"家庭照片", "旅行", "婚礼", "毕业",
				"Family_Photo", "Vacation", "Birthday", "Graduation",
			},
		},
		{
			FileType:   "database",
			Extensions: []string{".db", ".sqlite", ".mdb", ".accdb"},
			MinSize:    102400,
			MaxSize:    10485760,
			Names: []string{
				"客户数据库", "订单记录", "库存",
				"Customer_DB", "Orders", "Inventory", "Records",
			},
		},
		{
			FileType:   "archive",
			Extensions: []string{".zip", ".rar", ".7z", ".tar.gz"},
			MinSize:    102400,
			MaxSize:    52428800,
			Names: []string{
				"备份", "归档", "重要文件",
				"Backup", "Archive", "Important_Files", "Project_Files",
			},
		},
		{
			FileType:   "code",
			Extensions: []string{".py", ".go", ".js", ".java", ".cpp"},
			MinSize:    1024,
			MaxSize:    102400,
			Names: []string{
				"源码", "配置脚本", "自动化",
				"source_code", "config_script", "automation", "main",
			},
		},
	}
}

// =============================================================================
// HoneypotManager 蜜罐管理器
// =============================================================================

// HoneypotManager 蜜罐管理器
type HoneypotManager struct {
	mu        sync.RWMutex
	config    HoneypotConfig
	honeypots map[string]*HoneypotFile
	running   bool
	stopCh    chan struct{}

	// onTrigger 蜜罐触发回调
	onTrigger func(hp *HoneypotFile, activity FileActivity)
}

// NewHoneypotManager 创建新的蜜罐管理器
func NewHoneypotManager(config HoneypotConfig) *HoneypotManager {
	return &HoneypotManager{
		config:    config,
		honeypots: make(map[string]*HoneypotFile),
		stopCh:    make(chan struct{}),
	}
}

// SetTriggerCallback 设置触发回调函数
func (hm *HoneypotManager) SetTriggerCallback(callback func(hp *HoneypotFile, activity FileActivity)) {
	hm.mu.Lock()
	hm.onTrigger = callback
	hm.mu.Unlock()
}

// Start 启动蜜罐管理器
func (hm *HoneypotManager) Start() error {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return nil
	}
	hm.running = true
	hm.mu.Unlock()

	// 自动部署蜜罐到监控路径
	if hm.config.AutoDeploy {
		if err := hm.DeployAll(); err != nil {
			log.Printf("自动部署蜜罐警告: %v", err)
		}
	}

	// 启动监控循环
	go hm.monitorLoop()

	log.Println("✅ 蜜罐管理器已启动")
	return nil
}

// Stop 停止蜜罐管理器
func (hm *HoneypotManager) Stop() {
	hm.mu.Lock()
	if !hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = false
	close(hm.stopCh)
	hm.mu.Unlock()

	log.Println("蜜罐管理器已停止")
}

// DeployAll 部署所有蜜罐
func (hm *HoneypotManager) DeployAll() error {
	hm.mu.RLock()
	paths := hm.config.DeploymentPaths
	hm.mu.RUnlock()

	if len(paths) == 0 {
		return fmt.Errorf("未配置部署路径")
	}

	var lastErr error
	for _, path := range paths {
		if err := hm.DeployToPath(path); err != nil {
			log.Printf("部署蜜罐到 %s 失败: %v", path, err)
			lastErr = err
		}
	}
	return lastErr
}

// DeployToPath 部署蜜罐到指定路径
func (hm *HoneypotManager) DeployToPath(basePath string) error {
	// 检查路径是否存在
	info, err := os.Stat(basePath)
	if err != nil {
		return fmt.Errorf("路径不存在: %s: %w", basePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", basePath)
	}

	templates := getHoneypotTemplates()
	deployed := 0

	for _, tmpl := range templates {
		for i := 0; i < hm.config.HoneypotDensity; i++ {
			hp, err := hm.createHoneypot(basePath, tmpl)
			if err != nil {
				log.Printf("创建蜜罐文件失败: %v", err)
				continue
			}
			hm.mu.Lock()
			hm.honeypots[hp.ID] = hp
			hm.mu.Unlock()
			deployed++
		}
	}

	log.Printf("在 %s 部署了 %d 个蜜罐文件", basePath, deployed)
	return nil
}

// createHoneypot 创建单个蜜罐文件
func (hm *HoneypotManager) createHoneypot(basePath string, tmpl honeypotTemplate) (*HoneypotFile, error) {
	// 选择随机扩展名
	ext := tmpl.Extensions[randomInt(len(tmpl.Extensions))]

	// 选择随机文件名
	namePrefix := tmpl.Names[randomInt(len(tmpl.Names))]
	fileName := fmt.Sprintf("%s_%s%s", namePrefix, randomHex(4), ext)
	filePath := filepath.Join(basePath, fileName)

	// 生成随机内容（模拟真实文件）
	size := tmpl.MinSize + int64(randomInt(int(tmpl.MaxSize-tmpl.MinSize)))
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		return nil, fmt.Errorf("生成随机内容失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return nil, fmt.Errorf("写入蜜罐文件失败: %w", err)
	}

	// 计算哈希
	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	hp := &HoneypotFile{
		ID:           fmt.Sprintf("hp-%s-%d", randomHex(8), time.Now().UnixNano()),
		Path:         filePath,
		FileName:     fileName,
		FileType:     tmpl.FileType,
		ContentHash:  contentHash,
		FileSize:     size,
		ShareName:    filepath.Base(basePath),
		Protocol:     ProtocolSMB,
		Enabled:      true,
		Tags:         []string{"auto-deploy", tmpl.FileType},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastCheckedAt: time.Now(),
		TriggerCount: 0,
	}

	return hp, nil
}

// CheckActivity 检查文件活动是否触碰了蜜罐
func (hm *HoneypotManager) CheckActivity(activity FileActivity) *HoneypotFile {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for _, hp := range hm.honeypots {
		if !hp.Enabled {
			continue
		}

		// 路径匹配
		if activity.Path == hp.Path || activity.OldPath == hp.Path {
			// 检查是否被修改
			if activity.Operation == FileOpModify ||
				activity.Operation == FileOpDelete ||
				activity.Operation == FileOpRename {

				// 验证内容确实被改变
				if hm.isContentChanged(hp) {
					hp.TriggerCount++
					hp.LastCheckedAt = time.Now()

					if hm.onTrigger != nil {
						go hm.onTrigger(hp, activity)
					}
					return hp
				}
			}
		}
	}
	return nil
}

// isContentChanged 检查蜜罐文件内容是否被改变
func (hm *HoneypotManager) isContentChanged(hp *HoneypotFile) bool {
	data, err := os.ReadFile(hp.Path)
	if err != nil {
		// 文件被删除也视为触发
		return true
	}

	hash := sha256.Sum256(data)
	currentHash := hex.EncodeToString(hash[:])
	return currentHash != hp.ContentHash
}

// monitorLoop 蜜罐文件监控循环
func (hm *HoneypotManager) monitorLoop() {
	ticker := time.NewTicker(time.Duration(hm.config.CheckIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-hm.stopCh:
			return
		case <-ticker.C:
			hm.checkAllHoneypots()
		}
	}
}

// checkAllHoneypots 检查所有蜜罐文件
func (hm *HoneypotManager) checkAllHoneypots() {
	hm.mu.RLock()
	honeypots := make([]*HoneypotFile, 0, len(hm.honeypots))
	for _, hp := range hm.honeypots {
		if hp.Enabled {
			honeypots = append(honeypots, hp)
		}
	}
	hm.mu.RUnlock()

	for _, hp := range honeypots {
		if hm.isContentChanged(hp) {
			hm.mu.Lock()
			hp.TriggerCount++
			hp.LastCheckedAt = time.Now()
			hm.mu.Unlock()

			log.Printf("⚠️ 蜜罐文件被修改: %s (ID: %s)", hp.Path, hp.ID)

			// 构造虚拟活动用于回调
			activity := FileActivity{
				Path:      hp.Path,
				Operation: FileOpModify,
				Timestamp: time.Now(),
			}

			hm.mu.RLock()
			cb := hm.onTrigger
			hm.mu.RUnlock()

			if cb != nil {
				go cb(hp, activity)
			}
		}
	}
}

// GetHoneypots 获取所有蜜罐列表
func (hm *HoneypotManager) GetHoneypots() []*HoneypotFile {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]*HoneypotFile, 0, len(hm.honeypots))
	for _, hp := range hm.honeypots {
		result = append(result, hp)
	}
	return result
}

// GetHoneypot 获取单个蜜罐
func (hm *HoneypotManager) GetHoneypot(id string) (*HoneypotFile, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	hp, ok := hm.honeypots[id]
	return hp, ok
}

// AddHoneypot 手动添加蜜罐
func (hm *HoneypotManager) AddHoneypot(filePath string, fileType string, tags []string) (*HoneypotFile, error) {
	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %s: %w", filePath, err)
	}

	// 计算哈希
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	hash := sha256.Sum256(data)
	contentHash := hex.EncodeToString(hash[:])

	hp := &HoneypotFile{
		ID:           fmt.Sprintf("hp-%s-%d", randomHex(8), time.Now().UnixNano()),
		Path:         filePath,
		FileName:     filepath.Base(filePath),
		FileType:     fileType,
		ContentHash:  contentHash,
		FileSize:     info.Size(),
		Protocol:     ProtocolSMB,
		Enabled:      true,
		Tags:         tags,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastCheckedAt: time.Now(),
	}

	hm.mu.Lock()
	// 检查重复
	for _, existing := range hm.honeypots {
		if existing.Path == filePath {
			hm.mu.Unlock()
			return nil, fmt.Errorf("蜜罐已存在: %s", filePath)
		}
	}
	hm.honeypots[hp.ID] = hp
	hm.mu.Unlock()

	log.Printf("手动添加蜜罐: %s (ID: %s)", filePath, hp.ID)
	return hp, nil
}

// RemoveHoneypot 移除蜜罐
func (hm *HoneypotManager) RemoveHoneypot(id string, deleteFile bool) error {
	hm.mu.Lock()
	hp, ok := hm.honeypots[id]
	if !ok {
		hm.mu.Unlock()
		return fmt.Errorf("蜜罐不存在: %s", id)
	}
	delete(hm.honeypots, id)
	hm.mu.Unlock()

	if deleteFile {
		if err := os.Remove(hp.Path); err != nil && !os.IsNotExist(err) {
			log.Printf("删除蜜罐文件失败: %v", err)
		}
	}

	log.Printf("移除蜜罐: %s (ID: %s)", hp.Path, id)
	return nil
}

// EnableHoneypot 启用蜜罐
func (hm *HoneypotManager) EnableHoneypot(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hp, ok := hm.honeypots[id]
	if !ok {
		return fmt.Errorf("蜜罐不存在: %s", id)
	}
	hp.Enabled = true
	hp.UpdatedAt = time.Now()
	return nil
}

// DisableHoneypot 禁用蜜罐
func (hm *HoneypotManager) DisableHoneypot(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hp, ok := hm.honeypots[id]
	if !ok {
		return fmt.Errorf("蜜罐不存在: %s", id)
	}
	hp.Enabled = false
	hp.UpdatedAt = time.Now()
	return nil
}

// RefreshHoneypotHash 刷新蜜罐文件哈希（用于合法更新后）
func (hm *HoneypotManager) RefreshHoneypotHash(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hp, ok := hm.honeypots[id]
	if !ok {
		return fmt.Errorf("蜜罐不存在: %s", id)
	}

	data, err := os.ReadFile(hp.Path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	hash := sha256.Sum256(data)
	hp.ContentHash = hex.EncodeToString(hash[:])
	hp.UpdatedAt = time.Now()
	return nil
}

// GetStatus 获取蜜罐状态
func (hm *HoneypotManager) GetStatus() HoneypotStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := HoneypotStatus{
		TotalHoneypots: len(hm.honeypots),
	}

	shareSet := make(map[string]bool)
	for _, hp := range hm.honeypots {
		if hp.Enabled {
			status.ActiveHoneypots++
		}
		if hp.TriggerCount > 0 {
			status.TriggeredHoneypots++
		}
		if hp.ShareName != "" {
			shareSet[hp.ShareName] = true
		}
	}

	for share := range shareSet {
		status.MonitoredShares = append(status.MonitoredShares, share)
	}

	return status
}

// UpdateConfig 更新蜜罐配置
func (hm *HoneypotManager) UpdateConfig(config HoneypotConfig) {
	hm.mu.Lock()
	hm.config = config
	hm.mu.Unlock()
	log.Println("蜜罐配置已更新")
}

// GetConfig 获取蜜罐配置
func (hm *HoneypotManager) GetConfig() HoneypotConfig {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return hm.config
}

// =============================================================================
// 辅助函数
// =============================================================================

// randomInt 返回 [0, n) 的随机整数
func randomInt(n int) int {
	if n <= 0 {
		return 0
	}
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	val := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if val < 0 {
		val = -val
	}
	return val % n
}

// randomHex 返回指定长度的随机十六进制字符串
func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// IsHoneypotPath 检查路径是否为已知蜜罐路径（供外部快速查询）
func (hm *HoneypotManager) IsHoneypotPath(path string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	for _, hp := range hm.honeypots {
		if hp.Enabled && (hp.Path == path || strings.HasPrefix(path, filepath.Dir(hp.Path))) {
			return true
		}
	}
	return false
}

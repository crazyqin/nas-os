// Package ransomshield - 蜜罐文件系统
// 分层蜜罐部署、智能诱饵生成、访问追踪、异常行为触发
package ransomware

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	mrand "math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 蜜罐文件系统管理器
// ============================================================

// HoneypotManager 蜜罐文件系统管理器.
type HoneypotManager struct {
	mu sync.RWMutex

	// config 蜜罐配置
	config HoneypotConfig

	// files 所有蜜罐文件 (id -> HoneypotFile)
	files map[string]*HoneypotFile

	// pathIndex 路径索引 (path -> id)
	pathIndex map[string]string

	// accessLog 访问日志
	accessLog []HoneypotAccessEvent

	// templates 诱饵文件模板
	templates []DecoyTemplate

	// layers 分层蜜罐配置
	layers []HoneypotLayer

	// stats 统计信息
	stats HoneypotStats

	// onTrigger 触发回调
	onTrigger func(event HoneypotAccessEvent)

	// running 运行状态
	running  bool
	stopChan chan struct{}
}

// HoneypotLayer 蜜罐部署层.
type HoneypotLayer struct {
	Name      string   `json:"name"`       // 层名称
	Path      string   `json:"path"`       // 部署路径
	Depth     int      `json:"depth"`      // 目录深度
	FileCount int      `json:"file_count"` // 文件数量
	Types     []string `json:"types"`      // 文件类型
	Visible   bool     `json:"visible"`    // 是否可见（隐藏文件夹）
	Weight    int      `json:"weight"`     // 触发权重
}

// DecoyTemplate 诱饵文件模板.
type DecoyTemplate struct {
	Name        string `json:"name"`        // 模板名
	Extension   string `json:"extension"`   // 扩展名
	MinSizeKB   int    `json:"min_size_kb"` // 最小大小
	MaxSizeKB   int    `json:"max_size_kb"` // 最大大小
	Category    string `json:"category"`    // 分类: doc, image, financial, etc.
	HeaderBytes []byte `json:"-"`           // 文件头（模拟真实格式）
}

// HoneypotAccessEvent 蜜罐访问事件.
type HoneypotAccessEvent struct {
	ID          string      `json:"id"`
	HoneypotID  string      `json:"honeypot_id"`
	FilePath    string      `json:"file_path"`
	AccessTime  time.Time   `json:"access_time"`
	AccessMode  string      `json:"access_mode"` // read, write, delete, rename
	ProcessName string      `json:"process_name"`
	ProcessID   int         `json:"process_id"`
	UserID      int         `json:"user_id"`
	ThreatLevel ThreatLevel `json:"threat_level"`
	SourceIP    string      `json:"source_ip,omitempty"`
}

// HoneypotStats 蜜罐统计.
type HoneypotStats struct {
	TotalDeployed   int       `json:"total_deployed"`
	TotalTriggered  int64     `json:"total_triggered"`
	ActiveHoneypots int       `json:"active_honeypots"`
	LastRefreshTime time.Time `json:"last_refresh_time"`
	LastTriggerTime time.Time `json:"last_trigger_time"`
	AccessEvents24h int       `json:"access_events_24h"`
	UniqueAttackers int       `json:"unique_attackers"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewHoneypotManager 创建蜜罐管理器.
func NewHoneypotManager(config HoneypotConfig) *HoneypotManager {
	hm := &HoneypotManager{
		config:    config,
		files:     make(map[string]*HoneypotFile),
		pathIndex: make(map[string]string),
		accessLog: make([]HoneypotAccessEvent, 0, 5000),
		stopChan:  make(chan struct{}),
	}

	hm.initTemplates()
	hm.initLayers()
	return hm
}

// initTemplates 初始化诱饵模板.
func (hm *HoneypotManager) initTemplates() {
	hm.templates = []DecoyTemplate{
		{Name: "财务报表", Extension: ".xlsx", MinSizeKB: 10, MaxSizeKB: 500, Category: "financial",
			HeaderBytes: []byte{0x50, 0x4B, 0x03, 0x04}}, // ZIP header (xlsx is zip)
		{Name: "合同文档", Extension: ".docx", MinSizeKB: 5, MaxSizeKB: 200, Category: "document",
			HeaderBytes: []byte{0x50, 0x4B, 0x03, 0x04}},
		{Name: "客户名单", Extension: ".csv", MinSizeKB: 2, MaxSizeKB: 100, Category: "data"},
		{Name: "工资明细", Extension: ".pdf", MinSizeKB: 20, MaxSizeKB: 800, Category: "financial",
			HeaderBytes: []byte{0x25, 0x50, 0x44, 0x46}}, // %PDF
		{Name: "项目照片", Extension: ".jpg", MinSizeKB: 100, MaxSizeKB: 5000, Category: "image",
			HeaderBytes: []byte{0xFF, 0xD8, 0xFF}}, // JPEG
		{Name: "身份证扫描", Extension: ".png", MinSizeKB: 200, MaxSizeKB: 3000, Category: "sensitive",
			HeaderBytes: []byte{0x89, 0x50, 0x4E, 0x47}}, // PNG
		{Name: "密码备份", Extension: ".txt", MinSizeKB: 1, MaxSizeKB: 10, Category: "credential"},
		{Name: "税务记录", Extension: ".xlsx", MinSizeKB: 50, MaxSizeKB: 1000, Category: "financial",
			HeaderBytes: []byte{0x50, 0x4B, 0x03, 0x04}},
		{Name: "数据库备份", Extension: ".sql", MinSizeKB: 100, MaxSizeKB: 5000, Category: "database"},
		{Name: "银行对账单", Extension: ".pdf", MinSizeKB: 30, MaxSizeKB: 600, Category: "financial",
			HeaderBytes: []byte{0x25, 0x50, 0x44, 0x46}},
	}
}

// initLayers 初始化分层蜜罐.
func (hm *HoneypotManager) initLayers() {
	if len(hm.config.BasePaths) == 0 {
		return
	}

	basePath := hm.config.BasePaths[0]
	hm.layers = []HoneypotLayer{
		{
			Name: "surface", Path: filepath.Join(basePath, "Documents"),
			Depth: 0, FileCount: 5, Types: []string{"doc", "image"}, Visible: true, Weight: 30,
		},
		{
			Name: "financial", Path: filepath.Join(basePath, "Finance"),
			Depth: 1, FileCount: 8, Types: []string{"financial", "document"}, Visible: true, Weight: 50,
		},
		{
			Name: "hidden", Path: filepath.Join(basePath, ".backup"),
			Depth: 2, FileCount: 5, Types: []string{"credential", "database"}, Visible: false, Weight: 80,
		},
		{
			Name: "deep", Path: filepath.Join(basePath, ".snapshots", "weekly"),
			Depth: 3, FileCount: 4, Types: []string{"sensitive", "financial"}, Visible: false, Weight: 100,
		},
	}
}

// SetTriggerCallback 设置触发回调.
func (hm *HoneypotManager) SetTriggerCallback(fn func(event HoneypotAccessEvent)) {
	hm.mu.Lock()
	hm.onTrigger = fn
	hm.mu.Unlock()
}

// Start 启动蜜罐管理器.
func (hm *HoneypotManager) Start() error {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return nil
	}
	hm.running = true
	hm.mu.Unlock()

	if err := hm.DeployAll(); err != nil {
		log.Printf("[Honeypot] 部署失败: %v", err)
	}

	go hm.refreshLoop()
	go hm.monitorLoop()

	log.Println("[Honeypot] 蜜罐管理器已启动")
	return nil
}

// Stop 停止蜜罐管理器.
func (hm *HoneypotManager) Stop() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	if !hm.running {
		return
	}
	close(hm.stopChan)
	hm.running = false
	log.Println("[Honeypot] 蜜罐管理器已停止")
}

// ============================================================
// 部署
// ============================================================

// DeployAll 部署所有层的蜜罐.
func (hm *HoneypotManager) DeployAll() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	totalDeployed := 0
	for _, layer := range hm.layers {
		count, err := hm.deployLayer(layer)
		if err != nil {
			log.Printf("[Honeypot] 部署层 %s 失败: %v", layer.Name, err)
			continue
		}
		totalDeployed += count
	}

	// 使用配置的额外路径
	for _, basePath := range hm.config.BasePaths {
		honeypotDir := filepath.Join(basePath, ".honeypot")
		count, err := hm.deployToPath(honeypotDir, hm.config.FileCount)
		if err != nil {
			log.Printf("[Honeypot] 部署到 %s 失败: %v", honeypotDir, err)
			continue
		}
		totalDeployed += count
	}

	hm.stats.TotalDeployed = totalDeployed
	hm.stats.ActiveHoneypots = totalDeployed
	hm.stats.LastRefreshTime = time.Now()

	log.Printf("[Honeypot] 总共部署 %d 个蜜罐文件", totalDeployed)
	return nil
}

// deployLayer 部署单层蜜罐.
func (hm *HoneypotManager) deployLayer(layer HoneypotLayer) (int, error) {
	if err := os.MkdirAll(layer.Path, 0755); err != nil {
		return 0, fmt.Errorf("创建目录 %s: %w", layer.Path, err)
	}

	// 在各深度创建子目录和文件
	count := 0
	for d := 0; d <= layer.Depth; d++ {
		dirPath := layer.Path
		if d > 0 {
			subDir := fmt.Sprintf("sub_%d", d)
			dirPath = filepath.Join(dirPath, subDir)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				continue
			}
		}

		for i := 0; i < layer.FileCount/(layer.Depth+1); i++ {
			if err := hm.createDecoyFile(dirPath, layer.Types, layer.Weight); err == nil {
				count++
			}
		}
	}

	return count, nil
}

// deployToPath 部署蜜罐到指定路径.
func (hm *HoneypotManager) deployToPath(dir string, count int) (int, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, err
	}

	deployed := 0
	for i := 0; i < count; i++ {
		if err := hm.createDecoyFile(dir, nil, 50); err == nil {
			deployed++
		}
	}
	return deployed, nil
}

// createDecoyFile 创建诱饵文件.
func (hm *HoneypotManager) createDecoyFile(dir string, categories []string, weight int) error {
	// 选择模板
	tmpl := hm.selectTemplate(categories)

	// 生成文件名（模拟真实文件命名）
	filename := hm.generateFilename(tmpl)
	filePath := filepath.Join(dir, filename)

	// 检查是否已存在
	if _, exists := hm.pathIndex[filePath]; exists {
		return nil
	}

	// 生成文件大小
	sizeKB := tmpl.MinSizeKB
	if tmpl.MaxSizeKB > tmpl.MinSizeKB {
		sizeKB = tmpl.MinSizeKB + mrand.Intn(tmpl.MaxSizeKB-tmpl.MinSizeKB)
	}
	size := sizeKB * 1024

	// 生成文件内容
	data, err := hm.generateFileContent(tmpl, size)
	if err != nil {
		return err
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	// 计算哈希
	hash := sha256.Sum256(data)

	// 注册蜜罐
	id := uuid.New().String()
	hp := &HoneypotFile{
		ID:        id,
		Path:      filePath,
		Name:      filename,
		SizeBytes: int64(size),
		Extension: tmpl.Extension,
		Hash:      hex.EncodeToString(hash[:]),
		CreatedAt: time.Now(),
	}

	hm.files[id] = hp
	hm.pathIndex[filePath] = id

	return nil
}

// selectTemplate 选择诱饵模板.
func (hm *HoneypotManager) selectTemplate(categories []string) DecoyTemplate {
	if len(categories) == 0 {
		return hm.templates[mrand.Intn(len(hm.templates))]
	}

	// 过滤匹配分类的模板
	var matched []DecoyTemplate
	for _, tmpl := range hm.templates {
		for _, cat := range categories {
			if tmpl.Category == cat {
				matched = append(matched, tmpl)
				break
			}
		}
	}

	if len(matched) == 0 {
		return hm.templates[mrand.Intn(len(hm.templates))]
	}
	return matched[mrand.Intn(len(matched))]
}

// generateFilename 生成逼真的文件名.
func (hm *HoneypotManager) generateFilename(tmpl DecoyTemplate) string {
	prefixes := map[string][]string{
		"financial":  {"2024年度", "Q4季度", "12月份", "年度汇总"},
		"document":   {"合同_正式", "协议_最终", "审批_已签"},
		"data":       {"客户数据", "导出数据", "备份数据"},
		"image":      {"IMG_", "DSC_", "Photo_"},
		"sensitive":  {"扫描件_", "复印_", "认证_"},
		"credential": {"passwords", "credentials", "backup_keys"},
		"database":   {"dump_", "backup_", "export_"},
	}

	category := tmpl.Category
	pfxList, ok := prefixes[category]
	if !ok {
		pfxList = []string{"文件_", "文档_", "data_"}
	}

	prefix := pfxList[mrand.Intn(len(pfxList))]

	// 使用时间戳+随机数避免重复
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	return prefix + suffix + tmpl.Extension
}

// generateFileContent 生成文件内容.
func (hm *HoneypotManager) generateFileContent(tmpl DecoyTemplate, size int) ([]byte, error) {
	data := make([]byte, size)

	// 写入文件头
	if len(tmpl.HeaderBytes) > 0 {
		copy(data, tmpl.HeaderBytes)
	}

	// 填充随机数据（模拟加密/压缩后的数据）
	if _, err := rand.Read(data[len(tmpl.HeaderBytes):]); err != nil {
		return nil, err
	}

	return data, nil
}

// ============================================================
// 访问监控
// ============================================================

// RecordAccess 记录蜜罐访问.
func (hm *HoneypotManager) RecordAccess(path string, accessMode string, procName string, procID int, userID int, sourceIP string) *HoneypotAccessEvent {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// 查找对应的蜜罐文件
	id, exists := hm.pathIndex[path]
	if !exists {
		// 检查是否在蜜罐目录下
		for hpPath, hpID := range hm.pathIndex {
			if strings.HasPrefix(path, filepath.Dir(hpPath)) {
				id = hpID
				exists = true
				break
			}
		}
	}
	if !exists {
		return nil
	}

	hp := hm.files[id]
	now := time.Now()

	// 更新蜜罐状态
	hp.Triggered = true
	hp.TriggerCount++
	hp.LastTrigger = &now

	// 创建访问事件
	event := HoneypotAccessEvent{
		ID:          uuid.New().String(),
		HoneypotID:  id,
		FilePath:    path,
		AccessTime:  now,
		AccessMode:  accessMode,
		ProcessName: procName,
		ProcessID:   procID,
		UserID:      userID,
		SourceIP:    sourceIP,
		ThreatLevel: hm.evaluateAccessThreat(hp, accessMode),
	}

	hm.accessLog = append(hm.accessLog, event)
	hm.stats.TotalTriggered++
	hm.stats.LastTriggerTime = now

	// 保持日志在合理范围
	if len(hm.accessLog) > 10000 {
		hm.accessLog = hm.accessLog[1:]
	}

	// 触发回调
	if hm.onTrigger != nil {
		go hm.onTrigger(event)
	}

	log.Printf("[Honeypot] 蜜罐访问: 文件=%s, 模式=%s, 进程=%s(%d), 威胁=%s",
		path, accessMode, procName, procID, event.ThreatLevel.String())

	return &event
}

// evaluateAccessThreat 评估蜜罐访问的威胁级别.
func (hm *HoneypotManager) evaluateAccessThreat(hp *HoneypotFile, accessMode string) ThreatLevel {
	// 写入/删除蜜罐 = 严重威胁
	switch accessMode {
	case "write", "delete", "rename":
		return ThreatLevelCritical
	case "read":
		// 多次读取也可能是侦察
		if hp.TriggerCount > 3 {
			return ThreatLevelHigh
		}
		return ThreatLevelMedium
	default:
		return ThreatLevelMedium
	}
}

// ============================================================
// 查询
// ============================================================

// GetAll 获取所有蜜罐文件.
func (hm *HoneypotManager) GetAll() []HoneypotFile {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]HoneypotFile, 0, len(hm.files))
	for _, hp := range hm.files {
		result = append(result, *hp)
	}
	return result
}

// GetTriggered 获取已触发的蜜罐.
func (hm *HoneypotManager) GetTriggered() []HoneypotFile {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var result []HoneypotFile
	for _, hp := range hm.files {
		if hp.Triggered {
			result = append(result, *hp)
		}
	}
	return result
}

// GetAccessLog 获取访问日志.
func (hm *HoneypotManager) GetAccessLog(limit int) []HoneypotAccessEvent {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if limit <= 0 || limit > len(hm.accessLog) {
		limit = len(hm.accessLog)
	}

	start := len(hm.accessLog) - limit
	result := make([]HoneypotAccessEvent, limit)
	copy(result, hm.accessLog[start:])
	return result
}

// IsHoneypot 检查路径是否为蜜罐.
func (hm *HoneypotManager) IsHoneypot(path string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if _, exists := hm.pathIndex[path]; exists {
		return true
	}
	for hpPath := range hm.pathIndex {
		if strings.HasPrefix(path, filepath.Dir(hpPath)) {
			return true
		}
	}
	return false
}

// GetStats 获取统计信息.
func (hm *HoneypotManager) GetStats() HoneypotStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	stats := hm.stats
	stats.ActiveHoneypots = len(hm.files)

	// 计算24小时内事件数
	cutoff := time.Now().Add(-24 * time.Hour)
	count24h := 0
	attackers := make(map[int]bool)
	for _, evt := range hm.accessLog {
		if evt.AccessTime.After(cutoff) {
			count24h++
			attackers[evt.UserID] = true
		}
	}
	stats.AccessEvents24h = count24h
	stats.UniqueAttackers = len(attackers)

	return stats
}

// ============================================================
// 维护循环
// ============================================================

// refreshLoop 定期刷新蜜罐.
func (hm *HoneypotManager) refreshLoop() {
	interval := time.Duration(hm.config.RefreshIntervalMin) * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.stopChan:
			return
		case <-ticker.C:
			hm.refresh()
		}
	}
}

// refresh 刷新蜜罐文件.
func (hm *HoneypotManager) refresh() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	log.Println("[Honeypot] 开始刷新蜜罐文件...")

	// 删除旧蜜罐
	for id, hp := range hm.files {
		os.Remove(hp.Path)
		delete(hm.pathIndex, hp.Path)
		delete(hm.files, id)
	}

	// 重新部署
	hm.stats = HoneypotStats{}

	// 释放锁后重新部署
	hm.mu.Unlock()
	hm.DeployAll()
	hm.mu.Lock()

	log.Println("[Honeypot] 蜜罐刷新完成")
}

// monitorLoop 监控蜜罐文件完整性.
func (hm *HoneypotManager) monitorLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-hm.stopChan:
			return
		case <-ticker.C:
			hm.checkIntegrity()
		}
	}
}

// checkIntegrity 检查蜜罐文件完整性.
func (hm *HoneypotManager) checkIntegrity() {
	hm.mu.RLock()
	files := make(map[string]*HoneypotFile)
	for k, v := range hm.files {
		files[k] = v
	}
	hm.mu.RUnlock()

	for id, hp := range files {
		// 检查文件是否存在
		info, err := os.Stat(hp.Path)
		if os.IsNotExist(err) {
			log.Printf("[Honeypot] 蜜罐文件被删除: %s (ID: %s)", hp.Path, id)
			hm.RecordAccess(hp.Path, "delete", "unknown", 0, 0, "")
			continue
		}
		if err != nil {
			continue
		}

		// 检查大小是否变化
		if info.Size() != hp.SizeBytes {
			log.Printf("[Honeypot] 蜜罐文件大小变化: %s (原始: %d, 当前: %d)",
				hp.Path, hp.SizeBytes, info.Size())
			hm.RecordAccess(hp.Path, "write", "unknown", 0, 0, "")
		}

		// 检查哈希是否变化
		data, err := os.ReadFile(hp.Path)
		if err != nil {
			continue
		}
		hash := sha256.Sum256(data)
		currentHash := hex.EncodeToString(hash[:])
		if currentHash != hp.Hash {
			log.Printf("[Honeypot] 蜜罐文件内容变化: %s", hp.Path)
			hm.RecordAccess(hp.Path, "write", "unknown", 0, 0, "")
		}
	}
}

// GenerateRandomInt 生成安全的随机整数 [0, max).
func GenerateRandomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return mrand.Intn(max)
	}
	return int(n.Int64())
}

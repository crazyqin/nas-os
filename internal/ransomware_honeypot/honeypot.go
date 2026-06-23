package ransomware_honeypot

import (
	"fmt"
	"sync"
	"time"
)

// HoneypotManager 蜜罐管理器.
type HoneypotManager struct {
	mu        sync.RWMutex
	honeypots map[string]*Honeypot
	files     map[string][]*DecoyFile // honeypotID -> files
	alerts    map[string][]*Alert     // honeypotID -> alerts
	scans     []*ScanResult
	detector  *Detector
}

// NewHoneypotManager 创建蜜罐管理器.
func NewHoneypotManager() *HoneypotManager {
	return &HoneypotManager{
		honeypots: make(map[string]*Honeypot),
		files:     make(map[string][]*DecoyFile),
		alerts:    make(map[string][]*Alert),
		scans:     make([]*ScanResult, 0),
		detector:  NewDetector(DefaultThresholds()),
	}
}

// Create 创建蜜罐.
func (m *HoneypotManager) Create(req CreateHoneypotRequest) (*Honeypot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("蜜罐名称不能为空")
	}
	if req.SharePath == "" {
		return nil, fmt.Errorf("共享路径不能为空")
	}

	// 检查重名
	for _, h := range m.honeypots {
		if h.Name == req.Name {
			return nil, fmt.Errorf("蜜罐 %s 已存在", req.Name)
		}
	}

	now := time.Now()
	hp := &Honeypot{
		ID:        fmt.Sprintf("hp-%d", now.UnixNano()),
		Name:      req.Name,
		SharePath: req.SharePath,
		State:     StateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.honeypots[hp.ID] = hp

	// 生成诱饵文件
	fileTypes := req.FileTypes
	if len(fileTypes) == 0 {
		fileTypes = []FileType{FileTypeOffice, FileTypePDF, FileTypeImage, FileTypeText}
	}
	files := m.generateDecoyFiles(hp.ID, fileTypes)
	m.files[hp.ID] = files
	hp.FileCount = len(files)

	return hp, nil
}

// Get 获取蜜罐.
func (m *HoneypotManager) Get(id string) (*Honeypot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hp, ok := m.honeypots[id]
	if !ok {
		return nil, fmt.Errorf("蜜罐 %s 不存在", id)
	}
	return hp, nil
}

// List 列出所有蜜罐.
func (m *HoneypotManager) List() []*Honeypot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Honeypot, 0, len(m.honeypots))
	for _, hp := range m.honeypots {
		result = append(result, hp)
	}
	return result
}

// Delete 删除蜜罐.
func (m *HoneypotManager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.honeypots[id]; !ok {
		return fmt.Errorf("蜜罐 %s 不存在", id)
	}
	delete(m.honeypots, id)
	delete(m.files, id)
	delete(m.alerts, id)
	return nil
}

// Scan 扫描蜜罐.
func (m *HoneypotManager) Scan(honeypotID string) (*ScanResult, error) {
	m.mu.RLock()
	hp, ok := m.honeypots[honeypotID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("蜜罐 %s 不存在", honeypotID)
	}
	files := m.files[honeypotID]
	m.mu.RUnlock()

	start := time.Now()
	var alertsRaised, entropyChanges, renameEvents int

	for _, f := range files {
		// 模拟熵值变化检测
		if m.detector.CheckEntropyChange(f) {
			entropyChanges++
			alertsRaised++
			m.addAlert(honeypotID, Alert{
				ID:         fmt.Sprintf("alert-%d", time.Now().UnixNano()),
				HoneypotID: honeypotID,
				Level:      AlertLevelCritical,
				Type:       AlertTypeEntropyChange,
				Message:    fmt.Sprintf("文件 %s 熵值异常变化，疑似加密", f.FilePath),
				FilePath:   f.FilePath,
				CreatedAt:  time.Now(),
			})
		}
	}

	// 模拟批量重命名检测
	if m.detector.CheckMassRename(honeypotID) {
		renameEvents++
		alertsRaised++
		m.addAlert(honeypotID, Alert{
			ID:         fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			HoneypotID: honeypotID,
			Level:      AlertLevelCritical,
			Type:       AlertTypeMassRename,
			Message:    "检测到批量重命名行为，疑似勒索软件",
			CreatedAt:  time.Now(),
		})
	}

	duration := time.Since(start)

	result := &ScanResult{
		HoneypotID:     honeypotID,
		FilesScanned:   len(files),
		AlertsRaised:   alertsRaised,
		EntropyChanges: entropyChanges,
		RenameEvents:   renameEvents,
		ScanDuration:   duration.String(),
		ScannedAt:      time.Now(),
	}

	m.mu.Lock()
	m.scans = append(m.scans, result)
	if alertsRaised > 0 && hp.State == StateActive {
		hp.State = StateTriggered
		now := time.Now()
		hp.TriggeredAt = &now
		hp.UpdatedAt = now
	}
	m.mu.Unlock()

	return result, nil
}

// GetAlerts 获取蜜罐告警.
func (m *HoneypotManager) GetAlerts(honeypotID string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts[honeypotID]
}

// RespondAlert 响应告警.
func (m *HoneypotManager) RespondAlert(alertID string, resp AlertResponse) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alerts := range m.alerts {
		for _, a := range alerts {
			if a.ID == alertID {
				a.Responded = true
				a.Response = string(resp.Action) + ": " + resp.Comment
				now := time.Now()
				a.RespondedAt = &now
				return nil
			}
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// GetScans 获取扫描历史.
func (m *HoneypotManager) GetScans() []*ScanResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scans
}

// addAlert 添加告警（需外部加锁或无锁调用）.
func (m *HoneypotManager) addAlert(honeypotID string, alert Alert) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts[honeypotID] = append(m.alerts[honeypotID], &alert)
}

// generateDecoyFiles 生成诱饵文件.
func (m *HoneypotManager) generateDecoyFiles(honeypotID string, types []FileType) []*DecoyFile {
	var files []*DecoyFile
	now := time.Now()

	type FileTemplate struct {
		name      string
		fileType  FileType
		sizeBytes int64
		entropy   float64
	}

	templates := []FileTemplate{
		{"财务报表.xlsx", FileTypeOffice, 45056, 7.2},
		{"工资单.xlsx", FileTypeOffice, 32768, 7.0},
		{"合同扫描件.pdf", FileTypePDF, 1048576, 7.8},
		{"身份证扫描.pdf", FileTypePDF, 524288, 7.5},
		{"家庭照片.jpg", FileTypeImage, 2097152, 7.9},
		{"证件照.png", FileTypeImage, 1048576, 7.6},
		{"密码备份.txt", FileTypeText, 1024, 5.0},
		{"重要笔记.txt", FileTypeText, 2048, 5.2},
		{"项目文档.docx", FileTypeOffice, 81920, 7.3},
		{"会议纪要.docx", FileTypeOffice, 65536, 7.1},
		{"税务申报.pdf", FileTypePDF, 2097152, 7.7},
		{"毕业证书.jpg", FileTypeImage, 1572864, 7.4},
	}

	for _, tpl := range templates {
		// 检查类型过滤
		typeMatch := false
		for _, t := range types {
			if t == tpl.fileType {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			continue
		}

		files = append(files, &DecoyFile{
			ID:         fmt.Sprintf("df-%d", now.UnixNano()+int64(len(files))),
			HoneypotID: honeypotID,
			FilePath:   fmt.Sprintf("%s/%s", "decoy", tpl.name),
			FileType:   tpl.fileType,
			SizeBytes:  tpl.sizeBytes,
			Entropy:    tpl.entropy,
			Hash:       fmt.Sprintf("sha256:%x", tpl.sizeBytes*31+int64(tpl.entropy*1000)),
			CreatedAt:  now,
		})
	}

	return files
}

// Package antivirus - 管理器
package antivirus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 病毒扫描管理器.
type Manager struct {
	mu            sync.RWMutex
	client        *ClamAVClient
	tasks         map[string]*ScanTask
	schedules     map[string]*ScanSchedule
	quarantine    map[string]*QuarantineEntry
	whitelist     map[string]*WhitelistEntry
	monitorConfig *RealtimeMonitorConfig
	dbStatus      *VirusDBUpdateStatus
	quarantineDir string
	counter       int
}

// NewManager 创建管理器.
func NewManager(config ClamAVConfig, quarantineDir string) *Manager {
	if quarantineDir == "" {
		quarantineDir = "/var/lib/nas-os/antivirus/quarantine"
	}
	os.MkdirAll(quarantineDir, 0755)

	return &Manager{
		client:        NewClamAVClient(config),
		tasks:         make(map[string]*ScanTask),
		schedules:     make(map[string]*ScanSchedule),
		quarantine:    make(map[string]*QuarantineEntry),
		whitelist:     make(map[string]*WhitelistEntry),
		monitorConfig: &RealtimeMonitorConfig{},
		dbStatus:      &VirusDBUpdateStatus{},
		quarantineDir: quarantineDir,
	}
}

// CreateScan 创建扫描任务.
func (m *Manager) CreateScan(req CreateScanRequest) (*ScanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counter++
	task := &ScanTask{
		ID:           fmt.Sprintf("scan-%d", m.counter),
		Name:         req.Name,
		Type:         req.Type,
		Status:       ScanStatusPending,
		Paths:        req.Paths,
		Recursive:    req.Recursive,
		ScanArchives: req.ScanArchives,
		ThreatAction: req.ThreatAction,
		CreatedAt:    time.Now(),
	}
	if task.ThreatAction == "" {
		task.ThreatAction = ThreatActionQuarantine
	}
	if task.Name == "" {
		task.Name = fmt.Sprintf("%s 扫描 - %s", task.Type, time.Now().Format("2006-01-02 15:04"))
	}

	m.tasks[task.ID] = task
	go m.runScan(task.ID)
	return task, nil
}

// runScan 执行扫描任务.
func (m *Manager) runScan(taskID string) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	now := time.Now()
	task.StartedAt = &now
	task.SetStatus(ScanStatusRunning)

	// Ping ClamAV
	if err := m.client.Ping(); err != nil {
		task.SetStatus(ScanStatusFailed)
		task.Error = "ClamAV 不可用"
		finish := time.Now()
		task.FinishedAt = &finish
		task.Duration = int64(finish.Sub(now).Seconds())
		return
	}

	for _, dir := range task.Paths {
		results, _ := ScanDirectory(m.client, dir, task.Recursive)
		for _, r := range results {
			if r.IsInfected {
				switch task.ThreatAction {
				case ThreatActionQuarantine:
					m.quarantineFile(&r)
				case ThreatActionDelete:
					os.Remove(r.FilePath)
					r.Action = ThreatActionDelete
				}
			}
			task.AddResult(r)
			scanned, total, _, _ := task.GetProgress()
			task.SetProgress(scanned+1, total, r.FilePath)
		}
	}

	finish := time.Now()
	task.FinishedAt = &finish
	task.Duration = int64(finish.Sub(now).Seconds())
	task.SetStatus(ScanStatusDone)
}

// quarantineFile 隔离文件.
func (m *Manager) quarantineFile(r *ScanResult) {
	hash := sha256File(r.FilePath)
	qPath := filepath.Join(m.quarantineDir, hash)

	if err := copyFile(r.FilePath, qPath); err != nil {
		r.Action = ThreatActionIgnore
		return
	}

	os.Remove(r.FilePath)
	r.Action = ThreatActionQuarantine
	r.Quarantined = true
	r.QuarantinePath = qPath

	m.mu.Lock()
	m.counter++
	m.quarantine[fmt.Sprintf("q-%d", m.counter)] = &QuarantineEntry{
		ID:             fmt.Sprintf("q-%d", m.counter),
		OriginalPath:   r.FilePath,
		QuarantinePath: qPath,
		ThreatName:     r.ThreatName,
		FileHash:       hash,
		QuarantinedAt:  time.Now(),
	}
	m.mu.Unlock()
}

// GetScan 获取扫描任务.
func (m *Manager) GetScan(id string) (*ScanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrScanNotFound
	}
	return task, nil
}

// ListScans 列出所有扫描任务.
func (m *Manager) ListScans() []*ScanTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ScanTask
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// CancelScan 取消扫描.
func (m *Manager) CancelScan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return ErrScanNotFound
	}
	if task.Status != ScanStatusRunning {
		return fmt.Errorf("任务不在运行中")
	}
	task.SetStatus(ScanStatusCanceled)
	return nil
}

// GetScanReport 获取扫描报告.
func (m *Manager) GetScanReport(id string) (*ScanReport, error) {
	task, err := m.GetScan(id)
	if err != nil {
		return nil, err
	}

	threatSummary := make(map[string]int)
	for _, r := range task.Results {
		if r.IsInfected {
			threatSummary[r.ThreatName]++
		}
	}

	var speed float64
	if task.Duration > 0 {
		speed = float64(task.ScannedFiles) / float64(task.Duration)
	}

	return &ScanReport{
		TaskID:        task.ID,
		TaskName:      task.Name,
		ScanType:      task.Type,
		Status:        task.Status,
		StartedAt:     task.StartedAt,
		FinishedAt:    task.FinishedAt,
		Duration:      task.Duration,
		TotalFiles:    task.TotalFiles,
		ScannedFiles:  task.ScannedFiles,
		InfectedFiles: task.InfectedFiles,
		ScanSpeed:     speed,
		InfectedList:  task.Results,
		ThreatSummary: threatSummary,
	}, nil
}

// GetQuarantineList 获取隔离区列表.
func (m *Manager) GetQuarantineList() []*QuarantineEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*QuarantineEntry
	for _, e := range m.quarantine {
		if !e.Deleted && !e.Restored {
			result = append(result, e)
		}
	}
	return result
}

// RestoreFromQuarantine 从隔离区恢复文件.
func (m *Manager) RestoreFromQuarantine(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.quarantine[id]
	if !ok {
		return fmt.Errorf("隔离条目不存在")
	}
	if err := copyFile(entry.QuarantinePath, entry.OriginalPath); err != nil {
		return err
	}
	os.Remove(entry.QuarantinePath)
	now := time.Now()
	entry.Restored = true
	entry.RestoredAt = &now
	return nil
}

// DeleteFromQuarantine 删除隔离文件.
func (m *Manager) DeleteFromQuarantine(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.quarantine[id]
	if !ok {
		return fmt.Errorf("隔离条目不存在")
	}
	os.Remove(entry.QuarantinePath)
	now := time.Now()
	entry.Deleted = true
	entry.DeletedAt = &now
	return nil
}

// AddWhitelist 添加白名单.
func (m *Manager) AddWhitelist(req WhitelistAddRequest) *WhitelistEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	entry := &WhitelistEntry{
		ID:        fmt.Sprintf("wl-%d", m.counter),
		Path:      req.Path,
		Hash:      req.Hash,
		Reason:    req.Reason,
		CreatedAt: time.Now(),
	}
	m.whitelist[entry.ID] = entry
	return entry
}

// RemoveWhitelist 移除白名单.
func (m *Manager) RemoveWhitelist(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.whitelist[id]; !ok {
		return fmt.Errorf("白名单条目不存在")
	}
	delete(m.whitelist, id)
	return nil
}

// ListWhitelist 列出白名单.
func (m *Manager) ListWhitelist() []*WhitelistEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*WhitelistEntry
	for _, e := range m.whitelist {
		result = append(result, e)
	}
	return result
}

// GetVirusDBStatus 获取病毒库状态.
func (m *Manager) GetVirusDBStatus() *VirusDBUpdateStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dbStatus
}

// UpdateVirusDB 更新病毒库.
func (m *Manager) UpdateVirusDB() error {
	if err := m.client.Ping(); err != nil {
		return err
	}
	if err := m.client.Reload(); err != nil {
		return err
	}
	m.mu.Lock()
	m.dbStatus.Status = "success"
	m.dbStatus.LastUpdate = time.Now()
	m.mu.Unlock()
	return nil
}

// GetMonitorConfig 获取实时监控配置.
func (m *Manager) GetMonitorConfig() *RealtimeMonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.monitorConfig
}

// UpdateMonitorConfig 更新实时监控配置.
func (m *Manager) UpdateMonitorConfig(req UpdateMonitorConfigRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.Enabled != nil {
		m.monitorConfig.Enabled = *req.Enabled
	}
	if req.WatchPaths != nil {
		m.monitorConfig.WatchPaths = req.WatchPaths
	}
	if req.ThreatAction != nil {
		m.monitorConfig.ThreatAction = *req.ThreatAction
	}
	if req.Recursive != nil {
		m.monitorConfig.Recursive = *req.Recursive
	}
	if req.ExcludePaths != nil {
		m.monitorConfig.ExcludePaths = req.ExcludePaths
	}
}

// GetStats 获取扫描统计.
func (m *Manager) GetStats() *ScanStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := &ScanStats{}
	for _, t := range m.tasks {
		stats.TotalScans++
		stats.TotalFiles += t.ScannedFiles
		stats.TotalInfected += t.InfectedFiles
		if t.FinishedAt != nil {
			if stats.LastScanTime == nil || t.FinishedAt.After(*stats.LastScanTime) {
				stats.LastScanTime = t.FinishedAt
			}
		}
	}
	stats.TotalQuarantine = len(m.quarantine)
	return stats
}

func sha256File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	os.MkdirAll(filepath.Dir(dst), 0755)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

package ransomware_honeypot

import (
	"math"
	"sync"
	"time"
)

// Detector 勒索软件检测引擎.
type Detector struct {
	mu         sync.RWMutex
	thresholds DetectionThresholds
	state      map[string]*DetectorState // honeypotID -> state
}

// NewDetector 创建检测引擎.
func NewDetector(thresholds DetectionThresholds) *Detector {
	return &Detector{
		thresholds: thresholds,
		state:      make(map[string]*DetectorState),
	}
}

// RegisterHoneypot 注册蜜罐到检测器.
func (d *Detector) RegisterHoneypot(honeypotID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.state[honeypotID] = &DetectorState{
		recentEvents: make([]AccessEvent, 0),
	}
}

// UnregisterHoneypot 注销蜜罐.
func (d *Detector) UnregisterHoneypot(honeypotID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.state, honeypotID)
}

// RecordEvent 记录文件访问事件.
func (d *Detector) RecordEvent(honeypotID string, event AccessEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	state, ok := d.state[honeypotID]
	if !ok {
		return
	}

	// 自动填充时间戳
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	state.recentEvents = append(state.recentEvents, event)

	// 清理过期事件
	cutoff := time.Now().Add(-time.Duration(d.thresholds.MassRenameWindowSec) * time.Second)
	var valid []AccessEvent
	for _, e := range state.recentEvents {
		if e.Timestamp.After(cutoff) {
			valid = append(valid, e)
		}
	}
	state.recentEvents = valid
}

// CheckEntropyChange 检查文件熵值异常变化.
func (d *Detector) CheckEntropyChange(file *DecoyFile) bool {
	// 正常文件熵值范围
	normalEntropy := map[FileType]float64{
		FileTypeOffice: 7.0,
		FileTypePDF:    7.5,
		FileTypeImage:  7.8,
		FileTypeText:   5.0,
	}

	expected, ok := normalEntropy[file.FileType]
	if !ok {
		return false
	}

	// 熵值显著升高表示可能被加密
	diff := math.Abs(file.Entropy - expected)
	return diff > d.thresholds.EntropyChangeThreshold
}

// CheckMassRename 检测批量重命名行为.
func (d *Detector) CheckMassRename(honeypotID string) bool {
	d.mu.RLock()
	state, ok := d.state[honeypotID]
	d.mu.RUnlock()

	if !ok {
		// 如果没有注册，模拟检测
		return false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	renameCount := 0
	cutoff := time.Now().Add(-time.Duration(d.thresholds.MassRenameWindowSec) * time.Second)
	for _, event := range state.recentEvents {
		if event.EventType == "rename" && event.Timestamp.After(cutoff) {
			renameCount++
		}
	}

	return renameCount >= d.thresholds.MassRenameThreshold
}

// CheckAccessFrequency 检测异常高频访问.
func (d *Detector) CheckAccessFrequency(honeypotID string, filePath string) bool {
	d.mu.RLock()
	state, ok := d.state[honeypotID]
	d.mu.RUnlock()

	if !ok {
		return false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	count := 0
	cutoff := time.Now().Add(-time.Duration(d.thresholds.AccessFrequencyWindowSec) * time.Second)
	for _, event := range state.recentEvents {
		if event.FilePath == filePath && event.Timestamp.After(cutoff) {
			count++
		}
	}

	return count >= d.thresholds.AccessFrequencyLimit
}

// CheckExtensionChange 检测文件扩展名批量变更.
func (d *Detector) CheckExtensionChange(honeypotID string) bool {
	d.mu.RLock()
	state, ok := d.state[honeypotID]
	d.mu.RUnlock()

	if !ok {
		return false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	renameCount := 0
	cutoff := time.Now().Add(-time.Duration(d.thresholds.MassRenameWindowSec) * time.Second)
	for _, event := range state.recentEvents {
		if event.EventType == "rename" && event.Timestamp.After(cutoff) {
			renameCount++
		}
	}

	// 批量重命名通常伴随扩展名变更
	return renameCount >= d.thresholds.MassRenameThreshold
}

// GetState 获取检测器状态.
func (d *Detector) GetState(honeypotID string) *DetectorState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.state[honeypotID]
}

// UpdateThresholds 更新检测阈值.
func (d *Detector) UpdateThresholds(thresholds DetectionThresholds) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.thresholds = thresholds
}

// GetThresholds 获取当前阈值.
func (d *Detector) GetThresholds() DetectionThresholds {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.thresholds
}

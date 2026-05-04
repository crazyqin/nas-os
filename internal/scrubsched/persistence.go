// Package scrubsched 提供智能Scrub调度功能
// persistence.go - 状态持久化，支持进程重启后恢复策略和进度
package scrubsched

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 持久化配置 ==========

// PersistConfig 持久化配置.
type PersistConfig struct {
	DataDir       string        // 数据目录
	AutoSave      bool          // 是否自动保存
	SaveInterval  time.Duration // 自动保存间隔
}

// DefaultPersistConfig 默认持久化配置.
func DefaultPersistConfig(dataDir string) PersistConfig {
	return PersistConfig{
		DataDir:      dataDir,
		AutoSave:     true,
		SaveInterval: 5 * time.Minute,
	}
}

// ========== 持久化数据结构 ==========

// PersistData 持久化数据快照.
type PersistData struct {
	Version   int                       `json:"version"`    // 数据版本
	Policies  map[string]*Policy        `json:"policies"`   // 策略快照
	History   []*ScrubRecord            `json:"history"`    // 执行历史
	SavedAt   time.Time                 `json:"saved_at"`   // 保存时间
}

// ========== 持久化管理器 ==========

// Persister 持久化管理器.
type Persister struct {
	mu       sync.Mutex
	config   PersistConfig
	manager  *Manager
	stopCh   chan struct{}
	running  bool
}

// NewPersister 创建持久化管理器.
func NewPersister(cfg PersistConfig, mgr *Manager) *Persister {
	return &Persister{
		config:  cfg,
		manager: mgr,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动自动持久化.
func (p *Persister) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	// 加载历史数据
	if err := p.Load(); err != nil {
		log.Printf("[scrubsched] 加载持久化数据失败: %v（首次启动正常）", err)
	}

	if !p.config.AutoSave {
		return
	}

	go p.autoSaveLoop()
	log.Printf("[scrubsched] 持久化已启动，目录: %s，间隔: %s", p.config.DataDir, p.config.SaveInterval)
}

// Stop 停止持久化.
func (p *Persister) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}
	p.running = false
	close(p.stopCh)

	// 退出前保存一次
	if err := p.Save(); err != nil {
		log.Printf("[scrubsched] 退出保存失败: %v", err)
	} else {
		log.Println("[scrubsched] 退出前数据已保存")
	}
}

// Save 保存当前状态到磁盘.
func (p *Persister) Save() error {
	p.manager.mu.RLock()
	data := PersistData{
		Version: 1,
		Policies: make(map[string]*Policy, len(p.manager.policies)),
		History:  make([]*ScrubRecord, len(p.manager.history)),
		SavedAt:  time.Now(),
	}
	// 深拷贝策略
	for id, pol := range p.manager.policies {
		cp := *pol
		data.Policies[id] = &cp
	}
	// 深拷贝历史
	copy(data.History, p.manager.history)
	p.manager.mu.RUnlock()

	// 序列化
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(p.config.DataDir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入临时文件后原子替换
	tmpPath := filepath.Join(p.config.DataDir, "scrubsched.json.tmp")
	finalPath := filepath.Join(p.config.DataDir, "scrubsched.json")

	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("原子替换失败: %w", err)
	}

	return nil
}

// Load 从磁盘加载状态.
func (p *Persister) Load() error {
	path := filepath.Join(p.config.DataDir, "scrubsched.json")

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 首次启动，无数据
		}
		return fmt.Errorf("读取文件失败: %w", err)
	}

	var data PersistData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("反序列化失败: %w", err)
	}

	if data.Version != 1 {
		return fmt.Errorf("不支持的数据版本: %d", data.Version)
	}

	// 恢复策略
	p.manager.mu.Lock()
	if data.Policies != nil {
		for id, pol := range data.Policies {
			// 重新计算下次执行时间（避免加载过期的 NextRun）
			nextRun := p.manager.calculateNextRun(pol)
			pol.NextRun = &nextRun
			p.manager.policies[id] = pol
		}
	}
	// 恢复历史（保留最近 500 条）
	if data.History != nil {
		start := 0
		if len(data.History) > 500 {
			start = len(data.History) - 500
		}
		p.manager.history = data.History[start:]
	}
	p.manager.mu.Unlock()

	log.Printf("[scrubsched] 已恢复 %d 个策略，%d 条历史记录（保存于 %s）",
		len(data.Policies), len(data.History),
		data.SavedAt.Format("2006-01-02 15:04:05"))

	return nil
}

// autoSaveLoop 自动保存循环.
func (p *Persister) autoSaveLoop() {
	ticker := time.NewTicker(p.config.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			if err := p.Save(); err != nil {
				log.Printf("[scrubsched] 自动保存失败: %v", err)
			}
		}
	}
}

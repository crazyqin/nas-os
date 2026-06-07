package recyclecleaner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Rule defines a recycle bin cleaning rule.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	SharePath   string `json:"share_path"`   // path to share
	RecyclePath string `json:"recycle_path"` // path to .recycle bin
	MaxAgeDays  int    `json:"max_age_days"` // delete items older than this
	MaxSizeMB   int    `json:"max_size_mb"`  // max recycle bin size
	Enabled     bool   `json:"enabled"`
}

// Stats holds cleaning statistics.
type Stats struct {
	FilesDeleted int           `json:"files_deleted"`
	FoldersDel   int           `json:"folders_deleted"`
	BytesFreed   int64         `json:"bytes_freed"`
	LastRunTime  time.Time     `json:"last_run_time"`
	LastDuration time.Duration `json:"last_duration"`
}

// Manager manages recycle bin cleaning.
type Manager struct {
	mu     sync.RWMutex
	rules  map[string]*Rule
	stats  map[string]*Stats
	stopCh chan struct{}
}

// NewManager creates a new recycle cleaner manager.
func NewManager() *Manager {
	return &Manager{
		rules:  make(map[string]*Rule),
		stats:  make(map[string]*Stats),
		stopCh: make(chan struct{}),
	}
}

// Start begins periodic cleaning.
func (m *Manager) Start() {
	go m.cleanLoop()
	log.Println("✅ 回收站自动清理已启动")
}

// Stop halts cleaning.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// AddRule adds a cleaning rule.
func (m *Manager) AddRule(rule Rule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rule.Enabled = true
	m.rules[rule.ID] = &rule
	m.stats[rule.ID] = &Stats{}
	log.Printf("回收站清理规则已添加: %s -> %s", rule.Name, rule.RecyclePath)
}

// RemoveRule removes a cleaning rule.
func (m *Manager) RemoveRule(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rules, id)
	delete(m.stats, id)
}

// UpdateRule updates a cleaning rule.
func (m *Manager) UpdateRule(rule Rule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.rules[rule.ID]; !ok {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}
	m.rules[rule.ID] = &rule
	return nil
}

// ListRules returns all cleaning rules.
func (m *Manager) ListRules() []Rule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Rule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, *r)
	}
	return result
}

// GetStats returns stats for a rule.
func (m *Manager) GetStats(id string) *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if s, ok := m.stats[id]; ok {
		return s
	}
	return nil
}

// RunCleanNow triggers immediate cleaning for a specific rule.
func (m *Manager) RunCleanNow(id string) (*Stats, error) {
	m.mu.RLock()
	rule, ok := m.rules[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return m.cleanRule(rule)
}

func (m *Manager) cleanLoop() {
	ticker := time.NewTicker(1 * time.Hour) // check every hour
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.cleanAll()
		}
	}
}

func (m *Manager) cleanAll() {
	m.mu.RLock()
	rules := make([]*Rule, 0, len(m.rules))
	for _, r := range m.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	m.mu.RUnlock()

	for _, rule := range rules {
		if _, err := m.cleanRule(rule); err != nil {
			log.Printf("回收站清理失败 [%s]: %v", rule.Name, err)
		}
	}
}

func (m *Manager) cleanRule(rule *Rule) (*Stats, error) {
	start := time.Now()
	stats := &Stats{}

	cutoff := time.Now().AddDate(0, 0, -rule.MaxAgeDays)

	err := filepath.Walk(rule.RecyclePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		// Check age
		if info.ModTime().Before(cutoff) {
			if info.IsDir() {
				if err := os.RemoveAll(path); err == nil {
					stats.FoldersDel++
				}
			} else {
				size := info.Size()
				if err := os.Remove(path); err == nil {
					stats.FilesDeleted++
					stats.BytesFreed += size
				}
			}
		}
		return nil
	})

	stats.LastRunTime = time.Now()
	stats.LastDuration = time.Since(start)

	m.mu.Lock()
	m.stats[rule.ID] = stats
	m.mu.Unlock()

	if err != nil && !os.IsNotExist(err) {
		return stats, err
	}

	log.Printf("♻️ 回收站清理完成 [%s]: 删除 %d 文件, %d 目录, 释放 %d MB",
		rule.Name, stats.FilesDeleted, stats.FoldersDel, stats.BytesFreed/1024/1024)
	return stats, nil
}

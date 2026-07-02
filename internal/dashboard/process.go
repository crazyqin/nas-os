package dashboard

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ProcessInfo 进程信息.
type ProcessInfo struct {
	PID        int       `json:"pid"`
	Name       string    `json:"name"`
	User       string    `json:"user"`
	CPUPercent float64   `json:"cpuPercent"`
	MemPercent float64   `json:"memPercent"`
	MemRSS     uint64    `json:"memRss"`
	Status     string    `json:"status"` // running/sleeping/stopped/zombie
	CreateTime time.Time `json:"createTime"`
	CmdLine    string    `json:"cmdLine"`
	NumThreads int       `json:"numThreads"`
	ParentPID  int       `json:"parentPid"`
}

// ProcessManager 进程监控管理器.
type ProcessManager struct {
	mu        sync.RWMutex
	processes map[int]*ProcessInfo
}

// NewProcessManager 创建进程管理器.
func NewProcessManager() *ProcessManager {
	return &ProcessManager{
		processes: make(map[int]*ProcessInfo),
	}
}

// Update 更新进程列表.
func (pm *ProcessManager) Update(procs []*ProcessInfo) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.processes = make(map[int]*ProcessInfo, len(procs))
	for _, p := range procs {
		pm.processes[p.PID] = p
	}
}

// Get 获取指定进程.
func (pm *ProcessManager) Get(pid int) (*ProcessInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.processes[pid]
	if !ok {
		return nil, fmt.Errorf("进程 %d 不存在", pid)
	}
	return p, nil
}

// List 列出所有进程.
func (pm *ProcessManager) List() []*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*ProcessInfo, 0, len(pm.processes))
	for _, p := range pm.processes {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CPUPercent > result[j].CPUPercent
	})
	return result
}

// ListByStatus 按状态过滤.
func (pm *ProcessManager) ListByStatus(status string) []*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*ProcessInfo
	for _, p := range pm.processes {
		if p.Status == status {
			result = append(result, p)
		}
	}
	return result
}

// TopCPU 返回 CPU 占用最高的 N 个进程.
func (pm *ProcessManager) TopCPU(n int) []*ProcessInfo {
	all := pm.List()
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

// TopMemory 返回内存占用最高的 N 个进程.
func (pm *ProcessManager) TopMemory(n int) []*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]*ProcessInfo, 0, len(pm.processes))
	for _, p := range pm.processes {
		result = append(result, p)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].MemPercent > result[j].MemPercent
	})
	if n > len(result) {
		n = len(result)
	}
	return result[:n]
}

// Search 按名称搜索进程.
func (pm *ProcessManager) Search(name string) []*ProcessInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []*ProcessInfo
	for _, p := range pm.processes {
		if containsIgnoreCase(p.Name, name) || containsIgnoreCase(p.CmdLine, name) {
			result = append(result, p)
		}
	}
	return result
}

// Count 返回进程数.
func (pm *ProcessManager) Count() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.processes)
}

func containsIgnoreCase(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			sc := s[i+j]
			tc := sub[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

package dashboard

import (
	"fmt"
	"sync"
	"time"
)

// ServiceState 服务状态.
type ServiceState string

const (
	ServiceRunning  ServiceState = "running"
	ServiceStopped  ServiceState = "stopped"
	ServiceFailed   ServiceState = "failed"
	ServiceStarting ServiceState = "starting"
	ServiceUnknown  ServiceState = "unknown"
)

// ServiceInfo 系统服务信息.
type ServiceInfo struct {
	Name        string       `json:"name"`
	State       ServiceState `json:"state"`
	Description string       `json:"description"`
	PID         int          `json:"pid,omitempty"`
	MemoryMB    uint64       `json:"memoryMb,omitempty"`
	Uptime      int64        `json:"uptimeSeconds,omitempty"`
	Enabled     bool         `json:"enabled"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	Ports       []int        `json:"ports,omitempty"`
}

// ServiceManager 系统服务监控管理器.
type ServiceManager struct {
	mu       sync.RWMutex
	services map[string]*ServiceInfo
}

// NewServiceManager 创建服务管理器.
func NewServiceManager() *ServiceManager {
	return &ServiceManager{
		services: make(map[string]*ServiceInfo),
	}
}

// Register 注册服务.
func (sm *ServiceManager) Register(svc *ServiceInfo) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	svc.UpdatedAt = time.Now()
	sm.services[svc.Name] = svc
}

// Update 更新服务状态.
func (sm *ServiceManager) Update(name string, state ServiceState) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	svc, ok := sm.services[name]
	if !ok {
		return fmt.Errorf("服务 %s 未注册", name)
	}
	svc.State = state
	svc.UpdatedAt = time.Now()
	return nil
}

// Get 获取服务信息.
func (sm *ServiceManager) Get(name string) (*ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	svc, ok := sm.services[name]
	if !ok {
		return nil, fmt.Errorf("服务 %s 不存在", name)
	}
	return svc, nil
}

// List 列出所有服务.
func (sm *ServiceManager) List() []*ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]*ServiceInfo, 0, len(sm.services))
	for _, svc := range sm.services {
		result = append(result, svc)
	}
	return result
}

// ListByState 按状态过滤.
func (sm *ServiceManager) ListByState(state ServiceState) []*ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	var result []*ServiceInfo
	for _, svc := range sm.services {
		if svc.State == state {
			result = append(result, svc)
		}
	}
	return result
}

// Unregister 注销服务.
func (sm *ServiceManager) Unregister(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.services, name)
}

// Count 返回注册的服务数.
func (sm *ServiceManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.services)
}

// Summary 返回服务状态汇总.
func (sm *ServiceManager) Summary() ServiceSummary {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	summary := ServiceSummary{Total: len(sm.services)}
	for _, svc := range sm.services {
		switch svc.State {
		case ServiceRunning:
			summary.Running++
		case ServiceStopped:
			summary.Stopped++
		case ServiceFailed:
			summary.Failed++
		case ServiceStarting:
			summary.Starting++
		default:
			summary.Unknown++
		}
	}
	return summary
}

// ServiceSummary 服务状态汇总.
type ServiceSummary struct {
	Total    int `json:"total"`
	Running  int `json:"running"`
	Stopped  int `json:"stopped"`
	Failed   int `json:"failed"`
	Starting int `json:"starting"`
	Unknown  int `json:"unknown"`
}

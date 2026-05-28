// Package distScheduler 资源分配器，支持CPU/内存/IO感知调度
package distScheduler

import (
	"fmt"
	"sort"
	"sync"

	"go.uber.org/zap"
)

// Allocator 资源分配器
type Allocator struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *Config
}

// NewAllocator 创建资源分配器
func NewAllocator(logger *zap.Logger, config *Config) *Allocator {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}
	return &Allocator{
		logger: logger,
		config: config,
	}
}

// AllocateNode 根据资源需求分配最佳节点
func (a *Allocator) AllocateNode(nodes map[string]*Node, req *ResourceReq) (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if req == nil {
		req = &ResourceReq{CPU: 1, Memory: 512}
	}

	candidates := a.filterEligible(nodes, req)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no eligible node available for resource requirements (cpu=%.1f, mem=%dMB)", req.CPU, req.Memory)
	}

	// 按资源可用率排序
	sort.Slice(candidates, func(i, j int) bool {
		return a.scoreNode(candidates[i], req) > a.scoreNode(candidates[j], req)
	})

	best := candidates[0]
	a.logger.Debug("allocated node",
		zap.String("node_id", best.ID),
		zap.Float64("score", a.scoreNode(best, req)),
	)
	return best.ID, nil
}

// filterEligible 过滤满足资源需求的节点
func (a *Allocator) filterEligible(nodes map[string]*Node, req *ResourceReq) []*Node {
	result := make([]*Node, 0)
	for _, node := range nodes {
		if node.Status != NodeStatusOnline {
			continue
		}
		if node.Resources == nil {
			// 无资源信息的节点只能接受无需求的任务
			if req.CPU <= 0 && req.Memory <= 0 {
				result = append(result, node)
			}
			continue
		}
		if a.hasCapacity(node, req) {
			result = append(result, node)
		}
	}
	return result
}

// hasCapacity 检查节点是否有足够容量
func (a *Allocator) hasCapacity(node *Node, req *ResourceReq) bool {
	res := node.Resources

	if req.CPU > 0 && res.CPU.Available < req.CPU {
		return false
	}
	if req.Memory > 0 && res.Memory.Available < float64(req.Memory) {
		return false
	}
	if req.GPU > 0 && res.GPU.Available < float64(req.GPU) {
		return false
	}
	if req.Disk > 0 && res.Disk.Available < float64(req.Disk) {
		return false
	}
	return true
}

// scoreNode 计算节点评分
func (a *Allocator) scoreNode(node *Node, req *ResourceReq) float64 {
	if node.Resources == nil {
		return 0.5
	}

	res := node.Resources
	score := 0.0
	weights := 0.0

	// CPU 可用率
	if res.CPU.Total > 0 {
		cpuAvail := res.CPU.Available / res.CPU.Total
		score += cpuAvail * 0.4
		weights += 0.4
	}

	// 内存可用率
	if res.Memory.Total > 0 {
		memAvail := res.Memory.Available / res.Memory.Total
		score += memAvail * 0.3
		weights += 0.3
	}

	// GPU 可用率
	if req.GPU > 0 && res.GPU.Total > 0 {
		gpuAvail := res.GPU.Available / res.GPU.Total
		score += gpuAvail * 0.2
		weights += 0.2
	}

	// 任务数（越少越好）
	if node.TaskCount < a.config.MaxConcurrent {
		loadScore := 1.0 - float64(node.TaskCount)/float64(a.config.MaxConcurrent)
		score += loadScore * 0.1
		weights += 0.1
	}

	if weights > 0 {
		score /= weights
	}
	return score
}

// ReserveResources 在节点上预留资源
func (a *Allocator) ReserveResources(node *Node, req *ResourceReq) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if node.Resources == nil {
		return nil
	}

	if !a.hasCapacity(node, req) {
		return fmt.Errorf("insufficient resources on node %s", node.ID)
	}

	node.Resources.CPU.Used += req.CPU
	node.Resources.CPU.Available -= req.CPU
	node.Resources.Memory.Used += float64(req.Memory)
	node.Resources.Memory.Available -= float64(req.Memory)
	if req.GPU > 0 {
		node.Resources.GPU.Used += float64(req.GPU)
		node.Resources.GPU.Available -= float64(req.GPU)
	}
	if req.Disk > 0 {
		node.Resources.Disk.Used += float64(req.Disk)
		node.Resources.Disk.Available -= float64(req.Disk)
	}

	return nil
}

// ReleaseResources 释放节点上的资源
func (a *Allocator) ReleaseResources(node *Node, req *ResourceReq) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if node.Resources == nil || req == nil {
		return
	}

	node.Resources.CPU.Used -= req.CPU
	node.Resources.CPU.Available += req.CPU
	node.Resources.Memory.Used -= float64(req.Memory)
	node.Resources.Memory.Available += float64(req.Memory)
	if req.GPU > 0 {
		node.Resources.GPU.Used -= float64(req.GPU)
		node.Resources.GPU.Available += float64(req.GPU)
	}
	if req.Disk > 0 {
		node.Resources.Disk.Used -= float64(req.Disk)
		node.Resources.Disk.Available += float64(req.Disk)
	}
}

// GetResourceUtilization 获取节点资源利用率
func (a *Allocator) GetResourceUtilization(node *Node) map[string]float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()

	util := make(map[string]float64)
	if node.Resources == nil {
		return util
	}

	res := node.Resources
	if res.CPU.Total > 0 {
		util["cpu"] = res.CPU.Used / res.CPU.Total * 100
	}
	if res.Memory.Total > 0 {
		util["memory"] = res.Memory.Used / res.Memory.Total * 100
	}
	if res.GPU.Total > 0 {
		util["gpu"] = res.GPU.Used / res.GPU.Total * 100
	}
	if res.Disk.Total > 0 {
		util["disk"] = res.Disk.Used / res.Disk.Total * 100
	}
	return util
}

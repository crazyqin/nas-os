// Package storage 提供唤醒优先级队列实现
package storage

import (
	"container/heap"
	"sync"
)

// ============== 优先级队列实现 ==============

// WakePriorityQueue 唤醒优先级队列
// 高优先级任务（备份/同步）优先处理
type WakePriorityQueue struct {
	items []*WakeRequest
	mu    sync.Mutex
}

// NewWakePriorityQueue 创建新的唤醒优先级队列
func NewWakePriorityQueue() *WakePriorityQueue {
	pq := &WakePriorityQueue{
		items: make([]*WakeRequest, 0),
	}
	heap.Init(pq)
	return pq
}

// Push 添加唤醒请求到队列
func (pq *WakePriorityQueue) Push(req *WakeRequest) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	heap.Push(pq, req)
}

// Pop 从队列取出最高优先级请求
func (pq *WakePriorityQueue) Pop() *WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.Len() == 0 {
		return nil
	}

	item := heap.Pop(pq).(*WakeRequest)
	return item
}

// Peek 查看队首元素但不移除
func (pq *WakePriorityQueue) Peek() *WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.Len() == 0 {
		return nil
	}

	return pq.items[0]
}

// Len 返回队列长度
func (pq *WakePriorityQueue) Len() int {
	return len(pq.items)
}

// Clear 清空队列
func (pq *WakePriorityQueue) Clear() {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	pq.items = make([]*WakeRequest, 0)
}

// RemoveByTaskID 根据任务ID移除请求
func (pq *WakePriorityQueue) RemoveByTaskID(taskID string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	for i, item := range pq.items {
		if item.TaskID == taskID {
			heap.Remove(pq, i)
			return true
		}
	}
	return false
}

// RemoveByDevice 移除指定设备的所有请求
func (pq *WakePriorityQueue) RemoveByDevice(device string) int {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	removed := 0
	newItems := make([]*WakeRequest, 0)

	for _, item := range pq.items {
		if item.Device != device {
			newItems = append(newItems, item)
		} else {
			removed++
		}
	}

	pq.items = newItems
	heap.Init(pq)

	return removed
}

// GetByDevice 获取指定设备的请求列表
func (pq *WakePriorityQueue) GetByDevice(device string) []*WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	result := make([]*WakeRequest, 0)
	for _, item := range pq.items {
		if item.Device == device {
			result = append(result, item)
		}
	}
	return result
}

// GetByPriority 获取指定优先级的请求列表
func (pq *WakePriorityQueue) GetByPriority(priority WakePriority) []*WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	result := make([]*WakeRequest, 0)
	for _, item := range pq.items {
		if item.Priority == priority {
			result = append(result, item)
		}
	}
	return result
}

// ============== heap.Interface 实现 ==============

// Len 实现 heap.Interface
func (pq *WakePriorityQueue) Len() int {
	return len(pq.items)
}

// Less 实现 heap.Interface
// 优先级高的排在前面，优先级相同时按请求时间排序（先请求的先处理）
func (pq *WakePriorityQueue) Less(i, j int) bool {
	// 注意：WakePriority 数值越大优先级越高
	// 我们希望高优先级排在前面，所以使用 >
	if pq.items[i].Priority != pq.items[j].Priority {
		return pq.items[i].Priority > pq.items[j].Priority
	}
	// 优先级相同时，先请求的排在前面
	return pq.items[i].RequestedAt.Before(pq.items[j].RequestedAt)
}

// Swap 实现 heap.Interface
func (pq *WakePriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
}

// Push 实现 heap.Interface（注意：这里是添加 interface{} 类型）
func (pq *WakePriorityQueue) Push(x interface{}) {
	item := x.(*WakeRequest)
	pq.items = append(pq.items, item)
}

// Pop 实现 heap.Interface（注意：这里返回 interface{} 类型）
func (pq *WakePriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // 避免内存泄漏
	pq.items = old[0 : n-1]
	return item
}

// ============== 队列统计 ==============

// QueueStats 队列统计信息
type QueueStats struct {
	Total          int            `json:"total"`
	ByPriority     map[WakePriority]int `json:"byPriority"`
	OldestRequest  *WakeRequest   `json:"oldestRequest,omitempty"`
	NewestRequest  *WakeRequest   `json:"newestRequest,omitempty"`
	HighPriority   int            `json:"highPriority"`   // PriorityHigh 及以上
	CriticalOnly   int            `json:"criticalOnly"`   // PriorityCritical
}

// GetStats 获取队列统计信息
func (pq *WakePriorityQueue) GetStats() *QueueStats {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	stats := &QueueStats{
		Total:      len(pq.items),
		ByPriority: make(map[WakePriority]int),
	}

	if len(pq.items) == 0 {
		return stats
	}

	// 初始化所有优先级
	for p := PriorityLow; p <= PriorityCritical; p++ {
		stats.ByPriority[p] = 0
	}

	// 统计
	oldest := pq.items[0]
	newest := pq.items[0]

	for _, item := range pq.items {
		stats.ByPriority[item.Priority]++

		if item.Priority >= PriorityHigh {
			stats.HighPriority++
		}
		if item.Priority == PriorityCritical {
			stats.CriticalOnly++
		}

		if item.RequestedAt.Before(oldest.RequestedAt) {
			oldest = item
		}
		if item.RequestedAt.After(newest.RequestedAt) {
			newest = item
		}
	}

	stats.OldestRequest = oldest
	stats.NewestRequest = newest

	return stats
}
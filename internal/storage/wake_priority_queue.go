// Package storage 提供唤醒优先级队列实现
package storage

import (
	"container/heap"
	"sync"
)

// WakePriority 唤醒优先级
type WakePriority int

const (
	WakePriorityLow    WakePriority = 0
	WakePriorityNormal WakePriority = 1
	WakePriorityHigh   WakePriority = 2
)

// Note: WakeRequest is defined in disk_wake_trigger.go

// WakePriorityQueue 唤醒优先级队列
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
func (pq *WakePriorityQueue) AddRequest(req *WakeRequest) {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	heap.Push(pq, req)
}

// Pop 从队列取出最高优先级请求
func (pq *WakePriorityQueue) PopRequest() *WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	if pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq).(*WakeRequest)
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

// RemoveByID 根据ID移除请求
func (pq *WakePriorityQueue) RemoveByID(id string) bool {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	for i, item := range pq.items {
		if item.ID == id {
			heap.Remove(pq, i)
			return true
		}
	}
	return false
}

// GetByDiskPath 获取指定磁盘的请求列表
func (pq *WakePriorityQueue) GetByDiskPath(diskPath string) []*WakeRequest {
	pq.mu.Lock()
	defer pq.mu.Unlock()
	result := make([]*WakeRequest, 0)
	for _, item := range pq.items {
		if item.DiskPath == diskPath {
			result = append(result, item)
		}
	}
	return result
}

// ============== heap.Interface 实现 ==============

// Len 实现 heap.Interface
func (pq *WakePriorityQueue) Less(i, j int) bool {
	return pq.items[i].Priority > pq.items[j].Priority
}

// Swap 实现 heap.Interface
func (pq *WakePriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index = i
	pq.items[j].index = j
}

// Push 实现 heap.Interface (接收any类型)
func (pq *WakePriorityQueue) Push(x any) {
	item := x.(*WakeRequest)
	item.index = len(pq.items)
	pq.items = append(pq.items, item)
}

// Pop 实现 heap.Interface (返回any类型)
func (pq *WakePriorityQueue) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	pq.items = old[0 : n-1]
	return item
}

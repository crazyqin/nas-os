package sysresmon

import (
	"sync"
	"time"
)

// RingBuffer 环形缓冲区，存储资源快照历史.
type RingBuffer struct {
	mu       sync.RWMutex
	data     []ResourceSnapshot
	capacity int
	head     int
	tail     int
	count    int
}

// NewRingBuffer 创建环形缓冲区.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 2880 // 默认 24h（30秒间隔）
	}
	return &RingBuffer{
		data:     make([]ResourceSnapshot, capacity),
		capacity: capacity,
	}
}

// Push 添加新快照到缓冲区.
func (rb *RingBuffer) Push(snapshot ResourceSnapshot) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.data[rb.head] = snapshot
	rb.head = (rb.head + 1) % rb.capacity

	if rb.count == rb.capacity {
		// 缓冲区已满，移动尾部
		rb.tail = (rb.tail + 1) % rb.capacity
	} else {
		rb.count++
	}
}

// Latest 获取最新的快照.
func (rb *RingBuffer) Latest() *ResourceSnapshot {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	idx := (rb.head - 1 + rb.capacity) % rb.capacity
	return &rb.data[idx]
}

// Since 获取指定时间之后的所有快照.
func (rb *RingBuffer) Since(since time.Time) []ResourceSnapshot {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]ResourceSnapshot, 0, rb.count)
	current := rb.tail

	for i := 0; i < rb.count; i++ {
		snap := rb.data[current]
		if snap.Timestamp.After(since) || snap.Timestamp.Equal(since) {
			result = append(result, snap)
		}
		current = (current + 1) % rb.capacity
	}

	return result
}

// LastN 获取最近 N 条快照.
func (rb *RingBuffer) LastN(n int) []ResourceSnapshot {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	if n > rb.count {
		n = rb.count
	}

	result := make([]ResourceSnapshot, n)
	start := (rb.head - n + rb.capacity) % rb.capacity

	for i := 0; i < n; i++ {
		result[i] = rb.data[(start+i)%rb.capacity]
	}

	return result
}

// Count 获取缓冲区中的元素数量.
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// IsFull 缓冲区是否已满.
func (rb *RingBuffer) IsFull() bool {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count == rb.capacity
}

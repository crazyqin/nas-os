// Package audiostation 提供音乐中心管理功能
package audiostation

import (
	"math/rand"
	"sync"
)

// Player 播放队列管理器.
type Player struct {
	mu    sync.Mutex
	mgr   *Manager
	queue []string // trackID 列表
	index int      // 当前播放索引
	mode  PlayMode // 播放模式
}

// NewPlayer 创建播放队列管理器.
func NewPlayer(mgr *Manager) *Player {
	return &Player{
		mgr:   mgr,
		queue: make([]string, 0),
		index: 0,
		mode:  PlayModeOrder,
	}
}

// AddToQueue 添加曲目到播放队列.
func (p *Player) AddToQueue(trackIDs []string, position int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 验证曲目存在
	for _, id := range trackIDs {
		if _, err := p.mgr.GetTrack(id); err != nil {
			return err
		}
	}

	if position < 0 || position >= len(p.queue) {
		// 追加到末尾
		p.queue = append(p.queue, trackIDs...)
	} else {
		// 插入到指定位置
		newQueue := make([]string, 0, len(p.queue)+len(trackIDs))
		newQueue = append(newQueue, p.queue[:position]...)
		newQueue = append(newQueue, trackIDs...)
		newQueue = append(newQueue, p.queue[position:]...)
		p.queue = newQueue

		// 如果插入位置在当前播放位置之前，调整索引
		if position <= p.index {
			p.index += len(trackIDs)
		}
	}

	return nil
}

// RemoveFromQueue 从播放队列移除.
func (p *Player) RemoveFromQueue(index int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(p.queue) {
		return ErrQueueIndexInvalid
	}

	p.queue = append(p.queue[:index], p.queue[index+1:]...)

	// 调整当前播放索引
	if index < p.index {
		p.index--
	} else if index == p.index && p.index >= len(p.queue) && len(p.queue) > 0 {
		p.index = len(p.queue) - 1
	}

	return nil
}

// ReorderQueue 重排序播放队列.
func (p *Player) ReorderQueue(fromIndex, toIndex int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if fromIndex < 0 || fromIndex >= len(p.queue) {
		return ErrQueueIndexInvalid
	}
	if toIndex < 0 || toIndex >= len(p.queue) {
		return ErrQueueIndexInvalid
	}

	// 移动元素
	item := p.queue[fromIndex]
	p.queue = append(p.queue[:fromIndex], p.queue[fromIndex+1:]...)
	newQueue := make([]string, 0, len(p.queue)+1)
	newQueue = append(newQueue, p.queue[:toIndex]...)
	newQueue = append(newQueue, item)
	newQueue = append(newQueue, p.queue[toIndex:]...)
	p.queue = newQueue

	// 调整当前播放索引
	if fromIndex == p.index {
		p.index = toIndex
	} else {
		if fromIndex < p.index && toIndex >= p.index {
			p.index--
		} else if fromIndex > p.index && toIndex <= p.index {
			p.index++
		}
	}

	return nil
}

// GetQueue 获取当前播放队列.
func (p *Player) GetQueue() *PlayQueue {
	p.mu.Lock()
	defer p.mu.Unlock()

	items := make([]QueueItem, 0, len(p.queue))
	totalDuration := 0

	for i, trackID := range p.queue {
		item := QueueItem{
			Index:   i,
			TrackID: trackID,
		}

		// 填充曲目详情
		if track, err := p.mgr.GetTrack(trackID); err == nil {
			item.Track = track
			totalDuration += track.Duration
		}

		items = append(items, item)
	}

	return &PlayQueue{
		Items:         items,
		CurrentIndex:  p.index,
		Mode:          p.mode,
		TotalCount:    len(p.queue),
		TotalDuration: totalDuration,
	}
}

// ClearQueue 清空播放队列.
func (p *Player) ClearQueue() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.queue = make([]string, 0)
	p.index = 0
}

// SetMode 设置播放模式.
func (p *Player) SetMode(mode PlayMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// GetMode 获取播放模式.
func (p *Player) GetMode() PlayMode {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mode
}

// Next 下一曲.
func (p *Player) Next() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return "", ErrQueueEmpty
	}

	switch p.mode {
	case PlayModeRandom:
		p.index = rand.Intn(len(p.queue))
	case PlayModeRepeatOne:
		// 保持当前索引
	case PlayModeRepeatAll:
		p.index = (p.index + 1) % len(p.queue)
	default: // PlayModeOrder
		if p.index < len(p.queue)-1 {
			p.index++
		} else {
			return "", ErrQueueEmpty
		}
	}

	return p.queue[p.index], nil
}

// Prev 上一曲.
func (p *Player) Prev() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 {
		return "", ErrQueueEmpty
	}

	switch p.mode {
	case PlayModeRandom:
		p.index = rand.Intn(len(p.queue))
	case PlayModeRepeatOne:
		// 保持当前索引
	default:
		if p.index > 0 {
			p.index--
		} else if p.mode == PlayModeRepeatAll {
			p.index = len(p.queue) - 1
		}
	}

	return p.queue[p.index], nil
}

// GetCurrentTrackID 获取当前播放的曲目ID.
func (p *Player) GetCurrentTrackID() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.queue) == 0 || p.index >= len(p.queue) {
		return "", false
	}
	return p.queue[p.index], true
}

// JumpTo 跳转到指定索引.
func (p *Player) JumpTo(index int) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if index < 0 || index >= len(p.queue) {
		return "", ErrQueueIndexInvalid
	}

	p.index = index
	return p.queue[p.index], nil
}

// QueueLength 获取队列长度.
func (p *Player) QueueLength() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.queue)
}

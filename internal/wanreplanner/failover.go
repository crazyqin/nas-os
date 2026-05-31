package wanreplanner

import (
	"fmt"
	"time"
)

// CheckAndFailover 检测链路故障并执行切换
// 返回是否发生了故障切换
func (p *WANPlanner) CheckAndFailover() (bool, *FailoverEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 找到当前最优链路
	best := p.findBestLink()
	if best == nil {
		return false, nil
	}

	// 检查是否有需要切换的链路
	for _, link := range p.links {
		if link.Status == LinkStatusDown && link.Score > 0 {
			// 此链路刚故障，执行切换
			event := FailoverEvent{
				ID:         fmt.Sprintf("fo-%d", time.Now().UnixNano()),
				FromLinkID: link.ID,
				ToLinkID:   best.ID,
				Reason:     "link down detected",
				Timestamp:  time.Now(),
			}
			p.failoverLog = append(p.failoverLog, event)
			return true, &event
		}
	}
	return false, nil
}

// ExecuteFailover 执行从 fromID 到 toID 的故障切换
func (p *WANPlanner) ExecuteFailover(fromID, toID, reason string) (*FailoverEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	from, fromOK := p.links[fromID]
	to, toOK := p.links[toID]
	if !fromOK {
		return nil, fmt.Errorf("%w: %s", ErrLinkNotFound, fromID)
	}
	if !toOK {
		return nil, fmt.Errorf("%w: %s", ErrLinkNotFound, toID)
	}
	if to.Status != LinkStatusUp {
		return nil, fmt.Errorf("target link %s is not up", toID)
	}

	// 将故障链路标记为 down
	from.Status = LinkStatusDown
	from.UpdatedAt = time.Now()

	event := FailoverEvent{
		ID:         fmt.Sprintf("fo-%d", time.Now().UnixNano()),
		FromLinkID: fromID,
		ToLinkID:   toID,
		Reason:     reason,
		Timestamp:  time.Now(),
	}
	p.failoverLog = append(p.failoverLog, event)
	return &event, nil
}

// ExecuteFallback 执行回切：从备用链路切回主链路
func (p *WANPlanner) ExecuteFallback(primaryID, backupID string) (*FailoverEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	primary, primaryOK := p.links[primaryID]
	_, backupOK := p.links[backupID]
	if !primaryOK {
		return nil, fmt.Errorf("%w: %s", ErrLinkNotFound, primaryID)
	}
	if !backupOK {
		return nil, fmt.Errorf("%w: %s", ErrLinkNotFound, backupID)
	}
	if primary.Status != LinkStatusUp {
		return nil, fmt.Errorf("primary link %s is not up", primaryID)
	}

	event := FailoverEvent{
		ID:         fmt.Sprintf("fb-%d", time.Now().UnixNano()),
		FromLinkID: backupID,
		ToLinkID:   primaryID,
		Reason:     "fallback to primary",
		IsFallback: true,
		Timestamp:  time.Now(),
	}
	p.failoverLog = append(p.failoverLog, event)
	return &event, nil
}

// GetFailoverChain 获取从指定链路出发的完整故障切换链
func (p *WANPlanner) GetFailoverChain(linkID string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	chain := []string{linkID}
	visited := map[string]bool{linkID: true}

	for _, event := range p.failoverLog {
		if event.FromLinkID == chain[len(chain)-1] && !event.IsFallback {
			if !visited[event.ToLinkID] {
				chain = append(chain, event.ToLinkID)
				visited[event.ToLinkID] = true
			}
		}
	}
	return chain
}

// findBestLink 找到评分最高的活跃链路（调用者需持锁）
func (p *WANPlanner) findBestLink() *WANLink {
	var best *WANLink
	for _, l := range p.links {
		if l.Status != LinkStatusUp {
			continue
		}
		if best == nil || l.Score > best.Score {
			best = l
		}
	}
	return best
}

// AutoRecover 自动检测并恢复已恢复的链路
func (p *WANPlanner) AutoRecover() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	recovered := make([]string, 0)
	for _, link := range p.links {
		if link.Status == LinkStatusDown {
			// 模拟探测：检查最近一次探测时间
			if time.Since(link.LastCheck) < p.config.HealthCheckInterval*2 {
				// 链路可能已恢复，标记为 degraded 等待确认
				link.Status = LinkStatusDegraded
				link.UpdatedAt = time.Now()
				recovered = append(recovered, link.ID)
			}
		} else if link.Status == LinkStatusDegraded {
			// degraded 状态的链路如果评分恢复，标记为 UP
			if link.Score >= 60 {
				link.Status = LinkStatusUp
				link.UpdatedAt = time.Now()
				recovered = append(recovered, link.ID)
			}
		}
	}
	return recovered
}

// GetFailoverConfig 获取故障切换配置
func (p *WANPlanner) GetFailoverConfig() (failoverDelay, fallbackDelay time.Duration) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.FailoverDelay, p.config.FallbackDelay
}

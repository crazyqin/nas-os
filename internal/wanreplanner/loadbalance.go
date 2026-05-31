package wanreplanner

import (
	"hash/fnv"
	"sort"
)

// selectRoundRobin 轮询选择
func (p *WANPlanner) selectRoundRobin(links []*WANLink) *WANLink {
	if len(links) == 0 {
		return nil
	}
	idx := p.rrIndex % len(links)
	p.rrIndex++
	return links[idx]
}

// selectWeighted 加权选择（按权重比例分配）
func (p *WANPlanner) selectWeighted(links []*WANLink) *WANLink {
	if len(links) == 0 {
		return nil
	}
	totalWeight := 0
	for _, l := range links {
		w := l.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}
	if totalWeight == 0 {
		return links[0]
	}
	// 基于 round-robin 索引做加权轮询
	pos := p.rrIndex % totalWeight
	p.rrIndex++
	cumulative := 0
	for _, l := range links {
		w := l.Weight
		if w <= 0 {
			w = 1
		}
		cumulative += w
		if pos < cumulative {
			return l
		}
	}
	return links[len(links)-1]
}

// selectLeastConn 最少连接选择
func (p *WANPlanner) selectLeastConn(links []*WANLink) *WANLink {
	if len(links) == 0 {
		return nil
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].ActiveConns == links[j].ActiveConns {
			return links[i].Score > links[j].Score
		}
		return links[i].ActiveConns < links[j].ActiveConns
	})
	return links[0]
}

// selectSourceHash 源地址哈希选择
func (p *WANPlanner) selectSourceHash(links []*WANLink, srcIP string) *WANLink {
	if len(links) == 0 {
		return nil
	}
	if srcIP == "" {
		return p.selectRoundRobin(links)
	}
	h := fnv.New32a()
	h.Write([]byte(srcIP))
	hash := h.Sum32()
	idx := hash % uint32(len(links))
	return links[idx]
}

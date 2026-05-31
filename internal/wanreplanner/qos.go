package wanreplanner

import (
	"fmt"
	"time"
)

// AddQoSRule 添加 QoS 规则
func (p *WANPlanner) AddQoSRule(rule *QoSRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}
	if _, exists := p.rules[rule.ID]; exists {
		return fmt.Errorf("rule %s already exists", rule.ID)
	}
	rule.CreatedAt = time.Now()
	p.rules[rule.ID] = rule
	return nil
}

// RemoveQoSRule 移除 QoS 规则
func (p *WANPlanner) RemoveQoSRule(ruleID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.rules[ruleID]; !exists {
		return ErrRuleNotFound
	}
	delete(p.rules, ruleID)
	return nil
}

// UpdateQoSRule 更新 QoS 规则
func (p *WANPlanner) UpdateQoSRule(rule *QoSRule) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.rules[rule.ID]; !exists {
		return ErrRuleNotFound
	}
	p.rules[rule.ID] = rule
	return nil
}

// GetQoSRule 获取 QoS 规则
func (p *WANPlanner) GetQoSRule(ruleID string) (*QoSRule, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	rule, exists := p.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	copy := *rule
	return &copy, nil
}

// ListQoSRules 列出所有 QoS 规则
func (p *WANPlanner) ListQoSRules() []*QoSRule {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*QoSRule, 0, len(p.rules))
	for _, r := range p.rules {
		copy := *r
		result = append(result, &copy)
	}
	return result
}

// ClassifyTraffic 根据协议和端口匹配流量分类
// 返回匹配的最高优先级规则
func (p *WANPlanner) ClassifyTraffic(protocol string, srcPort, dstPort int) *QoSRule {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *QoSRule
	for _, rule := range p.rules {
		if !rule.Enabled {
			continue
		}
		if !matchProtocol(rule.Protocol, protocol) {
			continue
		}
		if rule.SrcPort != 0 && rule.SrcPort != srcPort {
			continue
		}
		if rule.DstPort != 0 && rule.DstPort != dstPort {
			continue
		}
		if best == nil || rule.Priority < best.Priority {
			best = rule
		}
	}
	if best != nil {
		copy := *best
		return &copy
	}
	return nil
}

// GetTrafficClasses 获取所有流量分类统计
func (p *WANPlanner) GetTrafficClasses() []TrafficClass {
	p.mu.RLock()
	defer p.mu.RUnlock()

	classes := make(map[string]*TrafficClass)
	for _, rule := range p.rules {
		name := rule.Name
		if _, exists := classes[name]; !exists {
			classes[name] = &TrafficClass{
				Name:     name,
				Priority: rule.Priority,
			}
		}
	}

	result := make([]TrafficClass, 0, len(classes))
	for _, c := range classes {
		result = append(result, *c)
	}
	return result
}

// EnableQoSRule 启用规则
func (p *WANPlanner) EnableQoSRule(ruleID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	rule, exists := p.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}
	rule.Enabled = true
	return nil
}

// DisableQoSRule 禁用规则
func (p *WANPlanner) DisableQoSRule(ruleID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	rule, exists := p.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}
	rule.Enabled = false
	return nil
}

// GetEffectiveBandwidth 获取指定优先级的有效带宽（考虑限速规则）
func (p *WANPlanner) GetEffectiveBandwidth(linkID string, priority QoSPriority) (int64, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	link, exists := p.links[linkID]
	if !exists {
		return 0, ErrLinkNotFound
	}

	totalBandwidth := link.Bandwidth
	if totalBandwidth <= 0 {
		return 0, nil
	}

	// 找到匹配优先级的限速规则
	for _, rule := range p.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Priority == priority && rule.MaxBandwidth > 0 {
			if rule.MaxBandwidth < totalBandwidth {
				return rule.MaxBandwidth, nil
			}
		}
	}

	return totalBandwidth, nil
}

// matchProtocol 匹配协议
func matchProtocol(ruleProto, pktProto string) bool {
	if ruleProto == "" || ruleProto == "any" {
		return true
	}
	return ruleProto == pktProto
}

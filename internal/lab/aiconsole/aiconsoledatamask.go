package aiconsole

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewDataMaskEngine 创建脱敏引擎.
func NewDataMaskEngine(config *DataMaskConfig) *DataMaskEngine {
	if config == nil {
		config = DefaultDataMaskConfig()
	}

	engine := &DataMaskEngine{
		config:   config,
		patterns: make(map[string]*SensitivePattern),
		rules:    make(map[string]*MaskRule),
		stats: &MaskStats{
			ByType:      make(map[MaskType]int64),
			ByStrategy:  make(map[MaskStrategy]int64),
			LastUpdated: time.Now(),
		},
		cache:  make(map[string]*MaskResult),
		stopCh: make(chan struct{}),
	}

	// 初始化默认模式
	engine.initDefaultPatterns()

	return engine
}

// initDefaultPatterns 初始化默认模式.
func (e *DataMaskEngine) initDefaultPatterns() {
	defaultPatterns := []*SensitivePattern{
		{
			ID:       "email",
			Name:     "邮箱地址",
			Type:     MaskTypeEmail,
			Pattern:  `[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`,
			Strategy: StrategyPartial,
			Enabled:  true,
			Priority: 1,
		},
		{
			ID:       "phone",
			Name:     "手机号码",
			Type:     MaskTypePhone,
			Pattern:  `1[3-9]\d{9}`,
			Strategy: StrategyPartial,
			Enabled:  true,
			Priority: 1,
		},
		{
			ID:       "idcard",
			Name:     "身份证号",
			Type:     MaskTypeIDCard,
			Pattern:  `[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`,
			Strategy: StrategyPartial,
			Enabled:  true,
			Priority: 1,
		},
		{
			ID:       "creditcard",
			Name:     "信用卡号",
			Type:     MaskTypeCreditCard,
			Pattern:  `\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}`,
			Strategy: StrategyPartial,
			Enabled:  true,
			Priority: 1,
		},
	}

	for _, pattern := range defaultPatterns {
		pattern.CreatedAt = time.Now()
		e.patterns[pattern.ID] = pattern
	}
}

// Start 启动引擎.
func (e *DataMaskEngine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("脱敏引擎已在运行")
	}

	e.running = true
	log.Println("[AICSODataMask] 数据脱敏引擎启动")

	// 启动缓存清理器
	go e.cacheCleaner()

	return nil
}

// Stop 停止引擎.
func (e *DataMaskEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	close(e.stopCh)
	e.running = false
	log.Println("[AICSODataMask] 数据脱敏引擎停止")
}

// IsRunning 检查是否运行中.
func (e *DataMaskEngine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// Mask 执行脱敏.
func (e *DataMaskEngine) Mask(request *MaskRequest) (*MaskResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.running {
		return nil, fmt.Errorf("脱敏引擎未运行")
	}

	startTime := time.Now()

	// 检查输入长度
	if len(request.Text) > e.config.MaxInputLength {
		return nil, fmt.Errorf("输入文本超过最大长度限制: %d", e.config.MaxInputLength)
	}

	// 检查缓存
	if e.config.CacheEnabled {
		e.cacheMu.RLock()
		if cached, exists := e.cache[request.Text]; exists {
			e.cacheMu.RUnlock()
			return cached, nil
		}
		e.cacheMu.RUnlock()
	}

	// 执行脱敏
	maskedText := request.Text
	detectedTypes := make([]MaskType, 0)
	maskedCount := 0

	// 应用模式匹配和脱敏
	for _, pattern := range e.patterns {
		if !pattern.Enabled {
			continue
		}

		// 检查是否应该应用此模式
		if len(request.Types) > 0 {
			found := false
			for _, t := range request.Types {
				if t == pattern.Type {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 编译正则表达式
		regex, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			log.Printf("[AICSODataMask] 正则编译失败: %s - %v", pattern.Pattern, err)
			continue
		}

		// 查找匹配
		matches := regex.FindAllString(maskedText, -1)
		if len(matches) > 0 {
			detectedTypes = append(detectedTypes, pattern.Type)
			maskedCount += len(matches)

			// 应用脱敏策略
			strategy := pattern.Strategy
			if request.Strategy != "" {
				strategy = request.Strategy
			}

			for _, match := range matches {
				masked := e.applyStrategy(match, strategy)
				maskedText = strings.Replace(maskedText, match, masked, 1)
			}
		}
	}

	result := &MaskResult{
		OriginalLength: len(request.Text),
		MaskedLength:   len(maskedText),
		MaskedText:     maskedText,
		DetectedTypes:  detectedTypes,
		MaskedCount:    maskedCount,
		ProcessingTime: time.Since(startTime),
	}

	// 更新缓存
	if e.config.CacheEnabled {
		e.cacheMu.Lock()
		e.cache[request.Text] = result
		e.cacheMu.Unlock()
	}

	// 更新统计
	e.stats.mu.Lock()
	e.stats.TotalRequests++
	e.stats.TotalMasked += int64(maskedCount)
	for _, t := range detectedTypes {
		e.stats.ByType[t]++
	}
	e.stats.ByStrategy[request.Strategy]++
	e.stats.LastUpdated = time.Now()
	e.stats.mu.Unlock()

	return result, nil
}

// applyStrategy 应用脱敏策略.
func (e *DataMaskEngine) applyStrategy(text string, strategy MaskStrategy) string {
	switch strategy {
	case StrategyPartial:
		return e.partialMask(text)
	case StrategyReplace:
		return strings.Repeat("*", len(text))
	case StrategyHash:
		return e.hashText(text)
	case StrategyEncrypt:
		return "[ENCRYPTED]"
	case StrategyRemove:
		return ""
	default:
		return e.partialMask(text)
	}
}

// partialMask 部分遮挡.
func (e *DataMaskEngine) partialMask(text string) string {
	if len(text) <= 2 {
		return strings.Repeat("*", len(text))
	}

	// 保留首尾字符
	return string(text[0]) + strings.Repeat("*", len(text)-2) + string(text[len(text)-1])
}

// hashText 哈希处理.
func (e *DataMaskEngine) hashText(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:8]) // 使用前8字节
}

// BatchMask 批量脱敏.
func (e *DataMaskEngine) BatchMask(request *BatchMaskRequest) (*BatchMaskResult, error) {
	results := make([]MaskResult, 0, len(request.Requests))
	success := 0
	failed := 0

	for _, req := range request.Requests {
		result, err := e.Mask(&req)
		if err != nil {
			failed++
			results = append(results, MaskResult{
				OriginalLength: len(req.Text),
				MaskedText:     req.Text,
			})
		} else {
			success++
			results = append(results, *result)
		}
	}

	return &BatchMaskResult{
		Results: results,
		Total:   len(request.Requests),
		Success: success,
		Failed:  failed,
	}, nil
}

// AddPattern 添加模式.
func (e *DataMaskEngine) AddPattern(pattern *SensitivePattern) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if pattern.ID == "" {
		pattern.ID = uuid.New().String()
	}

	// 验证正则表达式
	if _, err := regexp.Compile(pattern.Pattern); err != nil {
		return fmt.Errorf("无效的正则表达式: %v", err)
	}

	pattern.CreatedAt = time.Now()
	e.patterns[pattern.ID] = pattern

	log.Printf("[AICSODataMask] 添加模式: %s - %s", pattern.ID, pattern.Name)

	return nil
}

// UpdatePattern 更新模式.
func (e *DataMaskEngine) UpdatePattern(id string, pattern *SensitivePattern) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.patterns[id]; !exists {
		return fmt.Errorf("模式不存在: %s", id)
	}

	// 验证正则表达式
	if _, err := regexp.Compile(pattern.Pattern); err != nil {
		return fmt.Errorf("无效的正则表达式: %v", err)
	}

	pattern.ID = id
	e.patterns[id] = pattern

	log.Printf("[AICSODataMask] 更新模式: %s - %s", id, pattern.Name)

	return nil
}

// DeletePattern 删除模式.
func (e *DataMaskEngine) DeletePattern(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.patterns[id]; !exists {
		return fmt.Errorf("模式不存在: %s", id)
	}

	delete(e.patterns, id)
	log.Printf("[AICSODataMask] 删除模式: %s", id)

	return nil
}

// GetPattern 获取模式.
func (e *DataMaskEngine) GetPattern(id string) (*SensitivePattern, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	pattern, exists := e.patterns[id]
	if !exists {
		return nil, fmt.Errorf("模式不存在: %s", id)
	}

	return pattern, nil
}

// ListPatterns 列出所有模式.
func (e *DataMaskEngine) ListPatterns() []*SensitivePattern {
	e.mu.RLock()
	defer e.mu.RUnlock()

	patterns := make([]*SensitivePattern, 0, len(e.patterns))
	for _, pattern := range e.patterns {
		patterns = append(patterns, pattern)
	}
	return patterns
}

// AddRule 添加规则.
func (e *DataMaskEngine) AddRule(rule *MaskRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	e.rules[rule.ID] = rule
	log.Printf("[AICSODataMask] 添加规则: %s - %s", rule.ID, rule.Name)

	return nil
}

// GetRule 获取规则.
func (e *DataMaskEngine) GetRule(id string) (*MaskRule, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rule, exists := e.rules[id]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", id)
	}

	return rule, nil
}

// ListRules 列出所有规则.
func (e *DataMaskEngine) ListRules() []*MaskRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rules := make([]*MaskRule, 0, len(e.rules))
	for _, rule := range e.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetStats 获取统计信息.
func (e *DataMaskEngine) GetStats() *MaskStats {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()

	return &MaskStats{
		TotalRequests:  e.stats.TotalRequests,
		TotalMasked:    e.stats.TotalMasked,
		ByType:         copyTypeMap(e.stats.ByType),
		ByStrategy:     copyStrategyMap(e.stats.ByStrategy),
		AvgProcessTime: e.stats.AvgProcessTime,
		LastUpdated:    e.stats.LastUpdated,
	}
}

// GetConfig 获取配置.
func (e *DataMaskEngine) GetConfig() *DataMaskConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.config
}

// UpdateConfig 更新配置.
func (e *DataMaskEngine) UpdateConfig(config *DataMaskConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.config = config
	log.Printf("[AICSODataMask] 配置已更新")
}

// cacheCleaner 缓存清理器.
func (e *DataMaskEngine) cacheCleaner() {
	ticker := time.NewTicker(e.config.CacheTTL)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.cleanCache()
		}
	}
}

// cleanCache 清理缓存.
func (e *DataMaskEngine) cleanCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	// 简化实现：清空所有缓存
	// 实际实现应该检查 TTL
	e.cache = make(map[string]*MaskResult)
}

// copyTypeMap 复制类型映射.
func copyTypeMap(src map[MaskType]int64) map[MaskType]int64 {
	dst := make(map[MaskType]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyStrategyMap 复制策略映射.
func copyStrategyMap(src map[MaskStrategy]int64) map[MaskStrategy]int64 {
	dst := make(map[MaskStrategy]int64)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ClearCache 清空缓存.
func (e *DataMaskEngine) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	e.cache = make(map[string]*MaskResult)
	log.Println("[AICSODataMask] 缓存已清空")
}

// GetCacheSize 获取缓存大小.
func (e *DataMaskEngine) GetCacheSize() int {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()

	return len(e.cache)
}

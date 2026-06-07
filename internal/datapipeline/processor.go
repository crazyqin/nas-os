package datapipeline

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Processor 数据处理器接口
type Processor interface {
	// Process 处理数据
	Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error)
	// GetType 获取处理器类型
	GetType() ProcessorType
	// GetName 获取处理器名称
	GetName() string
}

// ProcessorFactory 处理器工厂
type ProcessorFactory struct {
	factories map[ProcessorType]func(config map[string]interface{}) (Processor, error)
	mu        sync.RWMutex
}

// NewProcessorFactory 创建处理器工厂
func NewProcessorFactory() *ProcessorFactory {
	f := &ProcessorFactory{
		factories: make(map[ProcessorType]func(config map[string]interface{}) (Processor, error)),
	}

	// 注册内置处理器
	f.Register(ProcessorTypeFilter, NewFilterProcessor)
	f.Register(ProcessorTypeTransform, NewTransformProcessor)
	f.Register(ProcessorTypeAggregate, NewAggregateProcessor)
	f.Register(ProcessorTypeEnrichment, NewEnrichmentProcessor)
	f.Register(ProcessorTypeValidator, NewValidatorProcessor)
	f.Register(ProcessorTypeDeduplicator, NewDeduplicatorProcessor)
	f.Register(ProcessorTypeRouter, NewRouterProcessor)

	return f
}

// Register 注册处理器工厂方法
func (f *ProcessorFactory) Register(pType ProcessorType, factory func(config map[string]interface{}) (Processor, error)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.factories[pType] = factory
}

// Create 创建处理器实例
func (f *ProcessorFactory) Create(pType ProcessorType, config map[string]interface{}) (Processor, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	factory, ok := f.factories[pType]
	if !ok {
		return nil, fmt.Errorf("unsupported processor type: %s", pType)
	}

	return factory(config)
}

// FilterProcessor 过滤器处理器
type FilterProcessor struct {
	name      string
	config    map[string]interface{}
	condition FilterCondition
}

// FilterCondition 过滤条件
type FilterCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

// NewFilterProcessor 创建过滤器处理器
func NewFilterProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "filter"
	}

	condition, ok := config["condition"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("filter processor requires 'condition' in config")
	}

	cond := FilterCondition{
		Field:    condition["field"].(string),
		Operator: condition["operator"].(string),
		Value:    condition["value"],
	}

	return &FilterProcessor{
		name:      name,
		config:    config,
		condition: cond,
	}, nil
}

// Process 执行过滤
func (p *FilterProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	var result []map[string]interface{}

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if p.matchCondition(item, p.condition) {
			result = append(result, item)
		}
	}

	return result, nil
}

// matchCondition 匹配条件
func (p *FilterProcessor) matchCondition(item map[string]interface{}, cond FilterCondition) bool {
	value, exists := item[cond.Field]
	if !exists {
		return false
	}

	switch cond.Operator {
	case "equals", "eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
	case "not_equals", "ne":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Value)
	case "contains":
		strVal := fmt.Sprintf("%v", value)
		strCond := fmt.Sprintf("%v", cond.Value)
		return strings.Contains(strVal, strCond)
	case "starts_with":
		strVal := fmt.Sprintf("%v", value)
		strCond := fmt.Sprintf("%v", cond.Value)
		return strings.HasPrefix(strVal, strCond)
	case "ends_with":
		strVal := fmt.Sprintf("%v", value)
		strCond := fmt.Sprintf("%v", cond.Value)
		return strings.HasSuffix(strVal, strCond)
	case "regex":
		strVal := fmt.Sprintf("%v", value)
		strCond := fmt.Sprintf("%v", cond.Value)
		matched, _ := regexp.MatchString(strCond, strVal)
		return matched
	case "greater_than", "gt":
		return compareNumeric(value, cond.Value) > 0
	case "less_than", "lt":
		return compareNumeric(value, cond.Value) < 0
	case "greater_equal", "gte":
		return compareNumeric(value, cond.Value) >= 0
	case "less_equal", "lte":
		return compareNumeric(value, cond.Value) <= 0
	default:
		return false
	}
}

// compareNumeric 比较数值
func compareNumeric(a, b interface{}) int {
	aFloat, aOk := toFloat64(a)
	bFloat, bOk := toFloat64(b)

	if aOk && bOk {
		if aFloat < bFloat {
			return -1
		}
		if aFloat > bFloat {
			return 1
		}
		return 0
	}

	// 回退到字符串比较
	strA := fmt.Sprintf("%v", a)
	strB := fmt.Sprintf("%v", b)
	return strings.Compare(strA, strB)
}

// GetType 获取处理器类型
func (p *FilterProcessor) GetType() ProcessorType {
	return ProcessorTypeFilter
}

// GetName 获取处理器名称
func (p *FilterProcessor) GetName() string {
	return p.name
}

// TransformProcessor 转换器处理器
type TransformProcessor struct {
	name       string
	config     map[string]interface{}
	transforms []TransformRule
}

// TransformRule 转换规则
type TransformRule struct {
	Field    string `json:"field"`
	Action   string `json:"action"`
	Template string `json:"template,omitempty"`
	Value    string `json:"value,omitempty"`
}

// NewTransformProcessor 创建转换器处理器
func NewTransformProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "transform"
	}

	rulesRaw, ok := config["transforms"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("transform processor requires 'transforms' in config")
	}

	transforms := make([]TransformRule, 0, len(rulesRaw))
	for _, ruleRaw := range rulesRaw {
		rule, ok := ruleRaw.(map[string]interface{})
		if !ok {
			continue
		}

		transformRule := TransformRule{
			Field:  rule["field"].(string),
			Action: rule["action"].(string),
		}

		if v, ok := rule["template"].(string); ok {
			transformRule.Template = v
		}
		if v, ok := rule["value"].(string); ok {
			transformRule.Value = v
		}

		transforms = append(transforms, transformRule)
	}

	return &TransformProcessor{
		name:       name,
		config:     config,
		transforms: transforms,
	}, nil
}

// Process 执行转换
func (p *TransformProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(input))

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 深拷贝 item
		transformed := make(map[string]interface{})
		for k, v := range item {
			transformed[k] = v
		}

		// 应用转换规则
		for _, rule := range p.transforms {
			p.applyTransform(transformed, rule)
		}

		result = append(result, transformed)
	}

	return result, nil
}

// applyTransform 应用转换规则
func (p *TransformProcessor) applyTransform(item map[string]interface{}, rule TransformRule) {
	switch rule.Action {
	case "rename":
		// 重命名字段
		if value, exists := item[rule.Field]; exists {
			delete(item, rule.Field)
			item[rule.Value] = value
		}
	case "set":
		// 设置固定值
		item[rule.Field] = rule.Value
	case "copy":
		// 复制字段
		if value, exists := item[rule.Field]; exists {
			item[rule.Value] = value
		}
	case "delete":
		// 删除字段
		delete(item, rule.Field)
	case "uppercase":
		// 转大写
		if value, ok := item[rule.Field].(string); ok {
			item[rule.Field] = strings.ToUpper(value)
		}
	case "lowercase":
		// 转小写
		if value, ok := item[rule.Field].(string); ok {
			item[rule.Field] = strings.ToLower(value)
		}
	case "trim":
		// 去除空白
		if value, ok := item[rule.Field].(string); ok {
			item[rule.Field] = strings.TrimSpace(value)
		}
	case "default":
		// 设置默认值（如果字段不存在或为空）
		if _, exists := item[rule.Field]; !exists {
			item[rule.Field] = rule.Value
		}
	}
}

// GetType 获取处理器类型
func (p *TransformProcessor) GetType() ProcessorType {
	return ProcessorTypeTransform
}

// GetName 获取处理器名称
func (p *TransformProcessor) GetName() string {
	return p.name
}

// AggregateProcessor 聚合器处理器
type AggregateProcessor struct {
	name       string
	config     map[string]interface{}
	groupBy    []string
	aggregates []AggregateRule
}

// AggregateRule 聚合规则
type AggregateRule struct {
	Field    string `json:"field"`
	Function string `json:"function"`
	Output   string `json:"output"`
}

// NewAggregateProcessor 创建聚合器处理器
func NewAggregateProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "aggregate"
	}

	groupByRaw, ok := config["groupBy"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("aggregate processor requires 'groupBy' in config")
	}

	groupBy := make([]string, 0, len(groupByRaw))
	for _, g := range groupByRaw {
		if s, ok := g.(string); ok {
			groupBy = append(groupBy, s)
		}
	}

	aggsRaw, ok := config["aggregates"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("aggregate processor requires 'aggregates' in config")
	}

	aggregates := make([]AggregateRule, 0, len(aggsRaw))
	for _, aggRaw := range aggsRaw {
		agg, ok := aggRaw.(map[string]interface{})
		if !ok {
			continue
		}
		aggregates = append(aggregates, AggregateRule{
			Field:    agg["field"].(string),
			Function: agg["function"].(string),
			Output:   agg["output"].(string),
		})
	}

	return &AggregateProcessor{
		name:       name,
		config:     config,
		groupBy:    groupBy,
		aggregates: aggregates,
	}, nil
}

// Process 执行聚合
func (p *AggregateProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	// 按 groupBy 字段分组
	groups := make(map[string][]map[string]interface{})

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 构建分组键
		key := p.buildGroupKey(item)
		groups[key] = append(groups[key], item)
	}

	// 对每个分组执行聚合
	result := make([]map[string]interface{}, 0, len(groups))
	for _, group := range groups {
		aggregated := p.aggregateGroup(group)
		result = append(result, aggregated)
	}

	return result, nil
}

// buildGroupKey 构建分组键
func (p *AggregateProcessor) buildGroupKey(item map[string]interface{}) string {
	parts := make([]string, 0, len(p.groupBy))
	for _, field := range p.groupBy {
		value := item[field]
		parts = append(parts, fmt.Sprintf("%v", value))
	}
	return strings.Join(parts, "|")
}

// aggregateGroup 对分组执行聚合
func (p *AggregateProcessor) aggregateGroup(group []map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// 复制第一个元素的分组字段
	if len(group) > 0 {
		for _, field := range p.groupBy {
			result[field] = group[0][field]
		}
	}

	// 执行聚合
	for _, agg := range p.aggregates {
		output := agg.Output
		if output == "" {
			output = agg.Function + "_" + agg.Field
		}

		switch agg.Function {
		case "count":
			result[output] = len(group)
		case "sum":
			sum := 0.0
			for _, item := range group {
				if val, ok := toFloat64(item[agg.Field]); ok {
					sum += val
				}
			}
			result[output] = sum
		case "avg", "average":
			sum := 0.0
			count := 0
			for _, item := range group {
				if val, ok := toFloat64(item[agg.Field]); ok {
					sum += val
					count++
				}
			}
			if count > 0 {
				result[output] = sum / float64(count)
			}
		case "min":
			var min interface{}
			for _, item := range group {
				val := item[agg.Field]
				if min == nil || compareNumeric(val, min) < 0 {
					min = val
				}
			}
			result[output] = min
		case "max":
			var max interface{}
			for _, item := range group {
				val := item[agg.Field]
				if max == nil || compareNumeric(val, max) > 0 {
					max = val
				}
			}
			result[output] = max
		case "first":
			if len(group) > 0 {
				result[output] = group[0][agg.Field]
			}
		case "last":
			if len(group) > 0 {
				result[output] = group[len(group)-1][agg.Field]
			}
		}
	}

	return result
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	case float32:
		return float64(val), true
	default:
		return 0, false
	}
}

// GetType 获取处理器类型
func (p *AggregateProcessor) GetType() ProcessorType {
	return ProcessorTypeAggregate
}

// GetName 获取处理器名称
func (p *AggregateProcessor) GetName() string {
	return p.name
}

// EnrichmentProcessor 富化器处理器
type EnrichmentProcessor struct {
	name   string
	config map[string]interface{}
	fields []EnrichmentField
}

// EnrichmentField 富化字段
type EnrichmentField struct {
	Field    string      `json:"field"`
	Source   string      `json:"source"` // static, computed, lookup
	Value    interface{} `json:"value"`
	Template string      `json:"template"`
}

// NewEnrichmentProcessor 创建富化器处理器
func NewEnrichmentProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "enrichment"
	}

	fieldsRaw, ok := config["fields"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("enrichment processor requires 'fields' in config")
	}

	fields := make([]EnrichmentField, 0, len(fieldsRaw))
	for _, fieldRaw := range fieldsRaw {
		field, ok := fieldRaw.(map[string]interface{})
		if !ok {
			continue
		}
		fields = append(fields, EnrichmentField{
			Field:    field["field"].(string),
			Source:   field["source"].(string),
			Value:    field["value"],
			Template: field["template"].(string),
		})
	}

	return &EnrichmentProcessor{
		name:   name,
		config: config,
		fields: fields,
	}, nil
}

// Process 执行富化
func (p *EnrichmentProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(input))

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 深拷贝
		enriched := make(map[string]interface{})
		for k, v := range item {
			enriched[k] = v
		}

		// 应用富化规则
		for _, field := range p.fields {
			p.applyEnrichment(enriched, field)
		}

		result = append(result, enriched)
	}

	return result, nil
}

// applyEnrichment 应用富化规则
func (p *EnrichmentProcessor) applyEnrichment(item map[string]interface{}, field EnrichmentField) {
	switch field.Source {
	case "static":
		item[field.Field] = field.Value
	case "computed":
		// 简化的计算逻辑
		item[field.Field] = field.Value
	case "timestamp":
		item[field.Field] = time.Now()
	case "template":
		// 简化的模板替换
		result := field.Template
		for k, v := range item {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", k), fmt.Sprintf("%v", v))
		}
		item[field.Field] = result
	}
}

// GetType 获取处理器类型
func (p *EnrichmentProcessor) GetType() ProcessorType {
	return ProcessorTypeEnrichment
}

// GetName 获取处理器名称
func (p *EnrichmentProcessor) GetName() string {
	return p.name
}

// ValidatorProcessor 验证器处理器
type ValidatorProcessor struct {
	name       string
	config     map[string]interface{}
	rules      []ValidationRule
	strictMode bool
}

// ValidationRule 验证规则
type ValidationRule struct {
	Field    string `json:"field"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // string, number, boolean, email, url
	Min      int    `json:"min,omitempty"`
	Max      int    `json:"max,omitempty"`
	Pattern  string `json:"pattern,omitempty"`
}

// NewValidatorProcessor 创建验证器处理器
func NewValidatorProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "validator"
	}

	strictMode, _ := config["strictMode"].(bool)

	rulesRaw, ok := config["rules"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("validator processor requires 'rules' in config")
	}

	rules := make([]ValidationRule, 0, len(rulesRaw))
	for _, ruleRaw := range rulesRaw {
		rule, ok := ruleRaw.(map[string]interface{})
		if !ok {
			continue
		}

		validationRule := ValidationRule{
			Field:    rule["field"].(string),
			Required: rule["required"].(bool),
			Type:     rule["type"].(string),
		}

		if min, ok := rule["min"].(int); ok {
			validationRule.Min = min
		}
		if max, ok := rule["max"].(int); ok {
			validationRule.Max = max
		}
		if pattern, ok := rule["pattern"].(string); ok {
			validationRule.Pattern = pattern
		}

		rules = append(rules, validationRule)
	}

	return &ValidatorProcessor{
		name:       name,
		config:     config,
		rules:      rules,
		strictMode: strictMode,
	}, nil
}

// Process 执行验证
func (p *ValidatorProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	var result []map[string]interface{}
	var errors []string

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		valid := true
		for _, rule := range p.rules {
			if err := p.validateField(item, rule); err != nil {
				if p.strictMode {
					return nil, fmt.Errorf("validation failed for field %s: %w", rule.Field, err)
				}
				valid = false
				errors = append(errors, fmt.Sprintf("%s: %v", rule.Field, err))
			}
		}

		if valid || !p.strictMode {
			result = append(result, item)
		}
	}

	if len(errors) > 0 {
		// 在非严格模式下记录警告
	}

	return result, nil
}

// validateField 验证字段
func (p *ValidatorProcessor) validateField(item map[string]interface{}, rule ValidationRule) error {
	value, exists := item[rule.Field]

	// 必填检查
	if rule.Required && (!exists || value == nil) {
		return fmt.Errorf("field is required")
	}

	if !exists || value == nil {
		return nil
	}

	// 类型检查
	switch rule.Type {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "number":
		if _, ok := toFloat64(value); !ok {
			return fmt.Errorf("expected number, got %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	}

	// 长度检查（字符串）
	if str, ok := value.(string); ok {
		if rule.Min > 0 && len(str) < rule.Min {
			return fmt.Errorf("minimum length is %d, got %d", rule.Min, len(str))
		}
		if rule.Max > 0 && len(str) > rule.Max {
			return fmt.Errorf("maximum length is %d, got %d", rule.Max, len(str))
		}
	}

	// 正则检查
	if rule.Pattern != "" {
		if str, ok := value.(string); ok {
			matched, err := regexp.MatchString(rule.Pattern, str)
			if err != nil {
				return fmt.Errorf("invalid pattern: %w", err)
			}
			if !matched {
				return fmt.Errorf("does not match pattern: %s", rule.Pattern)
			}
		}
	}

	return nil
}

// GetType 获取处理器类型
func (p *ValidatorProcessor) GetType() ProcessorType {
	return ProcessorTypeValidator
}

// GetName 获取处理器名称
func (p *ValidatorProcessor) GetName() string {
	return p.name
}

// DeduplicatorProcessor 去重器处理器
type DeduplicatorProcessor struct {
	name   string
	config map[string]interface{}
	fields []string
}

// NewDeduplicatorProcessor 创建去重器处理器
func NewDeduplicatorProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "deduplicator"
	}

	fieldsRaw, ok := config["fields"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("deduplicator processor requires 'fields' in config")
	}

	fields := make([]string, 0, len(fieldsRaw))
	for _, f := range fieldsRaw {
		if s, ok := f.(string); ok {
			fields = append(fields, s)
		}
	}

	return &DeduplicatorProcessor{
		name:   name,
		config: config,
		fields: fields,
	}, nil
}

// Process 执行去重
func (p *DeduplicatorProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	seen := make(map[string]bool)
	var result []map[string]interface{}

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// 构建唯一键
		key := p.buildDedupeKey(item)
		if !seen[key] {
			seen[key] = true
			result = append(result, item)
		}
	}

	return result, nil
}

// buildDedupeKey 构建去重键
func (p *DeduplicatorProcessor) buildDedupeKey(item map[string]interface{}) string {
	parts := make([]string, 0, len(p.fields))
	for _, field := range p.fields {
		value := item[field]
		parts = append(parts, fmt.Sprintf("%v", value))
	}
	return strings.Join(parts, "|")
}

// GetType 获取处理器类型
func (p *DeduplicatorProcessor) GetType() ProcessorType {
	return ProcessorTypeDeduplicator
}

// GetName 获取处理器名称
func (p *DeduplicatorProcessor) GetName() string {
	return p.name
}

// RouterProcessor 路由器处理器
type RouterProcessor struct {
	name   string
	config map[string]interface{}
	routes []RouteRule
}

// RouteRule 路由规则
type RouteRule struct {
	Name      string          `json:"name"`
	Condition FilterCondition `json:"condition"`
	OutputKey string          `json:"outputKey"`
}

// NewRouterProcessor 创建路由器处理器
func NewRouterProcessor(config map[string]interface{}) (Processor, error) {
	name, _ := config["name"].(string)
	if name == "" {
		name = "router"
	}

	routesRaw, ok := config["routes"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("router processor requires 'routes' in config")
	}

	routes := make([]RouteRule, 0, len(routesRaw))
	for _, routeRaw := range routesRaw {
		route, ok := routeRaw.(map[string]interface{})
		if !ok {
			continue
		}

		condition, ok := route["condition"].(map[string]interface{})
		if !ok {
			continue
		}

		routes = append(routes, RouteRule{
			Name: route["name"].(string),
			Condition: FilterCondition{
				Field:    condition["field"].(string),
				Operator: condition["operator"].(string),
				Value:    condition["value"],
			},
			OutputKey: route["outputKey"].(string),
		})
	}

	return &RouterProcessor{
		name:   name,
		config: config,
		routes: routes,
	}, nil
}

// Process 执行路由
func (p *RouterProcessor) Process(ctx context.Context, input []map[string]interface{}) ([]map[string]interface{}, error) {
	// 初始化输出分组
	outputs := make(map[string][]map[string]interface{})
	for _, route := range p.routes {
		outputs[route.OutputKey] = []map[string]interface{}{}
	}
	outputs["default"] = []map[string]interface{}{}

	for _, item := range input {
		// 检查 context
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		matched := false
		for _, route := range p.routes {
			if matchCondition(item, route.Condition) {
				outputs[route.OutputKey] = append(outputs[route.OutputKey], item)
				matched = true
				break
			}
		}

		if !matched {
			outputs["default"] = append(outputs["default"], item)
		}
	}

	// 合并所有输出
	var result []map[string]interface{}
	for key, items := range outputs {
		for _, item := range items {
			// 添加路由标记
			routed := make(map[string]interface{})
			for k, v := range item {
				routed[k] = v
			}
			routed["_route"] = key
			result = append(result, routed)
		}
	}

	return result, nil
}

// matchCondition 匹配条件
func matchCondition(item map[string]interface{}, cond FilterCondition) bool {
	value, exists := item[cond.Field]
	if !exists {
		return false
	}

	switch cond.Operator {
	case "equals", "eq":
		return fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
	case "not_equals", "ne":
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Value)
	case "contains":
		strVal := fmt.Sprintf("%v", value)
		strCond := fmt.Sprintf("%v", cond.Value)
		return strings.Contains(strVal, strCond)
	case "greater_than", "gt":
		return compareNumeric(value, cond.Value) > 0
	case "less_than", "lt":
		return compareNumeric(value, cond.Value) < 0
	default:
		return false
	}
}

// GetType 获取处理器类型
func (p *RouterProcessor) GetType() ProcessorType {
	return ProcessorTypeRouter
}

// GetName 获取处理器名称
func (p *RouterProcessor) GetName() string {
	return p.name
}

// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"fmt"
	"regexp"
	"strings"
)

// ConditionEvaluator 条件评估器.
type ConditionEvaluator struct{}

// NewConditionEvaluator 创建条件评估器.
func NewConditionEvaluator() *ConditionEvaluator {
	return &ConditionEvaluator{}
}

// Evaluate 评估条件表达式.
func (ce *ConditionEvaluator) Evaluate(expr *ConditionExpr, vars map[string]interface{}) *ConditionResult {
	if expr == nil {
		return &ConditionResult{Matched: true, Detail: "nil expression, always true"}
	}

	switch expr.Logic {
	case LogicAnd:
		return ce.evaluateAnd(expr.Children, vars)
	case LogicOr:
		return ce.evaluateOr(expr.Children, vars)
	case LogicNot:
		return ce.evaluateNot(expr.Children, vars)
	default:
		return ce.evaluateComparison(expr, vars)
	}
}

// evaluateAnd 评估 AND 逻辑.
func (ce *ConditionEvaluator) evaluateAnd(children []*ConditionExpr, vars map[string]interface{}) *ConditionResult {
	if len(children) == 0 {
		return &ConditionResult{Matched: true, Detail: "empty AND, always true"}
	}

	for i, child := range children {
		result := ce.Evaluate(child, vars)
		if !result.Matched {
			return &ConditionResult{
				Matched: false,
				Detail:  fmt.Sprintf("AND child %d failed: %s", i, result.Detail),
			}
		}
	}

	return &ConditionResult{Matched: true, Detail: "all AND conditions met"}
}

// evaluateOr 评估 OR 逻辑.
func (ce *ConditionEvaluator) evaluateOr(children []*ConditionExpr, vars map[string]interface{}) *ConditionResult {
	if len(children) == 0 {
		return &ConditionResult{Matched: false, Detail: "empty OR, always false"}
	}

	for _, child := range children {
		result := ce.Evaluate(child, vars)
		if result.Matched {
			return &ConditionResult{Matched: true, Detail: "OR condition met"}
		}
	}

	return &ConditionResult{Matched: false, Detail: "no OR conditions met"}
}

// evaluateNot 评估 NOT 逻辑.
func (ce *ConditionEvaluator) evaluateNot(children []*ConditionExpr, vars map[string]interface{}) *ConditionResult {
	if len(children) == 0 {
		return &ConditionResult{Matched: true, Detail: "empty NOT, always true"}
	}

	result := ce.Evaluate(children[0], vars)
	return &ConditionResult{
		Matched: !result.Matched,
		Detail:  fmt.Sprintf("NOT: %s", result.Detail),
	}
}

// evaluateComparison 评估比较条件.
func (ce *ConditionEvaluator) evaluateComparison(expr *ConditionExpr, vars map[string]interface{}) *ConditionResult {
	// 获取字段值
	fieldValue := ce.getFieldValue(expr.Field, vars)

	switch expr.Op {
	case OpEquals:
		return ce.compareEquals(fieldValue, expr.Value)
	case OpNotEquals:
		result := ce.compareEquals(fieldValue, expr.Value)
		result.Matched = !result.Matched
		result.Detail = fmt.Sprintf("NOT %s", result.Detail)
		return result
	case OpGreaterThan:
		return ce.compareNumeric(fieldValue, expr.Value, ">")
	case OpLessThan:
		return ce.compareNumeric(fieldValue, expr.Value, "<")
	case OpGreaterEqual:
		return ce.compareNumeric(fieldValue, expr.Value, ">=")
	case OpLessEqual:
		return ce.compareNumeric(fieldValue, expr.Value, "<=")
	case OpContains:
		return ce.compareContains(fieldValue, expr.Value)
	case OpStartsWith:
		return ce.compareStartsWith(fieldValue, expr.Value)
	case OpEndsWith:
		return ce.compareEndsWith(fieldValue, expr.Value)
	case OpMatches:
		return ce.compareRegex(fieldValue, expr.Value)
	case OpIn:
		return ce.compareIn(fieldValue, expr.Value)
	case OpExists:
		return &ConditionResult{
			Matched: fieldValue != nil,
			Detail:  fmt.Sprintf("field %s exists: %v", expr.Field, fieldValue != nil),
		}
	default:
		return &ConditionResult{
			Matched: false,
			Detail:  fmt.Sprintf("unknown operator: %s", expr.Op),
		}
	}
}

// getFieldValue 从变量中获取字段值.
func (ce *ConditionEvaluator) getFieldValue(field string, vars map[string]interface{}) interface{} {
	if vars == nil {
		return nil
	}

	// 支持嵌套字段（用 . 分隔）
	parts := strings.Split(field, ".")
	current := vars

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return nil
		}

		if i == len(parts)-1 {
			return val
		}

		// 尝试转换为 map
		if nextMap, ok := val.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil
		}
	}

	return nil
}

// compareEquals 比较相等.
func (ce *ConditionEvaluator) compareEquals(fieldValue, expected interface{}) *ConditionResult {
	if fieldValue == nil && expected == nil {
		return &ConditionResult{Matched: true, Detail: "both nil"}
	}
	if fieldValue == nil || expected == nil {
		return &ConditionResult{
			Matched: false,
			Detail:  fmt.Sprintf("field=%v, expected=%v", fieldValue, expected),
		}
	}

	// 转换为字符串比较
	fieldStr := fmt.Sprintf("%v", fieldValue)
	expectedStr := fmt.Sprintf("%v", expected)

	return &ConditionResult{
		Matched: fieldStr == expectedStr,
		Detail:  fmt.Sprintf("field=%s, expected=%s", fieldStr, expectedStr),
	}
}

// compareNumeric 数值比较.
func (ce *ConditionEvaluator) compareNumeric(fieldValue, expected interface{}, op string) *ConditionResult {
	fieldNum := toFloat64(fieldValue)
	expectedNum := toFloat64(expected)

	var matched bool
	switch op {
	case ">":
		matched = fieldNum > expectedNum
	case "<":
		matched = fieldNum < expectedNum
	case ">=":
		matched = fieldNum >= expectedNum
	case "<=":
		matched = fieldNum <= expectedNum
	}

	return &ConditionResult{
		Matched: matched,
		Detail:  fmt.Sprintf("%v %s %v", fieldNum, op, expectedNum),
	}
}

// compareContains 比较包含.
func (ce *ConditionEvaluator) compareContains(fieldValue, expected interface{}) *ConditionResult {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	expectedStr := fmt.Sprintf("%v", expected)

	return &ConditionResult{
		Matched: strings.Contains(fieldStr, expectedStr),
		Detail:  fmt.Sprintf("'%s' contains '%s': %v", fieldStr, expectedStr, strings.Contains(fieldStr, expectedStr)),
	}
}

// compareStartsWith 比较前缀.
func (ce *ConditionEvaluator) compareStartsWith(fieldValue, expected interface{}) *ConditionResult {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	expectedStr := fmt.Sprintf("%v", expected)

	return &ConditionResult{
		Matched: strings.HasPrefix(fieldStr, expectedStr),
		Detail:  fmt.Sprintf("'%s' starts with '%s': %v", fieldStr, expectedStr, strings.HasPrefix(fieldStr, expectedStr)),
	}
}

// compareEndsWith 比较后缀.
func (ce *ConditionEvaluator) compareEndsWith(fieldValue, expected interface{}) *ConditionResult {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	expectedStr := fmt.Sprintf("%v", expected)

	return &ConditionResult{
		Matched: strings.HasSuffix(fieldStr, expectedStr),
		Detail:  fmt.Sprintf("'%s' ends with '%s': %v", fieldStr, expectedStr, strings.HasSuffix(fieldStr, expectedStr)),
	}
}

// compareRegex 正则匹配.
func (ce *ConditionEvaluator) compareRegex(fieldValue, pattern interface{}) *ConditionResult {
	fieldStr := fmt.Sprintf("%v", fieldValue)
	patternStr := fmt.Sprintf("%v", pattern)

	matched, err := regexp.MatchString(patternStr, fieldStr)
	if err != nil {
		return &ConditionResult{
			Matched: false,
			Detail:  fmt.Sprintf("invalid regex '%s': %v", patternStr, err),
		}
	}

	return &ConditionResult{
		Matched: matched,
		Detail:  fmt.Sprintf("'%s' matches '%s': %v", fieldStr, patternStr, matched),
	}
}

// compareIn 判断值是否在列表中.
func (ce *ConditionEvaluator) compareIn(fieldValue, list interface{}) *ConditionResult {
	if listSlice, ok := list.([]interface{}); ok {
		fieldStr := fmt.Sprintf("%v", fieldValue)
		for _, item := range listSlice {
			if fmt.Sprintf("%v", item) == fieldStr {
				return &ConditionResult{
					Matched: true,
					Detail:  fmt.Sprintf("'%s' is in list", fieldStr),
				}
			}
		}
		return &ConditionResult{
			Matched: false,
			Detail:  fmt.Sprintf("'%s' is not in list", fieldStr),
		}
	}

	return &ConditionResult{
		Matched: false,
		Detail:  "list parameter is not an array",
	}
}

// toFloat64 将值转换为 float64.
func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case uint:
		return float64(n)
	case uint64:
		return float64(n)
	case string:
		var f float64
		fmt.Sscanf(n, "%f", &f)
		return f
	default:
		return 0
	}
}

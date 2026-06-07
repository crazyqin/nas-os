// Package workflow_automation 提供工作流自动化引擎
package workflow_automation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConditionEvaluator(t *testing.T) {
	ce := NewConditionEvaluator()
	assert.NotNil(t, ce)
}

func TestConditionEvaluateNil(t *testing.T) {
	ce := NewConditionEvaluator()
	result := ce.Evaluate(nil, nil)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateEquals(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{
		"status": "active",
		"count":  10,
	}

	// 字符串相等
	expr := &ConditionExpr{Op: OpEquals, Field: "status", Value: "active"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	// 字符串不相等
	expr = &ConditionExpr{Op: OpEquals, Field: "status", Value: "inactive"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)

	// 数值相等
	expr = &ConditionExpr{Op: OpEquals, Field: "count", Value: 10}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateNotEquals(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"status": "active"}

	expr := &ConditionExpr{Op: OpNotEquals, Field: "status", Value: "inactive"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpNotEquals, Field: "status", Value: "active"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateGreaterThan(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"count": 15}

	expr := &ConditionExpr{Op: OpGreaterThan, Field: "count", Value: 10}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpGreaterThan, Field: "count", Value: 20}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)

	expr = &ConditionExpr{Op: OpGreaterThan, Field: "count", Value: 15}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateLessThan(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"count": 5}

	expr := &ConditionExpr{Op: OpLessThan, Field: "count", Value: 10}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpLessThan, Field: "count", Value: 3}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateGreaterEqual(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"count": 10}

	expr := &ConditionExpr{Op: OpGreaterEqual, Field: "count", Value: 10}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpGreaterEqual, Field: "count", Value: 9}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpGreaterEqual, Field: "count", Value: 11}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateLessEqual(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"count": 10}

	expr := &ConditionExpr{Op: OpLessEqual, Field: "count", Value: 10}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpLessEqual, Field: "count", Value: 11}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpLessEqual, Field: "count", Value: 9}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateContains(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"name": "hello world"}

	expr := &ConditionExpr{Op: OpContains, Field: "name", Value: "world"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpContains, Field: "name", Value: "golang"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateStartsWith(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"path": "/home/user"}

	expr := &ConditionExpr{Op: OpStartsWith, Field: "path", Value: "/home"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpStartsWith, Field: "path", Value: "/var"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateEndsWith(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"file": "test.go"}

	expr := &ConditionExpr{Op: OpEndsWith, Field: "file", Value: ".go"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpEndsWith, Field: "file", Value: ".py"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateMatches(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"email": "test@example.com"}

	expr := &ConditionExpr{Op: OpMatches, Field: "email", Value: `^[a-z]+@[a-z]+\.[a-z]+$`}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpMatches, Field: "email", Value: `^\d+$`}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateIn(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"color": "red"}

	expr := &ConditionExpr{
		Op:    OpIn,
		Field: "color",
		Value: []interface{}{"red", "green", "blue"},
	}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{
		Op:    OpIn,
		Field: "color",
		Value: []interface{}{"yellow", "purple"},
	}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateExists(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"name": "test"}

	expr := &ConditionExpr{Op: OpExists, Field: "name"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpExists, Field: "missing"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateAnd(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{
		"status": "active",
		"count":  10,
	}

	// AND 全部为 true
	expr := &ConditionExpr{
		Logic: LogicAnd,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "active"},
			{Op: OpGreaterThan, Field: "count", Value: 5},
		},
	}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	// AND 有一个为 false
	expr = &ConditionExpr{
		Logic: LogicAnd,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "active"},
			{Op: OpGreaterThan, Field: "count", Value: 20},
		},
	}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)

	// 空 AND
	expr = &ConditionExpr{Logic: LogicAnd, Children: []*ConditionExpr{}}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateOr(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"status": "active"}

	// OR 有一个为 true
	expr := &ConditionExpr{
		Logic: LogicOr,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "active"},
			{Op: OpEquals, Field: "status", Value: "deleted"},
		},
	}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	// OR 全部为 false
	expr = &ConditionExpr{
		Logic: LogicOr,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "inactive"},
			{Op: OpEquals, Field: "status", Value: "deleted"},
		},
	}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)

	// 空 OR
	expr = &ConditionExpr{Logic: LogicOr, Children: []*ConditionExpr{}}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateNot(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"status": "active"}

	// NOT true -> false
	expr := &ConditionExpr{
		Logic: LogicNot,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "active"},
		},
	}
	result := ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)

	// NOT false -> true
	expr = &ConditionExpr{
		Logic: LogicNot,
		Children: []*ConditionExpr{
			{Op: OpEquals, Field: "status", Value: "inactive"},
		},
	}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	// 空 NOT
	expr = &ConditionExpr{Logic: LogicNot, Children: []*ConditionExpr{}}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateNested(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{
		"status": "active",
		"count":  15,
		"role":   "admin",
	}

	// 复杂嵌套: (status == "active" AND count > 10) OR role == "admin"
	expr := &ConditionExpr{
		Logic: LogicOr,
		Children: []*ConditionExpr{
			{
				Logic: LogicAnd,
				Children: []*ConditionExpr{
					{Op: OpEquals, Field: "status", Value: "active"},
					{Op: OpGreaterThan, Field: "count", Value: 10},
				},
			},
			{Op: OpEquals, Field: "role", Value: "admin"},
		},
	}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateNestedField(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"name": "Alice",
			"age":  30,
		},
	}

	expr := &ConditionExpr{Op: OpEquals, Field: "user.name", Value: "Alice"}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)

	expr = &ConditionExpr{Op: OpGreaterThan, Field: "user.age", Value: 25}
	result = ce.Evaluate(expr, vars)
	assert.True(t, result.Matched)
}

func TestConditionEvaluateMissingField(t *testing.T) {
	ce := NewConditionEvaluator()

	vars := map[string]interface{}{"status": "active"}

	expr := &ConditionExpr{Op: OpEquals, Field: "missing", Value: nil}
	result := ce.Evaluate(expr, vars)
	assert.True(t, result.Matched) // nil == nil

	expr = &ConditionExpr{Op: OpEquals, Field: "missing", Value: "test"}
	result = ce.Evaluate(expr, vars)
	assert.False(t, result.Matched)
}

func TestConditionEvaluateNilVars(t *testing.T) {
	ce := NewConditionEvaluator()

	expr := &ConditionExpr{Op: OpExists, Field: "anything"}
	result := ce.Evaluate(expr, nil)
	assert.False(t, result.Matched)
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
	}{
		{float64(3.14), 3.14},
		{float32(2.5), 2.5},
		{int(10), 10.0},
		{int64(100), 100.0},
		{int32(50), 50.0},
		{uint(20), 20.0},
		{uint64(200), 200.0},
		{"3.14", 3.14},
		{nil, 0},
		{"invalid", 0},
		{true, 0},
	}

	for _, tt := range tests {
		result := toFloat64(tt.input)
		assert.InDelta(t, tt.expected, result, 0.01, "input: %v", tt.input)
	}
}

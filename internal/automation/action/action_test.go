package action

import (
	"context"
	"testing"
)

func TestMoveAction_GetType(t *testing.T) {
	action := &MoveAction{Type: TypeMove}
	if action.GetType() != TypeMove {
		t.Errorf("expected %s, got %s", TypeMove, action.GetType())
	}
}

func TestCopyAction_GetType(t *testing.T) {
	action := &CopyAction{Type: TypeCopy}
	if action.GetType() != TypeCopy {
		t.Errorf("expected %s, got %s", TypeCopy, action.GetType())
	}
}

func TestDeleteAction_GetType(t *testing.T) {
	action := &DeleteAction{Type: TypeDelete}
	if action.GetType() != TypeDelete {
		t.Errorf("expected %s, got %s", TypeDelete, action.GetType())
	}
}

func TestRenameAction_GetType(t *testing.T) {
	action := &RenameAction{Type: TypeRename}
	if action.GetType() != TypeRename {
		t.Errorf("expected %s, got %s", TypeRename, action.GetType())
	}
}

func TestNotifyAction_GetType(t *testing.T) {
	action := &NotifyAction{Type: TypeNotify}
	if action.GetType() != TypeNotify {
		t.Errorf("expected %s, got %s", TypeNotify, action.GetType())
	}
}

func TestCommandAction_GetType(t *testing.T) {
	action := &CommandAction{Type: TypeCommand}
	if action.GetType() != TypeCommand {
		t.Errorf("expected %s, got %s", TypeCommand, action.GetType())
	}
}

func TestWebhookAction_GetType(t *testing.T) {
	action := &WebhookAction{Type: TypeWebhook}
	if action.GetType() != TypeWebhook {
		t.Errorf("expected %s, got %s", TypeWebhook, action.GetType())
	}
}

func TestEmailAction_GetType(t *testing.T) {
	action := &EmailAction{Type: TypeEmail}
	if action.GetType() != TypeEmail {
		t.Errorf("expected %s, got %s", TypeEmail, action.GetType())
	}
}

func TestConditionalAction_GetType(t *testing.T) {
	action := &ConditionalAction{Type: TypeConditional}
	if action.GetType() != TypeConditional {
		t.Errorf("expected %s, got %s", TypeConditional, action.GetType())
	}
}

func TestReplaceVariables(t *testing.T) {
	ctx := map[string]interface{}{
		"name": "test",
		"nested": map[string]interface{}{
			"value": "nested-value",
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"hello {{name}}", "hello test"},
		{"{{nested.value}}", "nested-value"},
		{"no variables", "no variables"},
		{"{{unknown}}", "{{unknown}}"},
	}

	for _, tt := range tests {
		result := replaceVariables(tt.input, ctx)
		if result != tt.expected {
			t.Errorf("replaceVariables(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestGetNestedValue(t *testing.T) {
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "deep",
			},
			"simple": "here",
		},
		"top": "level",
	}

	tests := []struct {
		path     string
		expected interface{}
	}{
		{"top", "level"},
		{"level1.simple", "here"},
		{"level1.level2.value", "deep"},
		{"nonexistent", nil},
		{"level1.nonexistent", nil},
	}

	for _, tt := range tests {
		result := getNestedValue(data, tt.path)
		if result != tt.expected {
			t.Errorf("getNestedValue(%s) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestCompareEquals(t *testing.T) {
	tests := []struct {
		a, b     interface{}
		expected bool
	}{
		{nil, nil, true},
		{nil, "value", false},
		{"test", "test", true},
		{"test", "other", false},
		{123, 123, true},
		{123, 456, false},
	}

	for _, tt := range tests {
		result := compareEquals(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("compareEquals(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestCheckContains(t *testing.T) {
	tests := []struct {
		field    interface{}
		value    interface{}
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{nil, "value", false},
		{12345, "34", true},
	}

	for _, tt := range tests {
		result, err := checkContains(tt.field, tt.value)
		if err != nil {
			t.Errorf("checkContains(%v, %v) error: %v", tt.field, tt.value, err)
		}
		if result != tt.expected {
			t.Errorf("checkContains(%v, %v) = %v, want %v", tt.field, tt.value, result, tt.expected)
		}
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected float64
		hasError bool
	}{
		{int(10), 10.0, false},
		{int64(20), 20.0, false},
		{float32(30.5), 30.5, false},
		{float64(40.5), 40.5, false},
		{"50.5", 50.5, false},
		{"invalid", 0, true},
		{struct{}{}, 0, true},
	}

	for _, tt := range tests {
		result, err := toFloat64(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("toFloat64(%v) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("toFloat64(%v) error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		}
	}
}

func TestNewActionFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    Config
		typeCheck Type
	}{
		{"move", Config{Type: TypeMove, Source: "/a", Destination: "/b"}, TypeMove},
		{"copy", Config{Type: TypeCopy, Source: "/a", Destination: "/b"}, TypeCopy},
		{"delete", Config{Type: TypeDelete, Path: "/tmp"}, TypeDelete},
		{"rename", Config{Type: TypeRename, Path: "/a", NewName: "b"}, TypeRename},
		{"notify", Config{Type: TypeNotify, Channel: "email"}, TypeNotify},
		{"command", Config{Type: TypeCommand, Command: "ls"}, TypeCommand},
		{"webhook", Config{Type: TypeWebhook, URL: "http://example.com"}, TypeWebhook},
		{"email", Config{Type: TypeEmail, To: "test@example.com"}, TypeEmail},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := NewActionFromConfig(tt.config)
			if err != nil {
				t.Fatalf("NewActionFromConfig error: %v", err)
			}
			if action.GetType() != tt.typeCheck {
				t.Errorf("expected type %s, got %s", tt.typeCheck, action.GetType())
			}
		})
	}
}

func TestCommandAction_Execute(t *testing.T) {
	action := &CommandAction{
		Type:    TypeCommand,
		Command: "echo",
		Args:    []string{"hello"},
	}

	err := action.Execute(context.Background(), nil)
	if err != nil {
		t.Errorf("CommandAction.Execute failed: %v", err)
	}
}

func TestConditionalAction_Evaluate(t *testing.T) {
	tests := []struct {
		name      string
		condition Condition
		context   map[string]interface{}
		expected  bool
	}{
		{
			name:      "equals true",
			condition: Condition{Field: "value", Operator: OperatorEquals, Value: "test"},
			context:   map[string]interface{}{"value": "test"},
			expected:  true,
		},
		{
			name:      "equals false",
			condition: Condition{Field: "value", Operator: OperatorEquals, Value: "other"},
			context:   map[string]interface{}{"value": "test"},
			expected:  false,
		},
		{
			name:      "not_equals",
			condition: Condition{Field: "value", Operator: OperatorNotEquals, Value: "test"},
			context:   map[string]interface{}{"value": "other"},
			expected:  true,
		},
		{
			name:      "exists true",
			condition: Condition{Field: "value", Operator: OperatorExists},
			context:   map[string]interface{}{"value": "something"},
			expected:  true,
		},
		{
			name:      "exists false",
			condition: Condition{Field: "missing", Operator: OperatorExists},
			context:   map[string]interface{}{"value": "something"},
			expected:  false,
		},
		{
			name:      "not_exists true",
			condition: Condition{Field: "missing", Operator: OperatorNotExists},
			context:   map[string]interface{}{"value": "something"},
			expected:  true,
		},
		{
			name:      "contains true",
			condition: Condition{Field: "text", Operator: OperatorContains, Value: "world"},
			context:   map[string]interface{}{"text": "hello world"},
			expected:  true,
		},
		{
			name:      "greater_than",
			condition: Condition{Field: "count", Operator: OperatorGreaterThan, Value: 5},
			context:   map[string]interface{}{"count": 10},
			expected:  true,
		},
		{
			name:      "less_than",
			condition: Condition{Field: "count", Operator: OperatorLessThan, Value: 10},
			context:   map[string]interface{}{"count": 5},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action := &ConditionalAction{Condition: tt.condition}
			result, err := action.evaluateCondition(tt.context)
			if err != nil {
				t.Fatalf("evaluateCondition error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSanitizeEmailHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal@email.com", "normal@email.com"},
		{"test\r\nBcc: attacker@evil.com", "testBcc: attacker@evil.com"},
		{"test\nInjected", "testInjected"},
		{"normal text", "normal text"},
	}

	for _, tt := range tests {
		result := sanitizeEmailHeader(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeEmailHeader(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

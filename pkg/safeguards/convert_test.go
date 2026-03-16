package safeguards

import (
	"math"
	"testing"
)

func TestSafeInt64ToUint64(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		expected  uint64
		wantError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 123, 123, false},
		{"max int64", math.MaxInt64, math.MaxInt64, false},
		{"negative", -1, 0, true},
		{"large negative", -1000, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeInt64ToUint64(tt.input)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name      string
		input     uint64
		expected  int64
		wantError bool
	}{
		{"zero", 0, 0, false},
		{"small value", 123, 123, false},
		{"max int64", math.MaxInt64, math.MaxInt64, false},
		{"overflow", math.MaxInt64 + 1, 0, true},
		{"max uint64", math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeUint64ToInt64(tt.input)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeIntToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int64
	}{
		{"zero", 0, 0},
		{"positive", 123, 123},
		{"negative", -456, -456},
		{"max int", math.MaxInt, int64(math.MaxInt)},
		{"min int", math.MinInt, int64(math.MinInt)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIntToInt64(tt.input)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestSafeInt64ToInt(t *testing.T) {
	tests := []struct {
		name      string
		input     int64
		expected  int
		wantError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 123, 123, false},
		{"negative", -456, -456, false},
		{"max int", int64(math.MaxInt), math.MaxInt, false},
		{"min int", int64(math.MinInt), math.MinInt, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeInt64ToInt(tt.input)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeUint64ToInt(t *testing.T) {
	tests := []struct {
		name      string
		input     uint64
		expected  int
		wantError bool
	}{
		{"zero", 0, 0, false},
		{"small value", 123, 123, false},
		{"max int", uint64(math.MaxInt), math.MaxInt, false},
		{"overflow", uint64(math.MaxInt) + 1, 0, true},
		{"max uint64", math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeUint64ToInt(tt.input)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeAddUint64(t *testing.T) {
	tests := []struct {
		name      string
		a, b      uint64
		expected  uint64
		wantError bool
	}{
		{"zero + zero", 0, 0, 0, false},
		{"small values", 100, 200, 300, false},
		{"large values", math.MaxUint64 - 100, 100, math.MaxUint64, false},
		{"overflow", math.MaxUint64, 1, 0, true},
		{"overflow large", math.MaxUint64 - 10, 20, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeAddUint64(tt.a, tt.b)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeMulUint64(t *testing.T) {
	tests := []struct {
		name      string
		a, b      uint64
		expected  uint64
		wantError bool
	}{
		{"zero * value", 0, 100, 0, false},
		{"value * zero", 100, 0, 0, false},
		{"small values", 10, 20, 200, false},
		{"large values", 1000000, 1000000, 1000000000000, false},
		{"overflow", math.MaxUint64, 2, 0, true},
		{"overflow exact", 2, math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeMulUint64(tt.a, tt.b)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestSafeSubUint64(t *testing.T) {
	tests := []struct {
		name      string
		a, b      uint64
		expected  uint64
		wantError bool
	}{
		{"zero - zero", 0, 0, 0, false},
		{"equal values", 100, 100, 0, false},
		{"normal subtraction", 100, 30, 70, false},
		{"max - max", math.MaxUint64, math.MaxUint64, 0, false},
		{"underflow", 10, 100, 0, true},
		{"underflow large", 0, math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeSubUint64(tt.a, tt.b)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("got %d, want %d", result, tt.expected)
				}
			}
		})
	}
}

func TestClampInt64(t *testing.T) {
	tests := []struct {
		name     string
		val      int64
		min, max int64
		expected int64
	}{
		{"within range", 50, 0, 100, 50},
		{"below min", -10, 0, 100, 0},
		{"above max", 150, 0, 100, 100},
		{"at min", 0, 0, 100, 0},
		{"at max", 100, 0, 100, 100},
		{"negative range", -50, -100, -10, -50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampInt64(tt.val, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestClampUint64(t *testing.T) {
	tests := []struct {
		name     string
		val      uint64
		min, max uint64
		expected uint64
	}{
		{"within range", 50, 0, 100, 50},
		{"below min", 0, 10, 100, 10},
		{"above max", 150, 0, 100, 100},
		{"at min", 10, 10, 100, 10},
		{"at max", 100, 0, 100, 100},
		{"large values", math.MaxUint64, 0, math.MaxUint64, math.MaxUint64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampUint64(tt.val, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("got %d, want %d", result, tt.expected)
			}
		})
	}
}
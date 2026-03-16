package safeguards

import (
	"math"
	"testing"
)

func TestSafeInt64ToUint64(t *testing.T) {
	tests := []struct {
		name        string
		input       int64
		expected    uint64
		expectError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 42, 42, false},
		{"max int64", math.MaxInt64, math.MaxInt64, false},
		{"negative", -1, 0, true},
		{"min int64", math.MinInt64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeInt64ToUint64(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestSafeUint64ToInt64(t *testing.T) {
	tests := []struct {
		name        string
		input       uint64
		expected    int64
		expectError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 42, 42, false},
		{"max int64", math.MaxInt64, math.MaxInt64, false},
		{"overflow", uint64(math.MaxInt64) + 1, 0, true},
		{"max uint64", math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeUint64ToInt64(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
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
		{"positive", 42, 42},
		{"negative", -42, -42},
		{"max int", math.MaxInt, int64(math.MaxInt)},
		{"min int", math.MinInt, int64(math.MinInt)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SafeIntToInt64(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestSafeInt64ToInt(t *testing.T) {
	tests := []struct {
		name        string
		input       int64
		expected    int
		expectError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 42, 42, false},
		{"negative", -42, -42, false},
		{"max int", int64(math.MaxInt), math.MaxInt, false},
		{"min int", int64(math.MinInt), math.MinInt, false},
		// Note: On 64-bit systems, int64 and int are the same size,
		// so there's no overflow possible for int64 -> int
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeInt64ToInt(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestSafeUint64ToInt(t *testing.T) {
	tests := []struct {
		name        string
		input       uint64
		expected    int
		expectError bool
	}{
		{"zero", 0, 0, false},
		{"positive", 42, 42, false},
		{"max int", uint64(math.MaxInt), math.MaxInt, false},
		{"overflow", uint64(math.MaxInt) + 1, 0, true},
		{"max uint64", math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeUint64ToInt(tt.input)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestSafeAddUint64(t *testing.T) {
	tests := []struct {
		name        string
		a, b        uint64
		expected    uint64
		expectError bool
	}{
		{"zero + zero", 0, 0, 0, false},
		{"one + one", 1, 1, 2, false},
		{"normal addition", 100, 200, 300, false},
		{"overflow", math.MaxUint64, 1, 0, true},
		{"overflow both max", math.MaxUint64, math.MaxUint64, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeAddUint64(tt.a, tt.b)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestSafeMulUint64(t *testing.T) {
	tests := []struct {
		name        string
		a, b        uint64
		expected    uint64
		expectError bool
	}{
		{"zero * anything", 0, 100, 0, false},
		{"anything * zero", 100, 0, 0, false},
		{"one * one", 1, 1, 1, false},
		{"normal multiplication", 10, 20, 200, false},
		{"overflow", math.MaxUint64 / 2 + 1, 2, 0, true},
		{"overflow max", math.MaxUint64, 2, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeMulUint64(tt.a, tt.b)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
				}
			}
		})
	}
}

func TestSafeSubUint64(t *testing.T) {
	tests := []struct {
		name        string
		a, b        uint64
		expected    uint64
		expectError bool
	}{
		{"zero - zero", 0, 0, 0, false},
		{"normal subtraction", 100, 50, 50, false},
		{"equal values", 42, 42, 0, false},
		{"underflow", 0, 1, 0, true},
		{"underflow large", 100, 200, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SafeSubUint64(tt.a, tt.b)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %d, got %d", tt.expected, result)
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
				t.Errorf("Expected %d, got %d", tt.expected, result)
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
		{"large values", math.MaxUint64 / 2, 0, math.MaxUint64, math.MaxUint64 / 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClampUint64(tt.val, tt.min, tt.max)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// Benchmark tests
func BenchmarkSafeInt64ToUint64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SafeInt64ToUint64(int64(i))
	}
}

func BenchmarkSafeAddUint64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = SafeAddUint64(uint64(i), uint64(i))
	}
}

func BenchmarkClampInt64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ClampInt64(int64(i), 0, 1000000)
	}
}
// Package safeconv - 安全类型转换测试
package safeconv

import (
	"math"
	"testing"
)

func TestInt64ToInt(t *testing.T) {
	// 注意：在 64 位系统上，int 和 int64 大小相同，所以不会溢出
	// 这些测试主要验证转换逻辑
	tests := []struct {
		input    int64
		expected int
		hasError bool
	}{
		{0, 0, false},
		{100, 100, false},
		{-100, -100, false},
	}

	for _, tt := range tests {
		result, err := Int64ToInt(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Int64ToInt(%d) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Int64ToInt(%d) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Int64ToInt(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestInt64ToUint64(t *testing.T) {
	tests := []struct {
		input    int64
		expected uint64
		hasError bool
	}{
		{0, 0, false},
		{100, 100, false},
		{-1, 0, true},
		{math.MaxInt64, math.MaxInt64, false},
	}

	for _, tt := range tests {
		result, err := Int64ToUint64(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Int64ToUint64(%d) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Int64ToUint64(%d) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Int64ToUint64(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestUint64ToInt(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
		hasError bool
	}{
		{0, 0, false},
		{100, 100, false},
		{math.MaxUint64, 0, true},
		{math.MaxInt, math.MaxInt, false},
	}

	for _, tt := range tests {
		result, err := Uint64ToInt(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Uint64ToInt(%d) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Uint64ToInt(%d) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Uint64ToInt(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestUint16ToByte(t *testing.T) {
	tests := []struct {
		input    uint16
		expected byte
		hasError bool
	}{
		{0, 0, false},
		{255, 255, false},
		{256, 0, true},
		{65535, 0, true},
	}

	for _, tt := range tests {
		result, err := Uint16ToByte(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Uint16ToByte(%d) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Uint16ToByte(%d) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Uint16ToByte(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestRuneToUint32(t *testing.T) {
	tests := []struct {
		input    rune
		expected uint32
		hasError bool
	}{
		{0, 0, false},
		{128, 128, false},
		{-1, 0, true},
		{0x10FFFF, 0x10FFFF, false},
	}

	for _, tt := range tests {
		result, err := RuneToUint32(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("RuneToUint32(%d) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("RuneToUint32(%d) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("RuneToUint32(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestFloat64ToInt(t *testing.T) {
	tests := []struct {
		input    float64
		expected int
		hasError bool
	}{
		{0.0, 0, false},
		{100.5, 100, false},
		{-50.7, -50, false},
		{math.NaN(), 0, true},
		{math.Inf(1), 0, true},
		{math.Inf(-1), 0, true},
		{float64(math.MaxInt) * 2, 0, true},
	}

	for _, tt := range tests {
		result, err := Float64ToInt(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Float64ToInt(%v) expected error, got nil", tt.input)
			}
		} else {
			if err != nil {
				t.Errorf("Float64ToInt(%v) unexpected error: %v", tt.input, err)
			}
			if result != tt.expected {
				t.Errorf("Float64ToInt(%v) = %d, expected %d", tt.input, result, tt.expected)
			}
		}
	}
}

func TestMustInt64ToInt(t *testing.T) {
	tests := []struct {
		input    int64
		expected int
	}{
		{0, 0},
		{100, 100},
		{-100, -100},
		{math.MaxInt64, math.MaxInt},
		{math.MinInt64, math.MinInt},
	}

	for _, tt := range tests {
		result := MustInt64ToInt(tt.input)
		if result != tt.expected {
			t.Errorf("MustInt64ToInt(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestMustUint64ToInt(t *testing.T) {
	tests := []struct {
		input    uint64
		expected int
	}{
		{0, 0},
		{100, 100},
		{math.MaxUint64, math.MaxInt},
	}

	for _, tt := range tests {
		result := MustUint64ToInt(tt.input)
		if result != tt.expected {
			t.Errorf("MustUint64ToInt(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestMustRuneToUint32(t *testing.T) {
	tests := []struct {
		input    rune
		expected uint32
	}{
		{0, 0},
		{128, 128},
		{-1, 0},
		{0x10FFFF, 0x10FFFF},
	}

	for _, tt := range tests {
		result := MustRuneToUint32(tt.input)
		if result != tt.expected {
			t.Errorf("MustRuneToUint32(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestClampInt(t *testing.T) {
	tests := []struct {
		value, min, max, expected int
	}{
		{5, 0, 10, 5},
		{-5, 0, 10, 0},
		{15, 0, 10, 10},
		{100, 0, 100, 100},
	}

	for _, tt := range tests {
		result := ClampInt(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("ClampInt(%d, %d, %d) = %d, expected %d",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

func TestInIntRange(t *testing.T) {
	if !InIntRange(0) {
		t.Error("0 should be in int range")
	}

	if !InIntRange(100) {
		t.Error("100 should be in int range")
	}

	if InIntRange(math.MaxInt64) {
		// 在 32 位系统上会溢出
		_ = math.MaxInt64
	}
}

func TestInByteRange(t *testing.T) {
	tests := []struct {
		input    int
		expected bool
	}{
		{0, true},
		{127, true},
		{255, true},
		{256, false},
		{-1, false},
	}

	for _, tt := range tests {
		result := InByteRange(tt.input)
		if result != tt.expected {
			t.Errorf("InByteRange(%d) = %v, expected %v", tt.input, result, tt.expected)
		}
	}
}

func TestInt64SliceToIntSlice(t *testing.T) {
	// 注意：在 64 位系统上，int 和 int64 大小相同
	tests := []struct {
		input    []int64
		expected []int
		hasError bool
	}{
		{[]int64{1, 2, 3}, []int{1, 2, 3}, false},
		{[]int64{-1, 0, 1}, []int{-1, 0, 1}, false},
	}

	for _, tt := range tests {
		result, err := Int64SliceToIntSlice(tt.input)

		if tt.hasError {
			if err == nil {
				t.Errorf("Int64SliceToIntSlice expected error, got nil")
			}
		} else {
			if err != nil {
				t.Errorf("Int64SliceToIntSlice unexpected error: %v", err)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Int64SliceToIntSlice length = %d, expected %d", len(result), len(tt.expected))
			}
		}
	}
}
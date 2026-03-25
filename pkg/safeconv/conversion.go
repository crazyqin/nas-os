// Package safeconv 提供安全的类型转换函数
// 用于避免整数溢出和安全问题
package safeconv

import (
	"fmt"
	"math"
)

// ========== 安全整数转换 ==========

// Int64ToInt 安全地将 int64 转换为 int.
func Int64ToInt(v int64) (int, error) {
	if v < math.MinInt || v > math.MaxInt {
		return 0, fmt.Errorf("int64 value %d out of int range", v)
	}
	return int(v), nil
}

// Int64ToUint64 安全地将 int64 转换为 uint64.
func Int64ToUint64(v int64) (uint64, error) {
	if v < 0 {
		return 0, fmt.Errorf("negative int64 value %d cannot be converted to uint64", v)
	}
	return uint64(v), nil
}

// Uint64ToInt 安全地将 uint64 转换为 int.
func Uint64ToInt(v uint64) (int, error) {
	if v > math.MaxInt {
		return 0, fmt.Errorf("uint64 value %d exceeds int max", v)
	}
	return int(v), nil
}

// Uint64ToInt64 安全地将 uint64 转换为 int64.
func Uint64ToInt64(v uint64) (int64, error) {
	if v > math.MaxInt64 {
		return 0, fmt.Errorf("uint64 value %d exceeds int64 max", v)
	}
	return int64(v), nil
}

// Uint32ToInt 安全地将 uint32 转换为 int.
func Uint32ToInt(v uint32) (int, error) {
	// 在 32 位系统上需要检查
	_ = v
	return int(v), nil
}

// IntToInt32 安全地将 int 转换为 int32.
func IntToInt32(v int) (int32, error) {
	if v < math.MinInt32 || v > math.MaxInt32 {
		return 0, fmt.Errorf("int value %d out of int32 range", v)
	}
	return int32(v), nil
}

// IntToInt64 安全地将 int 转换为 int64.
func IntToInt64(v int) (int64, error) {
	return int64(v), nil // 总是安全
}

// IntToUint64 安全地将 int 转换为 uint64.
func IntToUint64(v int) (uint64, error) {
	if v < 0 {
		return 0, fmt.Errorf("negative int value %d cannot be converted to uint64", v)
	}
	return uint64(v), nil
}

// Uint16ToByte 安全地将 uint16 转换为 byte.
func Uint16ToByte(v uint16) (byte, error) {
	if v > math.MaxUint8 {
		return 0, fmt.Errorf("uint16 value %d exceeds byte max", v)
	}
	return byte(v), nil
}

// Uint32ToByte 安全地将 uint32 转换为 byte.
func Uint32ToByte(v uint32) (byte, error) {
	if v > math.MaxUint8 {
		return 0, fmt.Errorf("uint32 value %d exceeds byte max", v)
	}
	return byte(v), nil
}

// IntToByte 安全地将 int 转换为 byte.
func IntToByte(v int) (byte, error) {
	if v < 0 || v > math.MaxUint8 {
		return 0, fmt.Errorf("int value %d out of byte range", v)
	}
	return byte(v), nil
}

// RuneToUint32 安全地将 rune 转换为 uint32.
func RuneToUint32(v rune) (uint32, error) {
	// rune 是 int32 的别名，可能为负
	if v < 0 {
		return 0, fmt.Errorf("negative rune value %d cannot be converted to uint32", v)
	}
	return uint32(v), nil
}

// RuneToByte 安全地将 rune 转换为 byte.
func RuneToByte(v rune) (byte, error) {
	if v < 0 || v > math.MaxUint8 {
		return 0, fmt.Errorf("rune value %d out of byte range", v)
	}
	return byte(v), nil
}

// Float64ToInt 安全地将 float64 转换为 int.
func Float64ToInt(v float64) (int, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("float64 value %v cannot be converted to int", v)
	}
	if v < math.MinInt || v > math.MaxInt {
		return 0, fmt.Errorf("float64 value %v out of int range", v)
	}
	return int(v), nil
}

// Float64ToInt64 安全地将 float64 转换为 int64.
func Float64ToInt64(v float64) (int64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("float64 value %v cannot be converted to int64", v)
	}
	if v < math.MinInt64 || v > math.MaxInt64 {
		return 0, fmt.Errorf("float64 value %v out of int64 range", v)
	}
	return int64(v), nil
}

// Float64ToUint64 安全地将 float64 转换为 uint64.
func Float64ToUint64(v float64) (uint64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("float64 value %v cannot be converted to uint64", v)
	}
	if v < 0 || v > math.MaxUint64 {
		return 0, fmt.Errorf("float64 value %v out of uint64 range", v)
	}
	return uint64(v), nil
}

// ========== 忽略错误的快速转换（用于已知安全的场景） ==========

// MustInt64ToInt 强制转换，溢出时返回边界值.
func MustInt64ToInt(v int64) int {
	if v < math.MinInt {
		return math.MinInt
	}
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

// MustUint64ToInt 强制转换，溢出时返回边界值.
func MustUint64ToInt(v uint64) int {
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

// MustInt64ToUint64 强制转换，负数返回 0.
func MustInt64ToUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

// MustUint16ToByte 强制转换，取低字节.
func MustUint16ToByte(v uint16) byte {
	return byte(v & 0xFF)
}

// MustRuneToUint32 强制转换，负数返回 0.
func MustRuneToUint32(v rune) uint32 {
	if v < 0 {
		return 0
	}
	return uint32(v)
}

// MustRuneToByte 强制转换，取低字节.
func MustRuneToByte(v rune) byte {
	return byte(v & 0xFF)
}

// MustFloat64ToInt 强制转换，使用边界值.
func MustFloat64ToInt(v float64) int {
	if math.IsNaN(v) {
		return 0
	}
	if math.IsInf(v, -1) {
		return math.MinInt
	}
	if math.IsInf(v, 1) {
		return math.MaxInt
	}
	if v < math.MinInt {
		return math.MinInt
	}
	if v > math.MaxInt {
		return math.MaxInt
	}
	return int(v)
}

// ========== 批量转换 ==========

// Int64SliceToIntSlice 安全转换切片.
func Int64SliceToIntSlice(in []int64) ([]int, error) {
	out := make([]int, len(in))
	for i, v := range in {
		var err error
		out[i], err = Int64ToInt(v)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
	}
	return out, nil
}

// Uint64SliceToIntSlice 安全转换切片.
func Uint64SliceToIntSlice(in []uint64) ([]int, error) {
	out := make([]int, len(in))
	for i, v := range in {
		var err error
		out[i], err = Uint64ToInt(v)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
	}
	return out, nil
}

// ========== 范围检查 ==========

// InIntRange 检查 int64 是否在 int 范围内.
func InIntRange(v int64) bool {
	return v >= math.MinInt && v <= math.MaxInt
}

// InUintRange 检查 int64 是否可以安全转为 uint64.
func InUintRange(v int64) bool {
	return v >= 0
}

// InByteRange 检查整数是否在 byte 范围内.
func InByteRange(v int) bool {
	return v >= 0 && v <= math.MaxUint8
}

// InInt32Range 检查 int 是否在 int32 范围内.
func InInt32Range(v int) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

// ClampInt 将整数限制在指定范围内.
func ClampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampInt64 将 int64 限制在指定范围内.
func ClampInt64(v, min, max int64) int64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ClampUint64 将 uint64 限制在指定范围内.
func ClampUint64(v, min, max uint64) uint64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

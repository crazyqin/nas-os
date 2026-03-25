// Package safeguards provides integer overflow protection
package safeguards

import (
	"fmt"
	"math"
)

// SafeInt64ToUint64 safely converts int64 to uint64.
func SafeInt64ToUint64(val int64) (uint64, error) {
	if val < 0 {
		return 0, fmt.Errorf("negative value %d cannot be converted to uint64", val)
	}
	return uint64(val), nil
}

// SafeUint64ToInt64 safely converts uint64 to int64.
func SafeUint64ToInt64(val uint64) (int64, error) {
	if val > math.MaxInt64 {
		return 0, fmt.Errorf("value %d overflows int64 (max %d)", val, int64(math.MaxInt64))
	}
	return int64(val), nil
}

// SafeIntToInt64 safely converts int to int64 (always safe on 64-bit).
func SafeIntToInt64(val int) int64 {
	return int64(val)
}

// SafeInt64ToInt safely converts int64 to int.
func SafeInt64ToInt(val int64) (int, error) {
	if int64(math.MaxInt) < val || val < int64(math.MinInt) {
		return 0, fmt.Errorf("value %d overflows int", val)
	}
	return int(val), nil
}

// SafeUint64ToInt safely converts uint64 to int.
func SafeUint64ToInt(val uint64) (int, error) {
	if val > uint64(math.MaxInt) {
		return 0, fmt.Errorf("value %d overflows int", val)
	}
	return int(val), nil
}

// SafeAddUint64 safely adds two uint64 values with overflow check.
func SafeAddUint64(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, fmt.Errorf("addition overflow: %d + %d", a, b)
	}
	return a + b, nil
}

// SafeMulUint64 safely multiplies two uint64 values with overflow check.
func SafeMulUint64(a, b uint64) (uint64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a > math.MaxUint64/b {
		return 0, fmt.Errorf("multiplication overflow: %d * %d", a, b)
	}
	return a * b, nil
}

// SafeSubUint64 safely subtracts two uint64 values with underflow check.
func SafeSubUint64(a, b uint64) (uint64, error) {
	if b > a {
		return 0, fmt.Errorf("subtraction underflow: %d - %d", a, b)
	}
	return a - b, nil
}

// ClampInt64 clamps an int64 value to a range.
func ClampInt64(val, min, max int64) int64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// ClampUint64 clamps a uint64 value to a range.
func ClampUint64(val, min, max uint64) uint64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// Package reports 提供报表生成和管理功能
package reports

import "math"

// round 辅助函数：四舍五入到指定小数位。
func round(value float64, places int) float64 {
	multiplier := math.Pow(10, float64(places))
	return math.Round(value*multiplier) / multiplier
}

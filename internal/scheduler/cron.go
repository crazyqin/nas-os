package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CronExpression Cron 表达式解析器
type CronExpression struct {
	second     field
	minute     field
	hour       field
	dayOfMonth field
	month      field
	dayOfWeek  field
	year       field
	location   *time.Location
}

type field struct {
	values map[int]bool
}

// CronParseOptions 解析选项
type CronParseOptions struct {
	Second    bool // 是否包含秒字段
	Location  *time.Location
	AllowYear bool // 是否允许年字段
}

// NewCronExpression 创建 Cron 表达式
func NewCronExpression(expression string, opts ...CronParseOptions) (*CronExpression, error) {
	options := CronParseOptions{
		Second:   true,
		Location: time.Local,
	}
	if len(opts) > 0 {
		options = opts[0]
	}

	cron := &CronExpression{
		location: options.Location,
	}

	fields := strings.Fields(expression)
	if len(fields) < 5 {
		return nil, fmt.Errorf("无效的 Cron 表达式: %s", expression)
	}

	var err error
	offset := 0

	// 如果包含秒字段
	if options.Second {
		if len(fields) < 6 {
			return nil, fmt.Errorf("缺少秒字段: %s", expression)
		}
		cron.second, err = parseField(fields[0], 0, 59)
		if err != nil {
			return nil, fmt.Errorf("秒字段错误: %w", err)
		}
		offset = 1
	} else {
		cron.second = newField(0) // 默认 0 秒
	}

	// 分钟
	cron.minute, err = parseField(fields[offset], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("分钟字段错误: %w", err)
	}

	// 小时
	cron.hour, err = parseField(fields[offset+1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("小时字段错误: %w", err)
	}

	// 日
	cron.dayOfMonth, err = parseField(fields[offset+2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("日字段错误: %w", err)
	}

	// 月
	cron.month, err = parseField(fields[offset+3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("月字段错误: %w", err)
	}

	// 星期
	dayOfWeekField := fields[offset+4]
	// 处理星期字段的特殊值
	if dayOfWeekField == "7" {
		dayOfWeekField = "0"
	}
	cron.dayOfWeek, err = parseField(dayOfWeekField, 0, 6)
	if err != nil {
		return nil, fmt.Errorf("星期字段错误: %w", err)
	}

	// 年（可选）
	if options.AllowYear && len(fields) > offset+5 {
		cron.year, err = parseField(fields[offset+5], 1970, 2099)
		if err != nil {
			return nil, fmt.Errorf("年字段错误: %w", err)
		}
	} else {
		cron.year = newFieldAll(1970, 2099)
	}

	return cron, nil
}

// newField 创建字段
func newField(value int) field {
	f := field{values: make(map[int]bool)}
	f.values[value] = true
	return f
}

// newFieldAll 创建包含所有值的字段
func newFieldAll(min, max int) field {
	f := field{values: make(map[int]bool)}
	for i := min; i <= max; i++ {
		f.values[i] = true
	}
	return f
}

// parseField 解析字段
func parseField(s string, min, max int) (field, error) {
	f := field{values: make(map[int]bool)}

	// 处理逗号分隔的列表
	parts := strings.Split(s, ",")
	for _, part := range parts {
		if err := parseFieldPart(part, min, max, &f); err != nil {
			return f, err
		}
	}

	return f, nil
}

// parseFieldPart 解析字段部分
func parseFieldPart(s string, min, max int, f *field) error {
	// 处理特殊值
	switch s {
	case "*":
		for i := min; i <= max; i++ {
			f.values[i] = true
		}
		return nil
	case "?":
		// ? 表示不指定值，等同于 *
		for i := min; i <= max; i++ {
			f.values[i] = true
		}
		return nil
	}

	// 处理步长
	if strings.Contains(s, "/") {
		return parseStep(s, min, max, f)
	}

	// 处理范围
	if strings.Contains(s, "-") {
		return parseRange(s, min, max, f)
	}

	// 处理单个值
	value, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("无效的值: %s", s)
	}

	if value < min || value > max {
		return fmt.Errorf("值 %d 超出范围 [%d, %d]", value, min, max)
	}

	f.values[value] = true
	return nil
}

// parseStep 解析步长
func parseStep(s string, min, max int, f *field) error {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return fmt.Errorf("无效的步长格式: %s", s)
	}

	var start, end int

	// 解析范围
	if parts[0] == "*" {
		start = min
		end = max
	} else if strings.Contains(parts[0], "-") {
		rangeParts := strings.Split(parts[0], "-")
		if len(rangeParts) != 2 {
			return fmt.Errorf("无效的范围格式: %s", parts[0])
		}
		var err error
		start, err = strconv.Atoi(rangeParts[0])
		if err != nil {
			return err
		}
		end, err = strconv.Atoi(rangeParts[1])
		if err != nil {
			return err
		}
	} else {
		var err error
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return err
		}
		end = max
	}

	// 解析步长
	step, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	if step <= 0 {
		return fmt.Errorf("步长必须大于 0")
	}

	for i := start; i <= end; i += step {
		if i >= min && i <= max {
			f.values[i] = true
		}
	}

	return nil
}

// parseRange 解析范围
func parseRange(s string, min, max int, f *field) error {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return fmt.Errorf("无效的范围格式: %s", s)
	}

	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	if start > end {
		return fmt.Errorf("范围起始值大于结束值: %s", s)
	}

	for i := start; i <= end; i++ {
		if i >= min && i <= max {
			f.values[i] = true
		}
	}

	return nil
}

// Next 计算下次执行时间
func (c *CronExpression) Next(t time.Time) time.Time {
	// 转换到指定时区
	if c.location != nil {
		t = t.In(c.location)
	}

	// 从下一秒开始
	t = t.Add(time.Second).Truncate(time.Second)

	// 最多尝试 5 年
	for i := 0; i < 5*365*24*60*60; i++ {
		if c.match(t) {
			return t
		}
		t = t.Add(time.Second)
	}

	return time.Time{} // 没有找到匹配的时间
}

// Prev 计算上次执行时间
func (c *CronExpression) Prev(t time.Time) time.Time {
	if c.location != nil {
		t = t.In(c.location)
	}
	t = t.Add(-time.Second).Truncate(time.Second)

	for i := 0; i < 5*365*24*60*60; i++ {
		if c.match(t) {
			return t
		}
		t = t.Add(-time.Second)
	}

	return time.Time{}
}

// match 检查时间是否匹配
func (c *CronExpression) match(t time.Time) bool {
	return c.second.values[t.Second()] &&
		c.minute.values[t.Minute()] &&
		c.hour.values[t.Hour()] &&
		c.dayOfMonth.values[t.Day()] &&
		c.month.values[int(t.Month())] &&
		c.dayOfWeek.values[int(t.Weekday())] &&
		c.year.values[t.Year()]
}

// NextN 计算接下来 N 次执行时间
func (c *CronExpression) NextN(t time.Time, n int) []time.Time {
	result := make([]time.Time, 0, n)
	next := t

	for i := 0; i < n; i++ {
		next = c.Next(next)
		if next.IsZero() {
			break
		}
		result = append(result, next)
	}

	return result
}

// String 返回表达式字符串
func (c *CronExpression) String() string {
	return fmt.Sprintf("CronExpression(second=%v, minute=%v, hour=%v, day=%v, month=%v, weekday=%v)",
		getKeys(c.second.values),
		getKeys(c.minute.values),
		getKeys(c.hour.values),
		getKeys(c.dayOfMonth.values),
		getKeys(c.month.values),
		getKeys(c.dayOfWeek.values))
}

func getKeys(m map[int]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// IsValidCron validates if a cron expression is valid
func IsValidCron(expression string, withSecond bool) bool {
	opts := CronParseOptions{Second: withSecond}
	_, err := NewCronExpression(expression, opts)
	return err == nil
}

// CronPresets defines common cron expression presets
var CronPresets = map[string]string{
	"every_minute":       "0 * * * *",
	"every_hour":         "0 0 * * *",
	"every_day":          "0 0 0 * *",
	"every_week":         "0 0 0 * 0",
	"every_month":        "0 0 0 1 *",
	"every_year":         "0 0 0 1 1 *",
	"every_5_minutes":    "0 */5 * * * *",
	"every_15_minutes":   "0 */15 * * * *",
	"every_30_minutes":   "0 */30 * * * *",
	"every_2_hours":      "0 0 */2 * * *",
	"every_6_hours":      "0 0 */6 * * *",
	"every_12_hours":     "0 0 */12 * * *",
	"every_weekday":      "0 0 0 * * 1-5",
	"every_weekend":      "0 0 0 * * 0,6",
	"first_day_of_month": "0 0 0 1 * *",
	"last_day_of_month":  "0 0 0 L * *",
}

// GetPreset 获取预设表达式
func GetPreset(name string) (string, bool) {
	expr, ok := CronPresets[name]
	return expr, ok
}

// DescribeCron 描述 Cron 表达式
func DescribeCron(expression string) string {
	// 简单的描述生成
	parts := strings.Fields(expression)
	if len(parts) < 5 {
		return "无效的表达式"
	}

	// 检查是否是预设
	for name, preset := range CronPresets {
		if expression == preset || expression == "0 "+preset {
			return fmt.Sprintf("预设: %s", name)
		}
	}

	// 生成简单描述
	desc := "每"

	if parts[0] != "*" && parts[0] != "?" {
		desc += fmt.Sprintf(" %s 秒", parts[0])
	}
	if parts[1] != "*" && parts[1] != "?" {
		desc += fmt.Sprintf(" %s 分钟", parts[1])
	}
	if parts[2] != "*" && parts[2] != "?" {
		desc += fmt.Sprintf(" %s 小时", parts[2])
	}
	if parts[3] != "*" && parts[3] != "?" {
		desc += fmt.Sprintf(" %s 日", parts[3])
	}
	if parts[4] != "*" && parts[4] != "?" {
		desc += fmt.Sprintf(" %s 月", parts[4])
	}

	return desc
}

// ParseDuration 解析持续时间字符串
func ParseDuration(s string) (time.Duration, error) {
	// 支持格式: 1s, 1m, 1h, 1d, 1w
	if len(s) == 0 {
		return 0, fmt.Errorf("空的持续时间")
	}

	// 尝试标准 Go duration 格式
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// 解析自定义格式
	unit := s[len(s)-1:]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("无效的持续时间: %s", s)
	}

	switch unit {
	case "s":
		return time.Duration(value) * time.Second, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("未知的时间单位: %s", unit)
	}
}

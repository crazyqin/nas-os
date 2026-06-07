package datawarehouse

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// NewRingBuffer 创建环形缓冲区
func NewRingBuffer(capacity int) *RingBuffer {
	return &RingBuffer{
		data:     make([]DataPoint, capacity),
		capacity: capacity,
		head:     0,
		tail:     0,
		size:     0,
	}
}

// Push 推入数据点
func (rb *RingBuffer) Push(dp DataPoint) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.data[rb.head] = dp
	rb.head = (rb.head + 1) % rb.capacity
	if rb.size < rb.capacity {
		rb.size++
	} else {
		rb.tail = (rb.tail + 1) % rb.capacity
	}
}

// Range 查询时间范围内的数据
func (rb *RingBuffer) Range(start, end time.Time) []DataPoint {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]DataPoint, 0)
	for i := 0; i < rb.size; i++ {
		idx := (rb.tail + i) % rb.capacity
		dp := rb.data[idx]
		if !dp.Timestamp.Before(start) && !dp.Timestamp.After(end) {
			result = append(result, dp)
		}
	}
	return result
}

// Size 返回当前大小
func (rb *RingBuffer) Size() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.size
}

// NewTimeSeries 创建时间序列
func NewTimeSeries(maxAge time.Duration) *TimeSeries {
	return &TimeSeries{
		series: make(map[string]*RingBuffer),
		maxAge: maxAge,
	}
}

// Add 添加数据点到时间序列
func (ts *TimeSeries) Add(metric string, dp DataPoint) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, ok := ts.series[metric]; !ok {
		ts.series[metric] = NewRingBuffer(86400) // 默认1天
	}
	ts.series[metric].Push(dp)
}

// Query 查询时间序列
func (ts *TimeSeries) Query(metric string, start, end time.Time) []DataPoint {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if rb, ok := ts.series[metric]; ok {
		return rb.Range(start, end)
	}
	return nil
}

// Metrics 返回所有指标名称
func (ts *TimeSeries) Metrics() []string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	metrics := make([]string, 0, len(ts.series))
	for k := range ts.series {
		metrics = append(metrics, k)
	}
	return metrics
}

// NewCube 创建数据立方体
func NewCube(dimensions []Dimension, measures []Measure) *Cube {
	return &Cube{
		dimensions: dimensions,
		measures:   measures,
		cells:      make(map[string]map[string]float64),
	}
}

// Add 添加数据到立方体
func (c *Cube) Add(dp DataPoint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.buildKey(dp.Dimensions)
	if _, ok := c.cells[key]; !ok {
		c.cells[key] = make(map[string]float64)
		for _, m := range c.measures {
			c.cells[key][m.Name] = 0
		}
	}

	for name, value := range dp.Measures {
		c.cells[key][name] += value
	}
}

// Slice 切片查询
func (c *Cube) Slice(dimension string, value string) []CubeCell {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]CubeCell, 0)
	for key, measures := range c.cells {
		dims := c.parseKey(key)
		if dims[dimension] == value {
			result = append(result, CubeCell{
				Dimensions: dims,
				Measures:   measures,
			})
		}
	}
	return result
}

// Dice 切块查询
func (c *Cube) Dice(filters map[string]string) []CubeCell {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]CubeCell, 0)
	for key, measures := range c.cells {
		dims := c.parseKey(key)
		match := true
		for k, v := range filters {
			if dims[k] != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, CubeCell{
				Dimensions: dims,
				Measures:   measures,
			})
		}
	}
	return result
}

// buildKey 构建维度key
func (c *Cube) buildKey(dims map[string]string) string {
	keys := make([]string, 0, len(dims))
	for k := range dims {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	key := ""
	for _, k := range keys {
		key += fmt.Sprintf("%s=%s;", k, dims[k])
	}
	return key
}

// parseKey 解析维度key
func (c *Cube) parseKey(key string) map[string]string {
	result := make(map[string]string)
	pairs := splitString(key, ";")
	for _, pair := range pairs {
		kv := splitString(pair, "=")
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func splitString(s, sep string) []string {
	result := make([]string, 0)
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			if i > start {
				result = append(result, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

// NewWarehouse 创建数据仓库
func NewWarehouse(maxPoints int) *Warehouse {
	if maxPoints <= 0 {
		maxPoints = 86400 // 默认1天每秒一条
	}
	return &Warehouse{
		ringBuffers: make(map[DataSource]*RingBuffer),
		cubes:       make(map[string]*Cube),
		timeSeries:  NewTimeSeries(24 * time.Hour),
		maxPoints:   maxPoints,
	}
}

// Ingest 数据采集
func (w *Warehouse) Ingest(dp DataPoint) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 初始化ring buffer
	if _, ok := w.ringBuffers[dp.Source]; !ok {
		w.ringBuffers[dp.Source] = NewRingBuffer(w.maxPoints)
	}
	w.ringBuffers[dp.Source].Push(dp)

	// 添加到时间序列
	for name, value := range dp.Measures {
		w.timeSeries.Add(name, DataPoint{
			Timestamp:  dp.Timestamp,
			Source:     dp.Source,
			Dimensions: dp.Dimensions,
			Measures:   map[string]float64{name: value},
		})
	}

	// 添加到数据立方体
	cubeKey := string(dp.Source)
	if _, ok := w.cubes[cubeKey]; !ok {
		dims := make([]Dimension, 0)
		for k := range dp.Dimensions {
			dims = append(dims, Dimension{Name: k})
		}
		measures := make([]Measure, 0)
		for k := range dp.Measures {
			measures = append(measures, Measure{Name: k, Type: AggSum})
		}
		w.cubes[cubeKey] = NewCube(dims, measures)
	}
	w.cubes[cubeKey].Add(dp)
}

// Query 执行OLAP查询
func (w *Warehouse) Query(q Query) (*QueryResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// 获取数据源的ring buffer
	rb, ok := w.ringBuffers[q.Source]
	if !ok {
		return &QueryResult{
			Columns: []string{},
			Rows:    [][]interface{}{},
			Total:   0,
		}, nil
	}

	// 时间范围查询
	points := rb.Range(q.StartTime, q.EndTime)

	// 应用过滤器
	filtered := make([]DataPoint, 0)
	for _, dp := range points {
		match := true
		for k, v := range q.Filters {
			if dp.Dimensions[k] != v {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, dp)
		}
	}

	// 分组聚合
	groups := make(map[string][]DataPoint)
	for _, dp := range filtered {
		groupKey := ""
		for _, dim := range q.GroupBy {
			groupKey += dp.Dimensions[dim] + "|"
		}
		groups[groupKey] = append(groups[groupKey], dp)
	}

	// 构建结果
	result := &QueryResult{
		Columns: q.GroupBy,
		Total:   len(groups),
	}

	for _, group := range groups {
		row := make([]interface{}, 0)
		for _, dim := range q.GroupBy {
			row = append(row, group[0].Dimensions[dim])
		}

		// 计算聚合值
		for _, m := range q.Measures {
			agg := w.aggregate(group, m)
			row = append(row, agg)
			if !contains(result.Columns, m.Name) {
				result.Columns = append(result.Columns, m.Name)
			}
		}

		result.Rows = append(result.Rows, row)
	}

	// 排序
	if q.OrderBy != "" && len(result.Rows) > 0 {
		sort.Slice(result.Rows, func(i, j int) bool {
			idx := indexOf(result.Columns, q.OrderBy)
			if idx < 0 {
				return false
			}
			vi, ok1 := result.Rows[i][idx].(float64)
			vj, ok2 := result.Rows[j][idx].(float64)
			if !ok1 || !ok2 {
				return false
			}
			return vi < vj
		})
	}

	// 限制返回数量
	if q.Limit > 0 && len(result.Rows) > q.Limit {
		result.Rows = result.Rows[:q.Limit]
	}

	return result, nil
}

// aggregate 计算聚合值
func (w *Warehouse) aggregate(points []DataPoint, m Measure) float64 {
	if len(points) == 0 {
		return 0
	}

	values := make([]float64, 0, len(points))
	for _, dp := range points {
		if v, ok := dp.Measures[m.Name]; ok {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return 0
	}

	switch m.Type {
	case AggSum:
		return sum(values)
	case AggAvg:
		return sum(values) / float64(len(values))
	case AggMax:
		return max(values)
	case AggMin:
		return min(values)
	case AggCount:
		return float64(len(values))
	case AggP95:
		return percentile(values, 0.95)
	case AggP99:
		return percentile(values, 0.99)
	default:
		return sum(values)
	}
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

func max(values []float64) float64 {
	m := math.Inf(-1)
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

func min(values []float64) float64 {
	m := math.Inf(1)
	for _, v := range values {
		if v < m {
			m = v
		}
	}
	return m
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)
	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func indexOf(slice []string, item string) int {
	for i, s := range slice {
		if s == item {
			return i
		}
	}
	return -1
}

// Rollup 时间聚合
func (w *Warehouse) Rollup(req RollupRequest) (*QueryResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	rb, ok := w.ringBuffers[req.Source]
	if !ok {
		return &QueryResult{}, nil
	}

	points := rb.Range(req.StartTime, req.EndTime)

	// 应用过滤器
	filtered := make([]DataPoint, 0)
	for _, dp := range points {
		match := true
		for k, v := range req.Filters {
			if dp.Dimensions[k] != v {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, dp)
		}
	}

	// 按时间间隔分组
	groups := make(map[time.Time][]DataPoint)
	for _, dp := range filtered {
		key := dp.Timestamp.Truncate(req.Interval)
		groups[key] = append(groups[key], dp)
	}

	result := &QueryResult{
		Columns: []string{"timestamp"},
		Total:   len(groups),
	}

	for _, m := range req.Measures {
		result.Columns = append(result.Columns, m.Name)
	}

	for ts, group := range groups {
		row := []interface{}{ts}
		for _, m := range req.Measures {
			row = append(row, w.aggregate(group, m))
		}
		result.Rows = append(result.Rows, row)
	}

	sort.Slice(result.Rows, func(i, j int) bool {
		ti := result.Rows[i][0].(time.Time)
		tj := result.Rows[j][0].(time.Time)
		return ti.Before(tj)
	})

	return result, nil
}

// DrillDown 钻取查询
func (w *Warehouse) DrillDown(req DrillDownRequest) (*QueryResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	rb, ok := w.ringBuffers[req.Source]
	if !ok {
		return &QueryResult{}, nil
	}

	points := rb.Range(req.StartTime, req.EndTime)

	// 过滤出指定维度值的数据
	filtered := make([]DataPoint, 0)
	for _, dp := range points {
		if dp.Dimensions[req.Dimension] != req.Value {
			continue
		}
		match := true
		for k, v := range req.Filters {
			if dp.Dimensions[k] != v {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, dp)
		}
	}

	// 按钻取维度分组
	groups := make(map[string][]DataPoint)
	for _, dp := range filtered {
		groupKey := ""
		for _, dim := range req.DrillDims {
			groupKey += dp.Dimensions[dim] + "|"
		}
		groups[groupKey] = append(groups[groupKey], dp)
	}

	result := &QueryResult{
		Columns: req.DrillDims,
		Total:   len(groups),
	}

	for _, m := range req.Measures {
		result.Columns = append(result.Columns, m.Name)
	}

	for _, group := range groups {
		row := make([]interface{}, 0)
		for _, dim := range req.DrillDims {
			row = append(row, group[0].Dimensions[dim])
		}
		for _, m := range req.Measures {
			row = append(row, w.aggregate(group, m))
		}
		result.Rows = append(result.Rows, row)
	}

	return result, nil
}

// Pivot 旋转查询
func (w *Warehouse) Pivot(req PivotRequest) (*QueryResult, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	rb, ok := w.ringBuffers[req.Source]
	if !ok {
		return &QueryResult{}, nil
	}

	points := rb.Range(req.StartTime, req.EndTime)

	// 应用过滤器
	filtered := make([]DataPoint, 0)
	for _, dp := range points {
		match := true
		for k, v := range req.Filters {
			if dp.Dimensions[k] != v {
				match = false
				break
			}
		}
		if match {
			filtered = append(filtered, dp)
		}
	}

	// 构建pivot结构
	type pivotKey struct {
		rowKey string
		colKey string
	}
	groups := make(map[pivotKey][]DataPoint)
	rowSet := make(map[string]bool)
	colSet := make(map[string]bool)

	for _, dp := range filtered {
		rowKey := ""
		for _, dim := range req.RowDims {
			rowKey += dp.Dimensions[dim] + "|"
		}
		colKey := ""
		for _, dim := range req.ColDims {
			colKey += dp.Dimensions[dim] + "|"
		}
		key := pivotKey{rowKey, colKey}
		groups[key] = append(groups[key], dp)
		rowSet[rowKey] = true
		colSet[colKey] = true
	}

	// 构建列头
	cols := make([]string, 0)
	for col := range colSet {
		cols = append(cols, col)
	}
	sort.Strings(cols)

	result := &QueryResult{
		Columns: req.RowDims,
		Total:   len(rowSet),
	}
	for _, col := range cols {
		result.Columns = append(result.Columns, col)
	}

	// 填充数据
	for row := range rowSet {
		rowData := []interface{}{row}
		for _, col := range cols {
			key := pivotKey{row, col}
			if group, ok := groups[key]; ok {
				aggValue := w.aggregate(group, req.Measures[0])
				rowData = append(rowData, aggValue)
			} else {
				rowData = append(rowData, 0)
			}
		}
		result.Rows = append(result.Rows, rowData)
	}

	return result, nil
}

// Stats 获取统计信息
func (w *Warehouse) Stats() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stats := map[string]interface{}{
		"sources":    len(w.ringBuffers),
		"metrics":    w.timeSeries.Metrics(),
		"cubes":      len(w.cubes),
		"max_points": w.maxPoints,
	}

	sourceStats := make(map[string]int)
	for source, rb := range w.ringBuffers {
		sourceStats[string(source)] = rb.Size()
	}
	stats["source_sizes"] = sourceStats

	return stats
}

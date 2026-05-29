package datawarehouse

import (
	"sync"
	"time"
)

// AggregationType 定义聚合类型
type AggregationType string

const (
	AggSum   AggregationType = "SUM"
	AggAvg   AggregationType = "AVG"
	AggMax   AggregationType = "MAX"
	AggMin   AggregationType = "MIN"
	AggCount AggregationType = "COUNT"
	AggP95   AggregationType = "P95"
	AggP99   AggregationType = "P99"
)

// DataSource 定义数据来源
type DataSource string

const (
	SourceStorage DataSource = "storage"
	SourceNetwork DataSource = "network"
	SourceUser    DataSource = "user"
	SourceApp     DataSource = "app"
)

// Dimension 维度定义
type Dimension struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Measure 度量定义
type Measure struct {
	Name string          `json:"name"`
	Type AggregationType `json:"type"`
}

// DataPoint 数据点
type DataPoint struct {
	Timestamp  time.Time              `json:"timestamp"`
	Source     DataSource             `json:"source"`
	Dimensions map[string]string      `json:"dimensions"`
	Measures   map[string]float64     `json:"measures"`
}

// RingBuffer 环形缓冲区
type RingBuffer struct {
	mu       sync.RWMutex
	data     []DataPoint
	capacity int
	head     int
	tail     int
	size     int
}

// Cube 数据立方体
type Cube struct {
	mu        sync.RWMutex
	dimensions []Dimension
	measures   []Measure
	cells      map[string]map[string]float64 // key: dimension组合, value: measure值
}

// Query 查询定义
type Query struct {
	Source     DataSource         `json:"source"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	Dimensions []string           `json:"dimensions"`
	Measures   []Measure          `json:"measures"`
	Filters    map[string]string  `json:"filters"`
	GroupBy    []string           `json:"group_by"`
	OrderBy    string             `json:"order_by"`
	Limit      int                `json:"limit"`
}

// TimeSeries 时间序列
type TimeSeries struct {
	mu       sync.RWMutex
	series   map[string]*RingBuffer // key: metric name
	maxAge   time.Duration
}

// Warehouse 数据仓库
type Warehouse struct {
	mu          sync.RWMutex
	ringBuffers map[DataSource]*RingBuffer
	cubes       map[string]*Cube
	timeSeries  *TimeSeries
	maxPoints   int // 环形缓冲区大小，默认86400
}

// QueryResult 查询结果
type QueryResult struct {
	Columns []string               `json:"columns"`
	Rows    [][]interface{}        `json:"rows"`
	Total   int                    `json:"total"`
	Meta    map[string]interface{} `json:"meta"`
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp time.Time              `json:"timestamp"`
	Values    map[string]float64     `json:"values"`
	Tags      map[string]string      `json:"tags"`
}

// CubeCell 数据立方体单元格
type CubeCell struct {
	Dimensions map[string]string  `json:"dimensions"`
	Measures   map[string]float64 `json:"measures"`
}

// RollupRequest 聚合请求
type RollupRequest struct {
	Source    DataSource     `json:"source"`
	StartTime time.Time      `json:"start_time"`
	EndTime   time.Time      `json:"end_time"`
	Interval  time.Duration  `json:"interval"`
	Measures  []Measure      `json:"measures"`
	Filters   map[string]string `json:"filters"`
}

// DrillDownRequest 钻取请求
type DrillDownRequest struct {
	Source     DataSource         `json:"source"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	Dimension  string             `json:"dimension"`
	Value      string             `json:"value"`
	DrillDims  []string           `json:"drill_dimensions"`
	Measures   []Measure          `json:"measures"`
	Filters    map[string]string  `json:"filters"`
}

// PivotRequest 旋转请求
type PivotRequest struct {
	Source     DataSource         `json:"source"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    time.Time          `json:"end_time"`
	RowDims    []string           `json:"row_dimensions"`
	ColDims    []string           `json:"col_dimensions"`
	Measures   []Measure          `json:"measures"`
	Filters    map[string]string  `json:"filters"`
}

package datawarehouse

import (
	"testing"
	"time"
)

func TestRingBuffer(t *testing.T) {
	rb := NewRingBuffer(100)

	// 测试Push和Size
	for i := 0; i < 50; i++ {
		rb.Push(DataPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Source:    SourceStorage,
			Measures:  map[string]float64{"cpu": float64(i)},
		})
	}
	if rb.Size() != 50 {
		t.Errorf("Expected size 50, got %d", rb.Size())
	}

	// 测试环形覆盖
	for i := 0; i < 60; i++ {
		rb.Push(DataPoint{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Source:    SourceStorage,
			Measures:  map[string]float64{"cpu": float64(i)},
		})
	}
	if rb.Size() != 100 {
		t.Errorf("Expected size 100, got %d", rb.Size())
	}

	// 测试时间范围查询
	now := time.Now()
	rb2 := NewRingBuffer(1000)
	for i := 0; i < 100; i++ {
		rb2.Push(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Source:    SourceStorage,
			Measures:  map[string]float64{"value": float64(i)},
		})
	}

	result := rb2.Range(now.Add(10*time.Minute), now.Add(19*time.Minute))
	if len(result) != 10 {
		t.Errorf("Expected 10 points in range, got %d", len(result))
	}
}

func TestTimeSeries(t *testing.T) {
	ts := NewTimeSeries(24 * time.Hour)
	now := time.Now()

	// 添加数据
	for i := 0; i < 100; i++ {
		ts.Add("cpu_usage", DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Source:    SourceStorage,
			Measures:  map[string]float64{"cpu_usage": float64(i)},
		})
	}

	// 查询
	points := ts.Query("cpu_usage", now, now.Add(100*time.Second))
	if len(points) != 100 {
		t.Errorf("Expected 100 points, got %d", len(points))
	}

	// 获取指标列表
	metrics := ts.Metrics()
	if len(metrics) != 1 || metrics[0] != "cpu_usage" {
		t.Errorf("Expected [cpu_usage], got %v", metrics)
	}
}

func TestCube(t *testing.T) {
	dims := []Dimension{
		{Name: "user"},
		{Name: "action"},
	}
	measures := []Measure{
		{Name: "count", Type: AggCount},
		{Name: "bytes", Type: AggSum},
	}

	cube := NewCube(dims, measures)

	// 添加数据
	for i := 0; i < 100; i++ {
		user := "user1"
		if i%2 == 0 {
			user = "user2"
		}
		action := "read"
		if i%3 == 0 {
			action = "write"
		}
		cube.Add(DataPoint{
			Timestamp:  time.Now(),
			Dimensions: map[string]string{"user": user, "action": action},
			Measures:   map[string]float64{"count": 1, "bytes": float64(i * 100)},
		})
	}

	// 切片查询
	sliceResult := cube.Slice("user", "user1")
	if len(sliceResult) == 0 {
		t.Error("Expected non-empty slice result")
	}

	// 切块查询
	diceResult := cube.Dice(map[string]string{"user": "user2", "action": "read"})
	if len(diceResult) == 0 {
		t.Error("Expected non-empty dice result")
	}
}

func TestWarehouseQuery(t *testing.T) {
	wh := NewWarehouse(86400)
	now := time.Now()

	// 采集数据
	for i := 0; i < 100; i++ {
		wh.Ingest(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Source:    SourceStorage,
			Dimensions: map[string]string{
				"user":   "user1",
				"action": "read",
			},
			Measures: map[string]float64{
				"bytes": float64(i * 1024),
				"count": 1,
			},
		})
	}

	// 查询
	result, err := wh.Query(Query{
		Source:    SourceStorage,
		StartTime: now,
		EndTime:   now.Add(100 * time.Minute),
		Measures: []Measure{
			{Name: "bytes", Type: AggSum},
			{Name: "count", Type: AggCount},
		},
		GroupBy: []string{"user"},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.Total == 0 {
		t.Error("Expected non-empty result")
	}
}

func TestWarehouseAggregations(t *testing.T) {
	wh := NewWarehouse(86400)
	now := time.Now()

	// 采集数据
	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for i, v := range values {
		wh.Ingest(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Source:    SourceNetwork,
			Dimensions: map[string]string{
				"interface": "eth0",
			},
			Measures: map[string]float64{
				"bandwidth": v,
			},
		})
	}

	// 测试SUM
	result, _ := wh.Query(Query{
		Source:    SourceNetwork,
		StartTime: now,
		EndTime:   now.Add(10 * time.Second),
		Measures:  []Measure{{Name: "bandwidth", Type: AggSum}},
		GroupBy:   []string{"interface"},
	})
	if len(result.Rows) > 0 {
		sum := result.Rows[0][1].(float64)
		if sum != 550 {
			t.Errorf("Expected SUM 550, got %f", sum)
		}
	}

	// 测试AVG
	result, _ = wh.Query(Query{
		Source:    SourceNetwork,
		StartTime: now,
		EndTime:   now.Add(10 * time.Second),
		Measures:  []Measure{{Name: "bandwidth", Type: AggAvg}},
		GroupBy:   []string{"interface"},
	})
	if len(result.Rows) > 0 {
		avg := result.Rows[0][1].(float64)
		if avg != 55 {
			t.Errorf("Expected AVG 55, got %f", avg)
		}
	}

	// 测试MAX
	result, _ = wh.Query(Query{
		Source:    SourceNetwork,
		StartTime: now,
		EndTime:   now.Add(10 * time.Second),
		Measures:  []Measure{{Name: "bandwidth", Type: AggMax}},
		GroupBy:   []string{"interface"},
	})
	if len(result.Rows) > 0 {
		maxVal := result.Rows[0][1].(float64)
		if maxVal != 100 {
			t.Errorf("Expected MAX 100, got %f", maxVal)
		}
	}

	// 测试MIN
	result, _ = wh.Query(Query{
		Source:    SourceNetwork,
		StartTime: now,
		EndTime:   now.Add(10 * time.Second),
		Measures:  []Measure{{Name: "bandwidth", Type: AggMin}},
		GroupBy:   []string{"interface"},
	})
	if len(result.Rows) > 0 {
		minVal := result.Rows[0][1].(float64)
		if minVal != 10 {
			t.Errorf("Expected MIN 10, got %f", minVal)
		}
	}
}

func TestRollup(t *testing.T) {
	wh := NewWarehouse(86400)
	now := time.Now()

	// 采集数据
	for i := 0; i < 100; i++ {
		wh.Ingest(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Source:    SourceStorage,
			Dimensions: map[string]string{
				"user": "user1",
			},
			Measures: map[string]float64{
				"io": float64(i),
			},
		})
	}

	// 时间聚合
	result, err := wh.Rollup(RollupRequest{
		Source:    SourceStorage,
		StartTime: now,
		EndTime:   now.Add(100 * time.Minute),
		Interval:  10 * time.Minute,
		Measures:  []Measure{{Name: "io", Type: AggSum}},
	})
	if err != nil {
		t.Fatalf("Rollup failed: %v", err)
	}

	if len(result.Rows) < 10 || len(result.Rows) > 11 {
		t.Errorf("Expected 10-11 time buckets, got %d", result.Total)
	}
}

func TestDrillDown(t *testing.T) {
	wh := NewWarehouse(86400)
	now := time.Now()

	// 采集数据
	for i := 0; i < 50; i++ {
		action := "read"
		if i%2 == 0 {
			action = "write"
		}
		wh.Ingest(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Source:    SourceUser,
			Dimensions: map[string]string{
				"user":   "user1",
				"action": action,
			},
			Measures: map[string]float64{
				"ops": 1,
			},
		})
	}

	// 钻取查询
	result, err := wh.DrillDown(DrillDownRequest{
		Source:    SourceUser,
		StartTime: now,
		EndTime:   now.Add(50 * time.Second),
		Dimension: "user",
		Value:     "user1",
		DrillDims: []string{"action"},
		Measures:  []Measure{{Name: "ops", Type: AggCount}},
	})
	if err != nil {
		t.Fatalf("DrillDown failed: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 action groups, got %d", result.Total)
	}
}

func TestPivot(t *testing.T) {
	wh := NewWarehouse(86400)
	now := time.Now()

	// 采集数据
	users := []string{"user1", "user2"}
	actions := []string{"read", "write", "delete"}
	for i := 0; i < 30; i++ {
		wh.Ingest(DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Source:    SourceApp,
			Dimensions: map[string]string{
				"user":   users[i%2],
				"action": actions[i%3],
			},
			Measures: map[string]float64{
				"count": 1,
			},
		})
	}

	// 旋转查询
	result, err := wh.Pivot(PivotRequest{
		Source:    SourceApp,
		StartTime: now,
		EndTime:   now.Add(30 * time.Second),
		RowDims:   []string{"user"},
		ColDims:   []string{"action"},
		Measures:  []Measure{{Name: "count", Type: AggSum}},
	})
	if err != nil {
		t.Fatalf("Pivot failed: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 rows, got %d", result.Total)
	}
}

func TestStats(t *testing.T) {
	wh := NewWarehouse(86400)

	// 采集一些数据
	now := time.Now()
	wh.Ingest(DataPoint{
		Timestamp: now,
		Source:    SourceStorage,
		Measures:  map[string]float64{"cpu": 50},
	})
	wh.Ingest(DataPoint{
		Timestamp: now,
		Source:    SourceNetwork,
		Measures:  map[string]float64{"bandwidth": 100},
	})

	stats := wh.Stats()
	if stats["sources"] != 2 {
		t.Errorf("Expected 2 sources, got %v", stats["sources"])
	}
	if stats["max_points"] != 86400 {
		t.Errorf("Expected max_points 86400, got %v", stats["max_points"])
	}
}

func TestPercentile(t *testing.T) {
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	p95 := percentile(values, 0.95)
	if p95 != 10 {
		t.Errorf("Expected P95=10, got %f", p95)
	}

	p99 := percentile(values, 0.99)
	if p99 != 10 {
		t.Errorf("Expected P99=10, got %f", p99)
	}

	// 空值
	empty := percentile([]float64{}, 0.95)
	if empty != 0 {
		t.Errorf("Expected 0 for empty, got %f", empty)
	}
}

func TestRaceCondition(t *testing.T) {
	wh := NewWarehouse(1000)
	now := time.Now()
	done := make(chan bool, 10)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				wh.Ingest(DataPoint{
					Timestamp: now.Add(time.Duration(j) * time.Second),
					Source:    SourceStorage,
					Dimensions: map[string]string{
						"writer": string(rune('A' + id)),
					},
					Measures: map[string]float64{
						"value": float64(j),
					},
				})
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				wh.Query(Query{
					Source:    SourceStorage,
					StartTime: now,
					EndTime:   now.Add(100 * time.Second),
					Measures:  []Measure{{Name: "value", Type: AggSum}},
					GroupBy:   []string{"writer"},
				})
			}
			done <- true
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < 15; i++ {
		<-done
	}
}

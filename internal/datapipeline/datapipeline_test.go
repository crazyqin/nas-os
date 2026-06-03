package datapipeline

import (
	"context"
	"log"
	"os"
	"testing"
	"time"
)

// TestPipelineValidation 测试流水线验证
func TestPipelineValidation(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
		pipeline *Pipeline
	}{
		{
			name:    "valid pipeline",
			wantErr: false,
			pipeline: &Pipeline{
				ID:   "test-1",
				Name: "Test Pipeline",
				DataSources: []DataSource{
					{
						ID:   "ds-1",
						Name: "Local Files",
						Type: DataSourceFileSystem,
						Connection: map[string]interface{}{
							"path": "/tmp",
						},
						Enabled: true,
					},
				},
				Processors: []ProcessorNode{
					{
						ID:      "proc-1",
						Name:    "Filter",
						Type:    ProcessorTypeFilter,
						Enabled: true,
						Config: map[string]interface{}{
							"name": "size_filter",
							"condition": map[string]interface{}{
								"field":    "size",
								"operator": "greater_than",
								"value":    100,
							},
						},
					},
				},
				Outputs: []OutputNode{
					{
						ID:      "out-1",
						Name:    "File Output",
						Type:    OutputTypeFile,
						Enabled: true,
						Config: map[string]interface{}{
							"path":   "/tmp/output.json",
							"format": "json",
						},
					},
				},
			},
		},
		{
			name:    "missing ID",
			wantErr: true,
			pipeline: &Pipeline{
				Name: "Test Pipeline",
				DataSources: []DataSource{
					{
						ID:   "ds-1",
						Name: "Local Files",
						Type: DataSourceFileSystem,
						Connection: map[string]interface{}{
							"path": "/tmp",
						},
					},
				},
				Processors: []ProcessorNode{
					{
						ID:   "proc-1",
						Name: "Filter",
						Type: ProcessorTypeFilter,
						Config: map[string]interface{}{
							"name": "size_filter",
							"condition": map[string]interface{}{
								"field":    "size",
								"operator": "greater_than",
								"value":    100,
							},
						},
					},
				},
			},
		},
		{
			name:    "missing data source",
			wantErr: true,
			pipeline: &Pipeline{
				ID:   "test-2",
				Name: "Test Pipeline",
				Processors: []ProcessorNode{
					{
						ID:   "proc-1",
						Name: "Filter",
						Type: ProcessorTypeFilter,
						Config: map[string]interface{}{
							"name": "size_filter",
							"condition": map[string]interface{}{
								"field":    "size",
								"operator": "greater_than",
								"value":    100,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pipeline.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Pipeline.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestDAGValidation 测试 DAG 验证
func TestDAGValidation(t *testing.T) {
	t.Run("valid DAG", func(t *testing.T) {
		pipeline := &Pipeline{
			ID:   "dag-test",
			Name: "DAG Test",
			DataSources: []DataSource{
				{
					ID:   "ds-1",
					Name: "Source",
					Type: DataSourceFileSystem,
					Connection: map[string]interface{}{
						"path": "/tmp",
					},
				},
			},
			Processors: []ProcessorNode{
				{ID: "proc-1", Name: "Filter", Type: ProcessorTypeFilter, Config: map[string]interface{}{
					"name": "filter1",
					"condition": map[string]interface{}{
						"field":    "size",
						"operator": "greater_than",
						"value":    100,
					},
				}},
				{ID: "proc-2", Name: "Transform", Type: ProcessorTypeTransform, Config: map[string]interface{}{
					"name":       "transform1",
					"transforms": []interface{}{},
				}},
			},
			Outputs: []OutputNode{
				{ID: "out-1", Name: "Output", Type: OutputTypeFile, Config: map[string]interface{}{
					"path": "/tmp/out.json",
				}},
			},
			DAG: []DAGEdge{
				{From: "proc-1", To: "proc-2"},
				{From: "proc-2", To: "out-1"},
			},
		}

		if err := pipeline.validateDAG(); err != nil {
			t.Errorf("Expected valid DAG, got error: %v", err)
		}
	})

	t.Run("DAG with cycle", func(t *testing.T) {
		pipeline := &Pipeline{
			ID:   "dag-cycle-test",
			Name: "DAG Cycle Test",
			DataSources: []DataSource{
				{
					ID:   "ds-1",
					Name: "Source",
					Type: DataSourceFileSystem,
					Connection: map[string]interface{}{
						"path": "/tmp",
					},
				},
			},
			Processors: []ProcessorNode{
				{ID: "proc-1", Name: "Filter", Type: ProcessorTypeFilter, Config: map[string]interface{}{
					"name": "filter1",
					"condition": map[string]interface{}{
						"field":    "size",
						"operator": "greater_than",
						"value":    100,
					},
				}},
				{ID: "proc-2", Name: "Transform", Type: ProcessorTypeTransform, Config: map[string]interface{}{
					"name":       "transform1",
					"transforms": []interface{}{},
				}},
			},
			DAG: []DAGEdge{
				{From: "proc-1", To: "proc-2"},
				{From: "proc-2", To: "proc-1"}, // 创建环
			},
		}

		if err := pipeline.validateDAG(); err == nil {
			t.Error("Expected error for DAG with cycle, got nil")
		}
	})
}

// TestTopologicalOrder 测试拓扑排序
func TestTopologicalOrder(t *testing.T) {
	pipeline := &Pipeline{
		ID:   "topo-test",
		Name: "Topological Test",
		DataSources: []DataSource{
			{
				ID:   "ds-1",
				Name: "Source",
				Type: DataSourceFileSystem,
				Connection: map[string]interface{}{
					"path": "/tmp",
				},
			},
		},
		Processors: []ProcessorNode{
			{ID: "proc-1", Name: "P1", Type: ProcessorTypeFilter, Config: map[string]interface{}{
				"name": "filter1",
				"condition": map[string]interface{}{
					"field":    "size",
					"operator": "greater_than",
					"value":    100,
				},
			}},
			{ID: "proc-2", Name: "P2", Type: ProcessorTypeTransform, Config: map[string]interface{}{
				"name":       "transform1",
				"transforms": []interface{}{},
			}},
			{ID: "proc-3", Name: "P3", Type: ProcessorTypeEnrichment, Config: map[string]interface{}{
				"name":   "enrichment1",
				"fields": []interface{}{},
			}},
		},
		DAG: []DAGEdge{
			{From: "proc-1", To: "proc-2"},
			{From: "proc-1", To: "proc-3"},
		},
	}

	order, err := pipeline.GetTopologicalOrder()
	if err != nil {
		t.Fatalf("Failed to get topological order: %v", err)
	}

	// proc-1 应该在 proc-2 和 proc-3 之前
	proc1Idx := -1
	proc2Idx := -1
	proc3Idx := -1
	for i, id := range order {
		switch id {
		case "proc-1":
			proc1Idx = i
		case "proc-2":
			proc2Idx = i
		case "proc-3":
			proc3Idx = i
		}
	}

	if proc1Idx == -1 || proc2Idx == -1 || proc3Idx == -1 {
		t.Fatalf("Not all nodes found in order: %v", order)
	}

	if proc1Idx >= proc2Idx || proc1Idx >= proc3Idx {
		t.Errorf("proc-1 should be before proc-2 and proc-3, order: %v", order)
	}
}

// TestFilterProcessor 测试过滤器处理器
func TestFilterProcessor(t *testing.T) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name": "size_filter",
		"condition": map[string]interface{}{
			"field":    "size",
			"operator": "greater_than",
			"value":    100,
		},
	}

	processor, err := factory.Create(ProcessorTypeFilter, config)
	if err != nil {
		t.Fatalf("Failed to create filter processor: %v", err)
	}

	input := []map[string]interface{}{
		{"name": "file1.txt", "size": 200},
		{"name": "file2.txt", "size": 50},
		{"name": "file3.txt", "size": 300},
		{"name": "file4.txt", "size": 80},
	}

	ctx := context.Background()
	output, err := processor.Process(ctx, input)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	if len(output) != 2 {
		t.Errorf("Expected 2 items, got %d", len(output))
	}
}

// TestTransformProcessor 测试转换器处理器
func TestTransformProcessor(t *testing.T) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name": "name_transform",
		"transforms": []interface{}{
			map[string]interface{}{
				"field":  "name",
				"action": "uppercase",
			},
			map[string]interface{}{
				"field":  "type",
				"action": "set",
				"value":  "document",
			},
		},
	}

	processor, err := factory.Create(ProcessorTypeTransform, config)
	if err != nil {
		t.Fatalf("Failed to create transform processor: %v", err)
	}

	input := []map[string]interface{}{
		{"name": "hello", "size": 100},
		{"name": "world", "size": 200},
	}

	ctx := context.Background()
	output, err := processor.Process(ctx, input)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	if len(output) != 2 {
		t.Errorf("Expected 2 items, got %d", len(output))
	}

	if output[0]["name"] != "HELLO" {
		t.Errorf("Expected 'HELLO', got '%v'", output[0]["name"])
	}

	if output[0]["type"] != "document" {
		t.Errorf("Expected 'document', got '%v'", output[0]["type"])
	}
}

// TestAggregateProcessor 测试聚合器处理器
func TestAggregateProcessor(t *testing.T) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name":    "size_aggregate",
		"groupBy": []interface{}{"category"},
		"aggregates": []interface{}{
			map[string]interface{}{
				"field":    "size",
				"function": "sum",
				"output":   "total_size",
			},
			map[string]interface{}{
				"field":    "size",
				"function": "count",
				"output":   "file_count",
			},
		},
	}

	processor, err := factory.Create(ProcessorTypeAggregate, config)
	if err != nil {
		t.Fatalf("Failed to create aggregate processor: %v", err)
	}

	input := []map[string]interface{}{
		{"category": "docs", "size": 100},
		{"category": "docs", "size": 200},
		{"category": "images", "size": 300},
		{"category": "images", "size": 400},
	}

	ctx := context.Background()
	output, err := processor.Process(ctx, input)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	if len(output) != 2 {
		t.Errorf("Expected 2 groups, got %d", len(output))
	}
}

// TestDeduplicatorProcessor 测试去重器处理器
func TestDeduplicatorProcessor(t *testing.T) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name":   "dedup",
		"fields": []interface{}{"id"},
	}

	processor, err := factory.Create(ProcessorTypeDeduplicator, config)
	if err != nil {
		t.Fatalf("Failed to create deduplicator processor: %v", err)
	}

	input := []map[string]interface{}{
		{"id": 1, "name": "item1"},
		{"id": 2, "name": "item2"},
		{"id": 1, "name": "item1_dup"},
		{"id": 3, "name": "item3"},
	}

	ctx := context.Background()
	output, err := processor.Process(ctx, input)
	if err != nil {
		t.Fatalf("Failed to process: %v", err)
	}

	if len(output) != 3 {
		t.Errorf("Expected 3 unique items, got %d", len(output))
	}
}

// TestPipelineEngine 测试流水线引擎
func TestPipelineEngine(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	engine := NewPipelineEngine(logger)

	pipeline := &Pipeline{
		ID:          "test-pipeline",
		Name:        "Test Pipeline",
		Status:      PipelineStatusActive,
		Concurrency: 2,
		DataSources: []DataSource{
			{
				ID:   "ds-1",
				Name: "Test Source",
				Type: DataSourceFileSystem,
				Connection: map[string]interface{}{
					"path":      "/tmp",
					"recursive": false,
				},
				Enabled: true,
			},
		},
		Processors: []ProcessorNode{
			{
				ID:      "proc-1",
				Name:    "Filter",
				Type:    ProcessorTypeFilter,
				Enabled: true,
				Config: map[string]interface{}{
					"name": "size_filter",
					"condition": map[string]interface{}{
						"field":    "size",
						"operator": "greater_than",
						"value":    0,
					},
				},
			},
		},
		Outputs: []OutputNode{
			{
				ID:      "out-1",
				Name:    "File Output",
				Type:    OutputTypeFile,
				Enabled: true,
				Config: map[string]interface{}{
					"path":   "/tmp/test_output.json",
					"format": "json",
				},
			},
		},
		DAG: []DAGEdge{
			{From: "proc-1", To: "out-1"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	inputData := map[string]interface{}{
		"source": "test",
	}

	execution, err := engine.Execute(ctx, pipeline, inputData)
	if err != nil {
		t.Fatalf("Failed to execute pipeline: %v", err)
	}

	// 等待执行完成
	time.Sleep(2 * time.Second)

	if execution.Status != ExecutionStatusSuccess {
		t.Errorf("Expected success status, got %s", execution.Status)
	}
}

// TestPipelineManager 测试流水线管理器
func TestPipelineManager(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	manager := NewPipelineManager(logger)

	// 创建流水线
	pipeline := &Pipeline{
		ID:          "manager-test",
		Name:        "Manager Test",
		Status:      PipelineStatusDraft,
		Concurrency: 1,
		DataSources: []DataSource{
			{
				ID:   "ds-1",
				Name: "Test Source",
				Type: DataSourceFileSystem,
				Connection: map[string]interface{}{
					"path": "/tmp",
				},
				Enabled: true,
			},
		},
		Processors: []ProcessorNode{
			{
				ID:      "proc-1",
				Name:    "Filter",
				Type:    ProcessorTypeFilter,
				Enabled: true,
				Config: map[string]interface{}{
					"name": "size_filter",
					"condition": map[string]interface{}{
						"field":    "size",
						"operator": "greater_than",
						"value":    0,
					},
				},
			},
		},
		Outputs: []OutputNode{
			{
				ID:      "out-1",
				Name:    "File Output",
				Type:    OutputTypeFile,
				Enabled: true,
				Config: map[string]interface{}{
					"path": "/tmp/manager_output.json",
				},
			},
		},
	}

	err := manager.CreatePipeline(pipeline)
	if err != nil {
		t.Fatalf("Failed to create pipeline: %v", err)
	}

	// 获取流水线
	retrieved, err := manager.GetPipeline("manager-test")
	if err != nil {
		t.Fatalf("Failed to get pipeline: %v", err)
	}

	if retrieved.Name != "Manager Test" {
		t.Errorf("Expected 'Manager Test', got '%s'", retrieved.Name)
	}

	// 启用流水线
	err = manager.EnablePipeline("manager-test")
	if err != nil {
		t.Fatalf("Failed to enable pipeline: %v", err)
	}

	// 执行流水线
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	execution, err := manager.ExecutePipeline(ctx, "manager-test", nil)
	if err != nil {
		t.Fatalf("Failed to execute pipeline: %v", err)
	}

	// 等待执行完成
	time.Sleep(2 * time.Second)

	if execution.Status != ExecutionStatusSuccess {
		t.Errorf("Expected success, got %s", execution.Status)
	}

	// 获取统计
	stats, err := manager.GetPipelineStats("manager-test")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.TotalExecutions < 1 {
		t.Errorf("Expected at least 1 execution, got %d", stats.TotalExecutions)
	}
}

// TestConnectorFactory 测试连接器工厂
func TestConnectorFactory(t *testing.T) {
	tests := []struct {
		name    string
		dsType  DataSourceType
		wantErr bool
	}{
		{
			name:    "filesystem",
			dsType:  DataSourceFileSystem,
			wantErr: false,
		},
		{
			name:    "smb",
			dsType:  DataSourceSMB,
			wantErr: false,
		},
		{
			name:    "s3",
			dsType:  DataSourceS3,
			wantErr: false,
		},
		{
			name:    "database",
			dsType:  DataSourceDatabase,
			wantErr: false,
		},
		{
			name:    "http",
			dsType:  DataSourceHTTP,
			wantErr: false,
		},
		{
			name:    "unsupported",
			dsType:  "unsupported",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := DataSource{
				ID:   "test-ds",
				Name: "Test DS",
				Type: tt.dsType,
				Connection: map[string]interface{}{
					"path":     "/tmp",
					"host":     "localhost",
					"share":    "test",
					"bucket":   "test-bucket",
					"driver":   "sqlite",
					"dsn":      ":memory:",
					"url":      "http://localhost",
					"endpoint": "http://localhost:9000",
				},
			}

			connector, err := NewConnector(ds)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnector() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && connector == nil {
				t.Error("Expected connector, got nil")
			}
		})
	}
}

// TestProcessorFactory 测试处理器工厂
func TestProcessorFactory(t *testing.T) {
	factory := NewProcessorFactory()

	tests := []struct {
		name    string
		pType   ProcessorType
		wantErr bool
	}{
		{
			name:    "filter",
			pType:   ProcessorTypeFilter,
			wantErr: false,
		},
		{
			name:    "transform",
			pType:   ProcessorTypeTransform,
			wantErr: false,
		},
		{
			name:    "aggregate",
			pType:   ProcessorTypeAggregate,
			wantErr: false,
		},
		{
			name:    "deduplicator",
			pType:   ProcessorTypeDeduplicator,
			wantErr: false,
		},
		{
			name:    "unsupported",
			pType:   "unsupported",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := map[string]interface{}{
				"name": "test",
				"condition": map[string]interface{}{
					"field":    "size",
					"operator": "greater_than",
					"value":    100,
				},
				"transforms": []interface{}{},
				"aggregates": []interface{}{},
				"groupBy":    []interface{}{"key"},
				"fields":     []interface{}{"id"},
				"rules":      []interface{}{},
			}

			processor, err := factory.Create(tt.pType, config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Factory.Create() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && processor == nil {
				t.Error("Expected processor, got nil")
			}
		})
	}
}

// TestRetryPolicy 测试重试策略
func TestRetryPolicy(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	engine := NewPipelineEngine(logger)

	// 创建一个会失败的处理器
	pipeline := &Pipeline{
		ID:          "retry-test",
		Name:        "Retry Test",
		Status:      PipelineStatusActive,
		Concurrency: 1,
		DataSources: []DataSource{
			{
				ID:   "ds-1",
				Name: "Test Source",
				Type: DataSourceFileSystem,
				Connection: map[string]interface{}{
					"path": "/tmp",
				},
				Enabled: true,
			},
		},
		Processors: []ProcessorNode{
			{
				ID:      "proc-1",
				Name:    "Filter",
				Type:    ProcessorTypeFilter,
				Enabled: true,
				Config: map[string]interface{}{
					"name": "size_filter",
					"condition": map[string]interface{}{
						"field":    "size",
						"operator": "greater_than",
						"value":    0,
					},
				},
			},
		},
		Outputs: []OutputNode{
			{
				ID:      "out-1",
				Name:    "Invalid Output",
				Type:    OutputTypeFile,
				Enabled: true,
				Config: map[string]interface{}{
					"path": "/nonexistent/path/output.json",
				},
			},
		},
		DAG: []DAGEdge{
			{From: "proc-1", To: "out-1"},
		},
		RetryPolicy: &RetryPolicy{
			MaxRetries:      2,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     500 * time.Millisecond,
			Multiplier:      2.0,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	execution, err := engine.Execute(ctx, pipeline, nil)
	if err != nil {
		t.Fatalf("Failed to start execution: %v", err)
	}

	// 等待执行完成
	for i := 0; i < 50; i++ {
		time.Sleep(200 * time.Millisecond)
		if execution.Status == ExecutionStatusFailed {
			break
		}
	}

	if execution.Status != ExecutionStatusFailed {
		t.Errorf("Expected failed status, got %s", execution.Status)
	}
}

// BenchmarkFilterProcessor 基准测试过滤器
func BenchmarkFilterProcessor(b *testing.B) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name": "size_filter",
		"condition": map[string]interface{}{
			"field":    "size",
			"operator": "greater_than",
			"value":    100,
		},
	}

	processor, _ := factory.Create(ProcessorTypeFilter, config)

	input := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = map[string]interface{}{
			"name": "file",
			"size": i * 100,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.Process(ctx, input)
	}
}

// BenchmarkTransformProcessor 基准测试转换器
func BenchmarkTransformProcessor(b *testing.B) {
	factory := NewProcessorFactory()

	config := map[string]interface{}{
		"name": "transform",
		"transforms": []interface{}{
			map[string]interface{}{
				"field":  "name",
				"action": "uppercase",
			},
		},
	}

	processor, _ := factory.Create(ProcessorTypeTransform, config)

	input := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = map[string]interface{}{
			"name": "test",
			"size": i,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.Process(ctx, input)
	}
}

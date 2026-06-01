package unifieddatalake

import (
	"fmt"
	"testing"
	"time"
)

func TestDataLakeSources(t *testing.T) {
	dl := NewDataLake()

	// 添加S3数据源
	s3 := &DataSource{
		ID:       "s3-1",
		Name:     "my-s3-bucket",
		Type:     StorageS3,
		Endpoint: "https://s3.amazonaws.com",
		Bucket:   "my-bucket",
		Capacity: 1024 * 1024 * 1024 * 100, // 100GB
	}
	if err := dl.AddSource(s3); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// 添加NFS数据源
	nfs := &DataSource{
		ID:        "nfs-1",
		Name:      "nas-share",
		Type:      StorageNFS,
		Endpoint:  "192.168.1.100",
		MountPath: "/mnt/nas",
		Capacity:  1024 * 1024 * 1024 * 500, // 500GB
	}
	if err := dl.AddSource(nfs); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// 添加本地数据源
	local := &DataSource{
		ID:       "local-1",
		Name:     "local-ssd",
		Type:     StorageLocal,
		Endpoint: "/data",
		Capacity: 1024 * 1024 * 1024 * 50, // 50GB
	}
	if err := dl.AddSource(local); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}

	// 测试列出
	sources := dl.ListSources()
	if len(sources) != 3 {
		t.Errorf("Expected 3 sources, got %d", len(sources))
	}

	// 测试获取
	got, ok := dl.GetSource("s3-1")
	if !ok || got.Name != "my-s3-bucket" {
		t.Error("GetSource failed")
	}

	// 测试不存在
	_, ok = dl.GetSource("nonexistent")
	if ok {
		t.Error("Expected false for nonexistent source")
	}

	// 测试更新
	s3.Used = 1024 * 1024 * 50 // 50MB
	if err := dl.UpdateSource(s3); err != nil {
		t.Fatalf("UpdateSource failed: %v", err)
	}
	got, _ = dl.GetSource("s3-1")
	if got.Used != 1024*1024*50 {
		t.Error("UpdateSource didn't update")
	}

	// 测试移除
	if err := dl.RemoveSource("local-1"); err != nil {
		t.Fatalf("RemoveSource failed: %v", err)
	}
	sources = dl.ListSources()
	if len(sources) != 2 {
		t.Errorf("Expected 2 sources after delete, got %d", len(sources))
	}

	// 测试移除不存在的
	if err := dl.RemoveSource("nonexistent"); err == nil {
		t.Error("Expected error for removing nonexistent source")
	}
}

func TestDataLakeObjects(t *testing.T) {
	dl := NewDataLake()
	dl.AddSource(&DataSource{
		ID:   "src1",
		Name: "test-source",
		Type: StorageLocal,
	})

	// 注册对象
	obj := &DataObject{
		SourceID:    "src1",
		Path:        "/data/file.csv",
		Name:        "file.csv",
		Size:        1024,
		ContentType: "text/csv",
	}
	if err := dl.RegisterObject(obj); err != nil {
		t.Fatalf("RegisterObject failed: %v", err)
	}
	if obj.ID == "" {
		t.Error("Expected auto-generated ID")
	}

	// 注册到不存在的source
	badObj := &DataObject{
		SourceID: "nonexistent",
		Path:     "/data/bad.csv",
		Name:     "bad.csv",
	}
	if err := dl.RegisterObject(badObj); err == nil {
		t.Error("Expected error for nonexistent source")
	}

	// 获取
	got, ok := dl.GetObject(obj.ID)
	if !ok || got.Name != "file.csv" {
		t.Error("GetObject failed")
	}

	// 列出
	objects := dl.ListObjects("")
	if len(objects) != 1 {
		t.Errorf("Expected 1 object, got %d", len(objects))
	}

	// 按source过滤
	objects = dl.ListObjects("src1")
	if len(objects) != 1 {
		t.Errorf("Expected 1 object for src1, got %d", len(objects))
	}
	objects = dl.ListObjects("src2")
	if len(objects) != 0 {
		t.Errorf("Expected 0 objects for src2, got %d", len(objects))
	}

	// 移除
	if err := dl.RemoveObject(obj.ID); err != nil {
		t.Fatalf("RemoveObject failed: %v", err)
	}
	_, ok = dl.GetObject(obj.ID)
	if ok {
		t.Error("Expected object to be removed")
	}

	// 移除不存在的
	if err := dl.RemoveObject("nonexistent"); err == nil {
		t.Error("Expected error for removing nonexistent object")
	}
}

func TestDataLakeCatalog(t *testing.T) {
	dl := NewDataLake()
	dl.AddSource(&DataSource{ID: "src1", Name: "test", Type: StorageLocal})

	obj := &DataObject{SourceID: "src1", Path: "/data/test.csv", Name: "test.csv", Size: 2048}
	dl.RegisterObject(obj)

	// 添加目录条目
	entry := &CatalogEntry{
		ObjectID:    obj.ID,
		Name:        "测试数据集",
		Description: "这是一个测试数据集",
		Category:    "analytics",
		Owner:       "user1",
		Tags:        map[string]string{"env": "dev", "team": "data"},
		Schema: &DataSchema{
			Fields: []SchemaField{
				{Name: "id", Type: "int", Nullable: false},
				{Name: "name", Type: "string", Nullable: true},
				{Name: "value", Type: "float", Nullable: false},
			},
		},
	}
	if err := dl.AddCatalogEntry(entry); err != nil {
		t.Fatalf("AddCatalogEntry failed: %v", err)
	}
	if entry.Version != 1 {
		t.Errorf("Expected version 1, got %d", entry.Version)
	}

	// 添加到不存在的对象
	badEntry := &CatalogEntry{ObjectID: "nonexistent", Name: "bad"}
	if err := dl.AddCatalogEntry(badEntry); err == nil {
		t.Error("Expected error for nonexistent object")
	}

	// 获取
	got, ok := dl.GetCatalogEntry(entry.ID)
	if !ok || got.Name != "测试数据集" {
		t.Error("GetCatalogEntry failed")
	}

	// 列出
	entries := dl.ListCatalogEntries("")
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(entries))
	}
	entries = dl.ListCatalogEntries("analytics")
	if len(entries) != 1 {
		t.Errorf("Expected 1 entry for analytics, got %d", len(entries))
	}
	entries = dl.ListCatalogEntries("other")
	if len(entries) != 0 {
		t.Errorf("Expected 0 entries for other, got %d", len(entries))
	}

	// 更新
	entry.Description = "更新后的描述"
	if err := dl.UpdateCatalogEntry(entry); err != nil {
		t.Fatalf("UpdateCatalogEntry failed: %v", err)
	}
	got, _ = dl.GetCatalogEntry(entry.ID)
	if got.Version != 2 {
		t.Errorf("Expected version 2, got %d", got.Version)
	}
	if got.Description != "更新后的描述" {
		t.Error("Update didn't apply")
	}

	// 搜索
	results := dl.SearchCatalog("测试")
	if len(results) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(results))
	}
	results = dl.SearchCatalog("analytics")
	if len(results) != 1 {
		t.Errorf("Expected 1 search result for category, got %d", len(results))
	}
}

func TestDataLakeLineage(t *testing.T) {
	dl := NewDataLake()
	dl.AddSource(&DataSource{ID: "src1", Name: "test", Type: StorageLocal})

	obj1 := &DataObject{SourceID: "src1", Path: "/raw/data.csv", Name: "data.csv"}
	obj2 := &DataObject{SourceID: "src1", Path: "/processed/result.parquet", Name: "result.parquet"}
	dl.RegisterObject(obj1)
	dl.RegisterObject(obj2)

	// 创建血缘图
	graph := &LineageGraph{
		ID: "lineage-1",
		Nodes: []*LineageNode{
			{ID: "n1", ObjectID: obj1.ID, Name: "raw-data", Type: "source"},
			{ID: "n2", ObjectID: obj2.ID, Name: "processed", Type: "destination"},
		},
		Edges: []*LineageEdge{
			{ID: "e1", SourceNodeID: "n1", TargetNodeID: "n2", Transform: "ETL"},
		},
	}
	if err := dl.CreateLineage(graph); err != nil {
		t.Fatalf("CreateLineage failed: %v", err)
	}

	// 获取
	got, ok := dl.GetLineage("lineage-1")
	if !ok || len(got.Nodes) != 2 {
		t.Error("GetLineage failed")
	}

	// 列出
	lineages := dl.ListLineages()
	if len(lineages) != 1 {
		t.Errorf("Expected 1 lineage, got %d", len(lineages))
	}

	// 添加节点
	newNode := &LineageNode{ObjectID: obj1.ID, Name: "intermediate", Type: "transform"}
	if err := dl.AddLineageNode("lineage-1", newNode); err != nil {
		t.Fatalf("AddLineageNode failed: %v", err)
	}
	got, _ = dl.GetLineage("lineage-1")
	if len(got.Nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(got.Nodes))
	}

	// 添加边
	newEdge := &LineageEdge{SourceNodeID: "n1", TargetNodeID: newNode.ID, Transform: "filter"}
	if err := dl.AddLineageEdge("lineage-1", newEdge); err != nil {
		t.Fatalf("AddLineageEdge failed: %v", err)
	}

	// 根据对象获取血缘
	graphs := dl.GetLineageByObject(obj1.ID)
	if len(graphs) != 1 {
		t.Errorf("Expected 1 lineage for object, got %d", len(graphs))
	}

	// 获取上游
	upstream, err := dl.GetUpstream("lineage-1", "n2")
	if err != nil {
		t.Fatalf("GetUpstream failed: %v", err)
	}
	if len(upstream) != 1 || upstream[0].ID != "n1" {
		t.Errorf("Expected 1 upstream node (n1), got %v", upstream)
	}

	// 获取下游
	downstream, err := dl.GetDownstream("lineage-1", "n1")
	if err != nil {
		t.Fatalf("GetDownstream failed: %v", err)
	}
	if len(downstream) < 1 {
		t.Error("Expected at least 1 downstream node")
	}

	// 不存在的lineage
	_, err = dl.GetUpstream("nonexistent", "n1")
	if err == nil {
		t.Error("Expected error for nonexistent lineage")
	}
}

func TestDataLakeQuality(t *testing.T) {
	dl := NewDataLake()
	dl.AddSource(&DataSource{ID: "src1", Name: "test", Type: StorageLocal})
	obj := &DataObject{SourceID: "src1", Path: "/data/test.csv", Name: "test.csv"}
	dl.RegisterObject(obj)

	// 添加质量规则
	rule1 := &QualityRule{
		Name:        "not-null-check",
		Description: "检查必填字段",
		Type:        QualityNotNull,
		Field:       "id",
		Severity:    SeverityCritical,
		Enabled:     true,
	}
	rule2 := &QualityRule{
		Name:        "unique-check",
		Description: "检查唯一性",
		Type:        QualityUnique,
		Field:       "id",
		Severity:    SeverityHigh,
		Enabled:     true,
	}
	if err := dl.AddQualityRule(rule1); err != nil {
		t.Fatalf("AddQualityRule failed: %v", err)
	}
	if err := dl.AddQualityRule(rule2); err != nil {
		t.Fatalf("AddQualityRule failed: %v", err)
	}

	// 列出规则
	rules := dl.ListQualityRules()
	if len(rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rules))
	}

	// 获取规则
	got, ok := dl.GetQualityRule(rule1.ID)
	if !ok || got.Name != "not-null-check" {
		t.Error("GetQualityRule failed")
	}

	// 执行质量检查
	report, err := dl.RunQualityCheck(obj.ID, []string{rule1.ID, rule2.ID})
	if err != nil {
		t.Fatalf("RunQualityCheck failed: %v", err)
	}
	if len(report.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(report.Results))
	}
	if report.OverallScore <= 0 {
		t.Error("Expected positive overall score")
	}

	// 获取质量结果
	results := dl.GetQualityResults(obj.ID)
	if len(results) != 2 {
		t.Errorf("Expected 2 quality results, got %d", len(results))
	}

	// 检查不存在的对象
	_, err = dl.RunQualityCheck("nonexistent", []string{rule1.ID})
	if err == nil {
		t.Error("Expected error for nonexistent object")
	}

	// 移除规则
	if err := dl.RemoveQualityRule(rule1.ID); err != nil {
		t.Fatalf("RemoveQualityRule failed: %v", err)
	}
	rules = dl.ListQualityRules()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule after removal, got %d", len(rules))
	}

	// 移除不存在的规则
	if err := dl.RemoveQualityRule("nonexistent"); err == nil {
		t.Error("Expected error for removing nonexistent rule")
	}
}

func TestDataLakeStats(t *testing.T) {
	dl := NewDataLake()

	// 添加数据源
	dl.AddSource(&DataSource{ID: "s1", Name: "s3", Type: StorageS3, Capacity: 1000, Used: 500})
	dl.AddSource(&DataSource{ID: "s2", Name: "nfs", Type: StorageNFS, Capacity: 2000, Used: 800})

	// 添加对象
	obj := &DataObject{SourceID: "s1", Path: "/a.csv", Name: "a.csv"}
	dl.RegisterObject(obj)

	// 添加目录
	dl.AddCatalogEntry(&CatalogEntry{ObjectID: obj.ID, Name: "ds1", QualityScore: 90.0})

	// 添加血缘
	dl.CreateLineage(&LineageGraph{
		Nodes: []*LineageNode{{ID: "n1", ObjectID: obj.ID, Name: "a", Type: "source"}},
	})

	// 添加规则
	dl.AddQualityRule(&QualityRule{Name: "r1", Type: QualityNotNull})

	stats := dl.GetStats()
	if stats.TotalSources != 2 {
		t.Errorf("Expected 2 sources, got %d", stats.TotalSources)
	}
	if stats.OnlineSources != 2 {
		t.Errorf("Expected 2 online sources, got %d", stats.OnlineSources)
	}
	if stats.TotalObjects != 1 {
		t.Errorf("Expected 1 object, got %d", stats.TotalObjects)
	}
	if stats.TotalCatalogs != 1 {
		t.Errorf("Expected 1 catalog, got %d", stats.TotalCatalogs)
	}
	if stats.TotalLineages != 1 {
		t.Errorf("Expected 1 lineage, got %d", stats.TotalLineages)
	}
	if stats.TotalRules != 1 {
		t.Errorf("Expected 1 rule, got %d", stats.TotalRules)
	}
	if stats.TotalSize != 1300 {
		t.Errorf("Expected total size 1300, got %d", stats.TotalSize)
	}
	if stats.AvgQualityScore != 90.0 {
		t.Errorf("Expected avg quality score 90.0, got %f", stats.AvgQualityScore)
	}
}

func TestConcurrency(t *testing.T) {
	dl := NewDataLake()
	done := make(chan bool, 20)

	// 并发添加数据源
	for i := 0; i < 10; i++ {
		go func(id int) {
			dl.AddSource(&DataSource{
				ID:   fmt.Sprintf("src-%d", id),
				Name: fmt.Sprintf("source-%d", id),
				Type: StorageLocal,
			})
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			dl.ListSources()
			dl.GetStats()
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	sources := dl.ListSources()
	if len(sources) != 10 {
		t.Errorf("Expected 10 sources, got %d", len(sources))
	}
}

func TestDataObjectAccessTime(t *testing.T) {
	dl := NewDataLake()
	dl.AddSource(&DataSource{ID: "src1", Name: "test", Type: StorageLocal})

	before := time.Now()
	obj := &DataObject{SourceID: "src1", Path: "/test", Name: "test"}
	dl.RegisterObject(obj)
	after := time.Now()

	if obj.CreatedAt.Before(before) || obj.CreatedAt.After(after) {
		t.Error("CreatedAt not in expected range")
	}
	if obj.AccessedAt.Before(before) || obj.AccessedAt.After(after) {
		t.Error("AccessedAt not in expected range")
	}
}

package nasbenchmark

import (
	"testing"
)

func TestRunBenchmark(t *testing.T) {
	m := NewManager()

	config := BenchmarkConfig{
		Type:      BenchmarkSequentialRead,
		Path:      "/tmp/test",
		BlockSize: 1024,
		FileSize:  1024,
		Threads:   1,
	}

	result, err := m.RunBenchmark(config)
	if err != nil {
		t.Fatalf("Failed to run benchmark: %v", err)
	}

	if result.Status != "completed" {
		t.Errorf("Expected status completed, got %s", result.Status)
	}

	if result.Throughput <= 0 {
		t.Errorf("Expected positive throughput, got %f", result.Throughput)
	}

	if result.IOPS <= 0 {
		t.Errorf("Expected positive IOPS, got %f", result.IOPS)
	}
}

func TestRunSuite(t *testing.T) {
	m := NewManager()

	configs := []BenchmarkConfig{
		{Type: BenchmarkSequentialRead, Path: "/tmp/test", BlockSize: 1024, FileSize: 1024, Threads: 1},
		{Type: BenchmarkSequentialWrite, Path: "/tmp/test", BlockSize: 1024, FileSize: 1024, Threads: 1},
		{Type: BenchmarkRandomRead, Path: "/tmp/test", BlockSize: 4, FileSize: 1024, Threads: 1},
	}

	suite, err := m.RunSuite("Test Suite", configs)
	if err != nil {
		t.Fatalf("Failed to run suite: %v", err)
	}

	if suite.Summary.TotalTests != 3 {
		t.Errorf("Expected 3 total tests, got %d", suite.Summary.TotalTests)
	}

	if suite.Summary.PassedTests != 3 {
		t.Errorf("Expected 3 passed tests, got %d", suite.Summary.PassedTests)
	}

	if suite.Summary.AvgThroughput <= 0 {
		t.Errorf("Expected positive avg throughput, got %f", suite.Summary.AvgThroughput)
	}
}

func TestCompareResults(t *testing.T) {
	m := NewManager()

	config1 := BenchmarkConfig{Type: BenchmarkSequentialRead, Path: "/tmp/test1", BlockSize: 1024, FileSize: 1024, Threads: 1}
	config2 := BenchmarkConfig{Type: BenchmarkSequentialRead, Path: "/tmp/test2", BlockSize: 1024, FileSize: 1024, Threads: 1}

	r1, _ := m.RunBenchmark(config1)
	r2, _ := m.RunBenchmark(config2)

	comparison, err := m.CompareResults(r1.ID, r2.ID)
	if err != nil {
		t.Fatalf("Failed to compare results: %v", err)
	}

	if comparison["result1"] == nil {
		t.Error("Expected result1 in comparison")
	}

	if comparison["result2"] == nil {
		t.Error("Expected result2 in comparison")
	}
}

func TestGetResult(t *testing.T) {
	m := NewManager()

	config := BenchmarkConfig{Type: BenchmarkCPU, Path: "", BlockSize: 0, FileSize: 0, Threads: 1}
	result, _ := m.RunBenchmark(config)

	fetched, err := m.GetResult(result.ID)
	if err != nil {
		t.Fatalf("Failed to get result: %v", err)
	}

	if fetched.ID != result.ID {
		t.Errorf("Expected ID %s, got %s", result.ID, fetched.ID)
	}
}

func TestListResults(t *testing.T) {
	m := NewManager()

	config1 := BenchmarkConfig{Type: BenchmarkCPU, Path: "", BlockSize: 0, FileSize: 0, Threads: 1}
	config2 := BenchmarkConfig{Type: BenchmarkMemory, Path: "", BlockSize: 0, FileSize: 0, Threads: 1}

	m.RunBenchmark(config1)
	m.RunBenchmark(config2)

	results := m.ListResults()
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestGetSuite(t *testing.T) {
	m := NewManager()

	configs := []BenchmarkConfig{
		{Type: BenchmarkCPU, Path: "", BlockSize: 0, FileSize: 0, Threads: 1},
	}

	suite, _ := m.RunSuite("Test", configs)

	fetched, err := m.GetSuite(suite.ID)
	if err != nil {
		t.Fatalf("Failed to get suite: %v", err)
	}

	if fetched.ID != suite.ID {
		t.Errorf("Expected ID %s, got %s", suite.ID, fetched.ID)
	}
}

func TestListSuites(t *testing.T) {
	m := NewManager()

	configs := []BenchmarkConfig{
		{Type: BenchmarkCPU, Path: "", BlockSize: 0, FileSize: 0, Threads: 1},
	}

	m.RunSuite("Suite1", configs)
	m.RunSuite("Suite2", configs)

	suites := m.ListSuites()
	if len(suites) != 2 {
		t.Errorf("Expected 2 suites, got %d", len(suites))
	}
}

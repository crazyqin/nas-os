package digitaltwin

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	dt := New(Config{})
	if dt == nil {
		t.Fatal("New 返回 nil")
	}
	if dt.state != StateIdle {
		t.Fatalf("期望状态=idle, got %s", dt.state)
	}
}

func TestRegisterComponent(t *testing.T) {
	dt := New(Config{})
	comp := &Component{
		ID:       "cpu1",
		Type:     ComponentCPU,
		Name:     "CPU",
		Capacity: 100,
	}

	err := dt.RegisterComponent(comp)
	if err != nil {
		t.Fatalf("注册组件失败: %v", err)
	}
}

func TestRegisterComponentEmptyID(t *testing.T) {
	dt := New(Config{})
	err := dt.RegisterComponent(&Component{Name: "test"})
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestUpdateComponentUsage(t *testing.T) {
	dt := New(Config{})
	dt.RegisterComponent(&Component{ID: "cpu1", Type: ComponentCPU, Capacity: 100})

	err := dt.UpdateComponentUsage("cpu1", 75)
	if err != nil {
		t.Fatalf("更新使用率失败: %v", err)
	}
}

func TestUpdateComponentUsageNotFound(t *testing.T) {
	dt := New(Config{})
	err := dt.UpdateComponentUsage("nonexistent", 50)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestCreateScenario(t *testing.T) {
	dt := New(Config{})
	scenario := &SimulationScenario{
		ID:       "sc1",
		Name:     "负载测试",
		Duration: time.Hour,
	}

	err := dt.CreateScenario(scenario)
	if err != nil {
		t.Fatalf("创建场景失败: %v", err)
	}
}

func TestRunSimulation(t *testing.T) {
	dt := New(Config{})
	dt.RegisterComponent(&Component{ID: "cpu1", Type: ComponentCPU, Capacity: 100, Usage: 50})
	dt.CreateScenario(&SimulationScenario{ID: "sc1", Duration: time.Hour})

	result, err := dt.RunSimulation("sc1")
	if err != nil {
		t.Fatalf("运行模拟失败: %v", err)
	}
	if !result.Success {
		t.Fatal("模拟应该成功")
	}
}

func TestRunSimulationNotFound(t *testing.T) {
	dt := New(Config{})
	_, err := dt.RunSimulation("nonexistent")
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestPredictMetric(t *testing.T) {
	dt := New(Config{})
	dt.RegisterComponent(&Component{ID: "cpu1", Type: ComponentCPU, Capacity: 100, Usage: 50})

	pred, err := dt.PredictMetric("cpu1", "utilization", 24*time.Hour)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}
	if pred.Confidence < 0 || pred.Confidence > 1 {
		t.Fatalf("置信度超出范围: %f", pred.Confidence)
	}
}

func TestGetBottlenecks(t *testing.T) {
	dt := New(Config{})
	dt.RegisterComponent(&Component{ID: "cpu1", Type: ComponentCPU, Capacity: 100, Usage: 90})

	bottlenecks := dt.GetBottlenecks()
	if len(bottlenecks) != 1 {
		t.Fatalf("期望 1 个瓶颈, got %d", len(bottlenecks))
	}
}

func TestGetState(t *testing.T) {
	dt := New(Config{})
	if dt.GetState() != StateIdle {
		t.Fatal("初始状态应为 idle")
	}
}

func TestGetStats(t *testing.T) {
	dt := New(Config{})
	dt.RegisterComponent(&Component{ID: "cpu1", Type: ComponentCPU, Capacity: 100})

	stats := dt.GetStats()
	if stats["components"] != 1 {
		t.Fatalf("期望 components=1, got %v", stats["components"])
	}
}

func TestSync(t *testing.T) {
	dt := New(Config{})
	dt.Sync()
	if dt.lastSync.IsZero() {
		t.Fatal("lastSync 未更新")
	}
}

package containerhealth

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if len(m.ListContainers()) != 0 {
		t.Fatal("新管理器应该没有容器")
	}
}

func TestRegisterContainer(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 8080, Interval: 30, Timeout: 5}

	err := m.RegisterContainer("c1", "web", cfg, true)
	if err != nil {
		t.Fatalf("注册容器失败: %v", err)
	}

	containers := m.ListContainers()
	if len(containers) != 1 {
		t.Fatalf("期望1个容器，实际 %d", len(containers))
	}
	if containers[0].Name != "web" {
		t.Fatalf("期望容器名 web，实际 %s", containers[0].Name)
	}

	// 重复注册应报错
	err = m.RegisterContainer("c1", "web2", cfg, false)
	if err == nil {
		t.Fatal("重复注册应报错")
	}
}

func TestUnregisterContainer(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 8080}
	m.RegisterContainer("c1", "web", cfg, false)

	err := m.UnregisterContainer("c1")
	if err != nil {
		t.Fatalf("注销失败: %v", err)
	}

	if len(m.ListContainers()) != 0 {
		t.Fatal("注销后应无容器")
	}

	// 注销不存在的容器应报错
	err = m.UnregisterContainer("c99")
	if err == nil {
		t.Fatal("注销不存在的容器应报错")
	}
}

func TestCheckHealth(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 1, Timeout: 1} // 端口1通常不通
	m.RegisterContainer("c1", "test", cfg, false)

	result, err := m.CheckHealth("c1")
	if err != nil {
		t.Fatalf("检查健康失败: %v", err)
	}
	if result.FailCount != 1 {
		t.Fatalf("期望失败次数1，实际 %d", result.FailCount)
	}
	if result.LastCheck.IsZero() {
		t.Fatal("LastCheck 不应为零值")
	}

	// 不存在的容器
	_, err = m.CheckHealth("c99")
	if err == nil {
		t.Fatal("检查不存在的容器应报错")
	}
}

func TestCheckAllHealth(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 1, Timeout: 1}
	m.RegisterContainer("c1", "a", cfg, false)
	m.RegisterContainer("c2", "b", cfg, false)

	results := m.CheckAllHealth()
	if len(results) != 2 {
		t.Fatalf("期望2个结果，实际 %d", len(results))
	}
}

func TestGetContainer(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "http", Endpoint: "http://localhost", Timeout: 1}
	m.RegisterContainer("c1", "web", cfg, true)

	c, err := m.GetContainer("c1")
	if err != nil {
		t.Fatalf("获取容器失败: %v", err)
	}
	if c.ContainerID != "c1" || c.Name != "web" || !c.AutoRestart {
		t.Fatal("容器信息不匹配")
	}

	_, err = m.GetContainer("c99")
	if err == nil {
		t.Fatal("获取不存在的容器应报错")
	}
}

func TestSetAutoRestart(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 8080}
	m.RegisterContainer("c1", "web", cfg, false)

	err := m.SetAutoRestart("c1", true)
	if err != nil {
		t.Fatalf("设置自动重启失败: %v", err)
	}

	c, _ := m.GetContainer("c1")
	if !c.AutoRestart {
		t.Fatal("自动重启应为 true")
	}

	err = m.SetAutoRestart("c99", true)
	if err == nil {
		t.Fatal("设置不存在的容器应报错")
	}
}

func TestRestartContainer(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 8080}
	m.RegisterContainer("c1", "web", cfg, false)

	// 先标记为 unhealthy
	m.mu.Lock()
	m.containers["c1"].Status = "unhealthy"
	m.containers["c1"].FailCount = 3
	m.mu.Unlock()

	err := m.RestartContainer("c1")
	if err != nil {
		t.Fatalf("重启容器失败: %v", err)
	}

	c, _ := m.GetContainer("c1")
	if c.RestartCount != 1 {
		t.Fatalf("期望重启次数1，实际 %d", c.RestartCount)
	}
	if c.Status != "starting" {
		t.Fatalf("重启后期望状态 starting，实际 %s", c.Status)
	}
	if c.FailCount != 0 {
		t.Fatal("重启后失败次数应为0")
	}

	// 重启不存在的容器
	err = m.RestartContainer("c99")
	if err == nil {
		t.Fatal("重启不存在的容器应报错")
	}
}

func TestGetHealthReport(t *testing.T) {
	m := NewManager()
	cfg := HealthCheckConfig{Type: "tcp", Endpoint: "localhost", Port: 8080}

	m.RegisterContainer("c1", "web", cfg, false)
	m.RegisterContainer("c2", "db", cfg, true)

	// 设置不同状态
	m.mu.Lock()
	m.containers["c1"].Status = "healthy"
	m.containers["c2"].Status = "unhealthy"
	m.containers["c2"].RestartCount = 2
	m.mu.Unlock()

	report := m.GetHealthReport()

	if report["total_containers"] != 2 {
		t.Fatalf("期望总容器数2，实际 %v", report["total_containers"])
	}
	if report["healthy"] != 1 {
		t.Fatalf("期望健康数1，实际 %v", report["healthy"])
	}
	if report["unhealthy"] != 1 {
		t.Fatalf("期望不健康数1，实际 %v", report["unhealthy"])
	}
	if report["total_restarts"] != 2 {
		t.Fatalf("期望总重启数2，实际 %v", report["total_restarts"])
	}
	if _, ok := report["timestamp"].(time.Time); !ok {
		t.Fatal("timestamp 应为 time.Time 类型")
	}
}

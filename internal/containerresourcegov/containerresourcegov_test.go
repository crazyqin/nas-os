package containerresourcegov

import (
	"fmt"
	"log/slog"
	"testing"
	"time"
)

func TestNewResourceGovernor(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		governor := NewResourceGovernor(nil, nil)
		if governor == nil {
			t.Fatal("expected non-nil governor")
		}
		if governor.config == nil {
			t.Fatal("expected default config")
		}
		if governor.config.MonitoringInterval != 30*time.Second {
			t.Errorf("expected 30s monitoring interval, got %v", governor.config.MonitoringInterval)
		}
		governor.Stop()
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &GovernorConfig{
			MonitoringInterval: 10 * time.Second,
			PredictionWindow:   30 * time.Minute,
			AutoRemediate:      true,
			DryRun:             false,
			AlertThreshold:     0.9,
		}
		logger := slog.Default()
		governor := NewResourceGovernor(config, logger)
		if governor == nil {
			t.Fatal("expected non-nil governor")
		}
		if governor.config.MonitoringInterval != 10*time.Second {
			t.Errorf("expected 10s monitoring interval, got %v", governor.config.MonitoringInterval)
		}
		governor.Stop()
	})
}

func TestRegisterContainer(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	t.Run("register valid container", func(t *testing.T) {
		container := &Container{
			ID:    "test-1",
			Name:  "test-container",
			Image: "nginx:latest",
		}
		err := governor.RegisterContainer(container)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if governor.metrics.TotalContainers != 1 {
			t.Errorf("expected 1 total container, got %d", governor.metrics.TotalContainers)
		}
	})

	t.Run("register nil container", func(t *testing.T) {
		err := governor.RegisterContainer(nil)
		if err != ErrInvalidContainer {
			t.Errorf("expected ErrInvalidContainer, got %v", err)
		}
	})

	t.Run("register container without ID", func(t *testing.T) {
		container := &Container{
			Name: "no-id-container",
		}
		err := governor.RegisterContainer(container)
		if err != ErrContainerIDRequired {
			t.Errorf("expected ErrContainerIDRequired, got %v", err)
		}
	})

	t.Run("register duplicate container", func(t *testing.T) {
		container := &Container{
			ID: "test-1",
		}
		err := governor.RegisterContainer(container)
		if err != ErrContainerAlreadyExists {
			t.Errorf("expected ErrContainerAlreadyExists, got %v", err)
		}
	})
}

func TestUpdateResourceUsage(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器
	container := &Container{
		ID:   "test-1",
		Name: "test-container",
	}
	governor.RegisterContainer(container)

	t.Run("update existing container", func(t *testing.T) {
		err := governor.UpdateResourceUsage("test-1", 0.5, 0.6, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		c, _ := governor.GetContainer("test-1")
		if c.CPU.Current != 0.5 {
			t.Errorf("expected CPU current 0.5, got %f", c.CPU.Current)
		}
		if c.Memory.Current != 0.6 {
			t.Errorf("expected Memory current 0.6, got %f", c.Memory.Current)
		}
	})

	t.Run("update peak values", func(t *testing.T) {
		governor.UpdateResourceUsage("test-1", 0.8, 0.9, nil, nil)
		c, _ := governor.GetContainer("test-1")
		if c.CPU.Peak != 0.8 {
			t.Errorf("expected CPU peak 0.8, got %f", c.CPU.Peak)
		}
		if c.Memory.Peak != 0.9 {
			t.Errorf("expected Memory peak 0.9, got %f", c.Memory.Peak)
		}
	})

	t.Run("update non-existing container", func(t *testing.T) {
		err := governor.UpdateResourceUsage("non-existing", 0.5, 0.5, nil, nil)
		if err != ErrContainerNotFound {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})
}

func TestPredictResources(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器并添加采样数据
	container := &Container{
		ID: "test-1",
	}
	governor.RegisterContainer(container)

	t.Run("insufficient data", func(t *testing.T) {
		_, err := governor.PredictResources("test-1", time.Hour)
		if err != ErrInsufficientData {
			t.Errorf("expected ErrInsufficientData, got %v", err)
		}
	})

	t.Run("with sufficient data", func(t *testing.T) {
		// 添加多个采样
		for i := 0; i < 5; i++ {
			governor.UpdateResourceUsage("test-1", float64(i)*0.1, float64(i)*0.2, nil, nil)
		}

		prediction, err := governor.PredictResources("test-1", time.Hour)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prediction == nil {
			t.Fatal("expected non-nil prediction")
		}
		if prediction.ContainerID != "test-1" {
			t.Errorf("expected container ID test-1, got %s", prediction.ContainerID)
		}
		if prediction.Confidence != 0.85 {
			t.Errorf("expected confidence 0.85, got %f", prediction.Confidence)
		}
		if prediction.Method != "linear_regression" {
			t.Errorf("expected method linear_regression, got %s", prediction.Method)
		}
	})
}

func TestApplyProfile(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器
	container := &Container{
		ID: "test-1",
		CPU: &ResourceUsage{},
		Memory: &ResourceUsage{},
	}
	governor.RegisterContainer(container)

	// 注册配置文件
	profile := &ResourceProfile{
		ID:   "profile-1",
		Name: "standard",
		CPU: &ResourceLimit{
			Min:     0.1,
			Max:     2.0,
			Default: 0.5,
		},
		Memory: &ResourceLimit{
			Min:     128,
			Max:     2048,
			Default: 512,
		},
		Priority: PriorityNormal,
	}
	governor.RegisterProfile(profile)

	t.Run("apply existing profile", func(t *testing.T) {
		err := governor.ApplyProfile("test-1", "profile-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		c, _ := governor.GetContainer("test-1")
		if c.CPU.Limit != 2.0 {
			t.Errorf("expected CPU limit 2.0, got %f", c.CPU.Limit)
		}
		if c.Memory.Limit != 2048 {
			t.Errorf("expected Memory limit 2048, got %f", c.Memory.Limit)
		}
		if c.Profile != "profile-1" {
			t.Errorf("expected profile profile-1, got %s", c.Profile)
		}
	})

	t.Run("apply to non-existing container", func(t *testing.T) {
		err := governor.ApplyProfile("non-existing", "profile-1")
		if err != ErrContainerNotFound {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})

	t.Run("apply non-existing profile", func(t *testing.T) {
		err := governor.ApplyProfile("test-1", "non-existing")
		if err != ErrProfileNotFound {
			t.Errorf("expected ErrProfileNotFound, got %v", err)
		}
	})
}

func TestEvaluatePolicies(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器
	container := &Container{
		ID: "test-1",
		CPU: &ResourceUsage{
			Current: 0.9,
		},
		Memory: &ResourceUsage{
			Current: 0.5,
		},
	}
	governor.RegisterContainer(container)

	// 注册策略
	policy := &GovernancePolicy{
		ID:   "policy-1",
		Name: "cpu-high",
		Type: PolicyTypeResource,
		Conditions: []*PolicyCondition{
			{
				Metric:    "cpu",
				Operator:  ">",
				Threshold: 0.8,
			},
		},
		Actions: []*PolicyAction{
			{
				Type: ActionTypeAlert,
			},
		},
		Enabled: true,
	}
	governor.RegisterPolicy(policy)

	t.Run("evaluate with violations", func(t *testing.T) {
		violations := governor.EvaluatePolicies()
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation, got %d", len(violations))
		}
		if violations[0].PolicyID != "policy-1" {
			t.Errorf("expected policy-1, got %s", violations[0].PolicyID)
		}
		if violations[0].ContainerID != "test-1" {
			t.Errorf("expected test-1, got %s", violations[0].ContainerID)
		}
	})

	t.Run("evaluate disabled policy", func(t *testing.T) {
		policy.Enabled = false
		violations := governor.EvaluatePolicies()
		if len(violations) != 0 {
			t.Errorf("expected 0 violations, got %d", len(violations))
		}
		policy.Enabled = true
	})

	t.Run("evaluate with compliant container", func(t *testing.T) {
		container.CPU.Current = 0.5
		violations := governor.EvaluatePolicies()
		if len(violations) != 0 {
			t.Errorf("expected 0 violations, got %d", len(violations))
		}
	})
}

func TestAutoRemediate(t *testing.T) {
	t.Run("disabled auto-remediate", func(t *testing.T) {
		config := &GovernorConfig{
			MonitoringInterval: 30 * time.Second,
			AutoRemediate:      false,
			DryRun:             true,
		}
		governor := NewResourceGovernor(config, nil)
		defer governor.Stop()

		violations := []*PolicyViolation{
			{
				PolicyID:    "policy-1",
				ContainerID: "test-1",
			},
		}
		results := governor.AutoRemediate(violations)
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("dry run mode", func(t *testing.T) {
		config := &GovernorConfig{
			MonitoringInterval: 30 * time.Second,
			AutoRemediate:      true,
			DryRun:             true,
		}
		governor := NewResourceGovernor(config, nil)
		defer governor.Stop()

		policy := &GovernancePolicy{
			ID: "policy-1",
			Actions: []*PolicyAction{
				{Type: ActionTypeScale},
			},
		}
		governor.RegisterPolicy(policy)

		violations := []*PolicyViolation{
			{
				PolicyID:    "policy-1",
				ContainerID: "test-1",
			},
		}
		results := governor.AutoRemediate(violations)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Success {
			t.Error("expected success in dry run")
		}
	})

	t.Run("execute scale action", func(t *testing.T) {
		config := &GovernorConfig{
			MonitoringInterval: 30 * time.Second,
			AutoRemediate:      true,
			DryRun:             false,
		}
		governor := NewResourceGovernor(config, nil)
		defer governor.Stop()

		policy := &GovernancePolicy{
			ID: "policy-1",
			Actions: []*PolicyAction{
				{Type: ActionTypeScale},
			},
		}
		governor.RegisterPolicy(policy)

		violations := []*PolicyViolation{
			{
				PolicyID:    "policy-1",
				ContainerID: "test-1",
			},
		}
		results := governor.AutoRemediate(violations)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if !results[0].Success {
			t.Error("expected success")
		}
		if governor.metrics.AutoScaled != 1 {
			t.Errorf("expected 1 auto-scaled, got %d", governor.metrics.AutoScaled)
		}
	})
}

func TestGetEfficiencyReport(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册运行中的容器
	container := &Container{
		ID:     "test-1",
		Status: ContainerRunning,
		CPU: &ResourceUsage{
			Current: 0.5,
			Limit:   1.0,
		},
		Memory: &ResourceUsage{
			Current: 512,
			Limit:   1024,
		},
	}
	governor.RegisterContainer(container)

	report := governor.GetEfficiencyReport()
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.OverallEfficiency != 0.5 {
		t.Errorf("expected overall efficiency 0.5, got %f", report.OverallEfficiency)
	}
	if report.WastedCPU != 0.5 {
		t.Errorf("expected wasted CPU 0.5, got %f", report.WastedCPU)
	}
	if report.WastedMemory != 512 {
		t.Errorf("expected wasted memory 512, got %f", report.WastedMemory)
	}
}

func TestGetMetrics(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器
	container := &Container{
		ID: "test-1",
		CPU: &ResourceUsage{
			Current: 0.5,
			Limit:   1.0,
		},
		Memory: &ResourceUsage{
			Current: 512,
			Limit:   1024,
		},
	}
	governor.RegisterContainer(container)

	metrics := governor.GetMetrics()
	if metrics == nil {
		t.Fatal("expected non-nil metrics")
	}
	if metrics.TotalContainers != 1 {
		t.Errorf("expected 1 total container, got %d", metrics.TotalContainers)
	}
	if metrics.Compliant != 1 {
		t.Errorf("expected 1 compliant, got %d", metrics.Compliant)
	}
}

func TestListContainers(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册多个容器
	for i := 0; i < 3; i++ {
		container := &Container{
			ID:   fmt.Sprintf("test-%d", i),
			Name: fmt.Sprintf("container-%d", i),
		}
		governor.RegisterContainer(container)
	}

	containers := governor.ListContainers()
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
}

func TestRemoveContainer(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	// 注册容器
	container := &Container{
		ID: "test-1",
	}
	governor.RegisterContainer(container)

	t.Run("remove existing container", func(t *testing.T) {
		err := governor.RemoveContainer("test-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if governor.metrics.TotalContainers != 0 {
			t.Errorf("expected 0 total containers, got %d", governor.metrics.TotalContainers)
		}
	})

	t.Run("remove non-existing container", func(t *testing.T) {
		err := governor.RemoveContainer("non-existing")
		if err != ErrContainerNotFound {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})
}

func TestGetContainer(t *testing.T) {
	governor := NewResourceGovernor(nil, nil)
	defer governor.Stop()

	container := &Container{
		ID:   "test-1",
		Name: "test-container",
	}
	governor.RegisterContainer(container)

	t.Run("get existing container", func(t *testing.T) {
		c, err := governor.GetContainer("test-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.ID != "test-1" {
			t.Errorf("expected ID test-1, got %s", c.ID)
		}
	})

	t.Run("get non-existing container", func(t *testing.T) {
		_, err := governor.GetContainer("non-existing")
		if err != ErrContainerNotFound {
			t.Errorf("expected ErrContainerNotFound, got %v", err)
		}
	})
}



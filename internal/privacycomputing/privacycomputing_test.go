package privacycomputing

import (
	"testing"
)

func TestNewPrivacyEngine(t *testing.T) {
	engine := NewPrivacyEngine()
	if engine == nil {
		t.Fatal("Failed to create PrivacyEngine")
	}
	if engine.federatedMgr == nil {
		t.Fatal("FederatedManager is nil")
	}
	if engine.mpcMgr == nil {
		t.Fatal("MPCManager is nil")
	}
	if engine.differentialMgr == nil {
		t.Fatal("DifferentialManager is nil")
	}
	if engine.maskingMgr == nil {
		t.Fatal("MaskingManager is nil")
	}
}

// ==================== 联邦学习测试 ====================

func TestFederatedCreateTask(t *testing.T) {
	mgr := NewFederatedManager()

	req := CreateFederatedTaskRequest{
		Name:      "测试联邦任务",
		ModelType: "linear",
		MaxRounds: 5,
		Participants: []ParticipantRequest{
			{ID: "p1", Name: "参与方1"},
			{ID: "p2", Name: "参与方2"},
		},
		Config: FederatedConfig{
			AggregationStrategy: "fedavg",
			LearningRate:        0.01,
			BatchSize:           32,
			LocalEpochs:         5,
			MinParticipants:     2,
		},
	}

	task, err := mgr.CreateTask(req)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	if task.ID == "" {
		t.Fatal("Task ID is empty")
	}
	if task.Name != "测试联邦任务" {
		t.Fatalf("Expected name '测试联邦任务', got '%s'", task.Name)
	}
	if task.Status != "pending" {
		t.Fatalf("Expected status 'pending', got '%s'", task.Status)
	}
	if len(task.Participants) != 2 {
		t.Fatalf("Expected 2 participants, got %d", len(task.Participants))
	}
}

func TestFederatedGetTask(t *testing.T) {
	mgr := NewFederatedManager()

	req := CreateFederatedTaskRequest{
		Name:      "测试任务",
		ModelType: "linear",
		MaxRounds: 3,
	}

	task, _ := mgr.CreateTask(req)

	// 获取任务
	fetched, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}
	if fetched.ID != task.ID {
		t.Fatalf("Expected task ID %s, got %s", task.ID, fetched.ID)
	}

	// 获取不存在的任务
	_, err = mgr.GetTask("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
}

func TestFederatedListTasks(t *testing.T) {
	mgr := NewFederatedManager()

	// 创建多个任务
	for i := 0; i < 3; i++ {
		req := CreateFederatedTaskRequest{
			Name:      "任务",
			ModelType: "linear",
		}
		mgr.CreateTask(req)
	}

	tasks := mgr.ListTasks()
	if len(tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestFederatedStartTraining(t *testing.T) {
	mgr := NewFederatedManager()

	req := CreateFederatedTaskRequest{
		Name:      "训练任务",
		ModelType: "linear",
		MaxRounds: 2,
		Participants: []ParticipantRequest{
			{ID: "p1", Name: "参与方1"},
			{ID: "p2", Name: "参与方2"},
		},
		Config: FederatedConfig{
			MinParticipants: 2,
		},
	}

	task, _ := mgr.CreateTask(req)

	// 开始训练
	err := mgr.StartTraining(task.ID)
	if err != nil {
		t.Fatalf("Failed to start training: %v", err)
	}

	// 检查状态
	fetched, _ := mgr.GetTask(task.ID)
	if fetched.Status != "training" {
		t.Fatalf("Expected status 'training', got '%s'", fetched.Status)
	}
}

func TestFederatedDeleteTask(t *testing.T) {
	mgr := NewFederatedManager()

	req := CreateFederatedTaskRequest{
		Name:      "待删除任务",
		ModelType: "linear",
	}

	task, _ := mgr.CreateTask(req)

	err := mgr.DeleteTask(task.ID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// 确认已删除
	_, err = mgr.GetTask(task.ID)
	if err == nil {
		t.Fatal("Expected error for deleted task")
	}
}

// ==================== MPC测试 ====================

func TestMPCCreateProtocol(t *testing.T) {
	mgr := NewMPCManager()

	req := CreateMPCProtocolRequest{
		Name: "测试MPC协议",
		Type: "secret_sharing",
		Participants: []MPCParticipantReq{
			{ID: "p1", Name: "参与方1", Role: "dealer"},
			{ID: "p2", Name: "参与方2", Role: "compute"},
			{ID: "p3", Name: "参与方3", Role: "verifier"},
		},
		Computation: "sum",
	}

	protocol, err := mgr.CreateProtocol(req)
	if err != nil {
		t.Fatalf("Failed to create protocol: %v", err)
	}

	if protocol.ID == "" {
		t.Fatal("Protocol ID is empty")
	}
	if protocol.Name != "测试MPC协议" {
		t.Fatalf("Expected name '测试MPC协议', got '%s'", protocol.Name)
	}
	if len(protocol.Participants) != 3 {
		t.Fatalf("Expected 3 participants, got %d", len(protocol.Participants))
	}
}

func TestMPCGetProtocol(t *testing.T) {
	mgr := NewMPCManager()

	req := CreateMPCProtocolRequest{
		Name: "协议",
		Type: "secret_sharing",
		Participants: []MPCParticipantReq{
			{ID: "p1", Name: "参与方1", Role: "dealer"},
			{ID: "p2", Name: "参与方2", Role: "compute"},
		},
	}

	protocol, _ := mgr.CreateProtocol(req)

	fetched, err := mgr.GetProtocol(protocol.ID)
	if err != nil {
		t.Fatalf("Failed to get protocol: %v", err)
	}
	if fetched.ID != protocol.ID {
		t.Fatalf("Expected protocol ID %s, got %s", protocol.ID, fetched.ID)
	}
}

func TestMPCListProtocols(t *testing.T) {
	mgr := NewMPCManager()

	for i := 0; i < 3; i++ {
		req := CreateMPCProtocolRequest{
			Name: "协议",
			Type: "secret_sharing",
			Participants: []MPCParticipantReq{
				{ID: "p1", Name: "参与方1", Role: "dealer"},
				{ID: "p2", Name: "参与方2", Role: "compute"},
			},
		}
		mgr.CreateProtocol(req)
	}

	protocols := mgr.ListProtocols()
	if len(protocols) != 3 {
		t.Fatalf("Expected 3 protocols, got %d", len(protocols))
	}
}

func TestMPCStartComputation(t *testing.T) {
	mgr := NewMPCManager()

	req := CreateMPCProtocolRequest{
		Name: "计算协议",
		Type: "secret_sharing",
		Participants: []MPCParticipantReq{
			{ID: "p1", Name: "参与方1", Role: "dealer"},
			{ID: "p2", Name: "参与方2", Role: "compute"},
		},
	}

	protocol, _ := mgr.CreateProtocol(req)

	err := mgr.StartComputation(protocol.ID)
	if err != nil {
		t.Fatalf("Failed to start computation: %v", err)
	}
}

func TestMPCDeleteProtocol(t *testing.T) {
	mgr := NewMPCManager()

	req := CreateMPCProtocolRequest{
		Name: "待删除协议",
		Type: "secret_sharing",
		Participants: []MPCParticipantReq{
			{ID: "p1", Name: "参与方1", Role: "dealer"},
			{ID: "p2", Name: "参与方2", Role: "compute"},
		},
	}

	protocol, _ := mgr.CreateProtocol(req)

	err := mgr.DeleteProtocol(protocol.ID)
	if err != nil {
		t.Fatalf("Failed to delete protocol: %v", err)
	}

	_, err = mgr.GetProtocol(protocol.ID)
	if err == nil {
		t.Fatal("Expected error for deleted protocol")
	}
}

func TestMPCSplitSecret(t *testing.T) {
	mgr := NewMPCManager()

	secret := []byte("my secret data")

	shares, err := mgr.SplitSecret(secret, 5, 3)
	if err != nil {
		t.Fatalf("Failed to split secret: %v", err)
	}

	if len(shares) != 5 {
		t.Fatalf("Expected 5 shares, got %d", len(shares))
	}

	for i, share := range shares {
		if share.Index != i+1 {
			t.Fatalf("Expected share index %d, got %d", i+1, share.Index)
		}
		if len(share.Value) == 0 {
			t.Fatal("Share value is empty")
		}
	}
}

func TestMPCReconstructSecret(t *testing.T) {
	mgr := NewMPCManager()

	secret := []byte("test secret")

	shares, err := mgr.SplitSecret(secret, 5, 3)
	if err != nil {
		t.Fatalf("Failed to split secret: %v", err)
	}

	// 使用3个份额重构
	selectedShares := shares[:3]
	reconstructed, err := mgr.ReconstructSecret(selectedShares)
	if err != nil {
		t.Fatalf("Failed to reconstruct secret: %v", err)
	}

	if len(reconstructed) == 0 {
		t.Fatal("Reconstructed secret is empty")
	}
}

// ==================== 差分隐私测试 ====================

func TestDifferentialNewManager(t *testing.T) {
	mgr := NewDifferentialManager()
	if mgr == nil {
		t.Fatal("Failed to create DifferentialManager")
	}

	config := mgr.GetConfig()
	if config.Epsilon != 1.0 {
		t.Fatalf("Expected epsilon 1.0, got %f", config.Epsilon)
	}
}

func TestDifferentialSetConfig(t *testing.T) {
	mgr := NewDifferentialManager()

	config := DifferentialPrivacyConfig{
		Epsilon:     0.5,
		Delta:       1e-6,
		Mechanism:   "gaussian",
		Sensitivity: 2.0,
	}

	err := mgr.SetConfig(config)
	if err != nil {
		t.Fatalf("Failed to set config: %v", err)
	}

	fetched := mgr.GetConfig()
	if fetched.Epsilon != 0.5 {
		t.Fatalf("Expected epsilon 0.5, got %f", fetched.Epsilon)
	}
	if fetched.Mechanism != "gaussian" {
		t.Fatalf("Expected mechanism 'gaussian', got '%s'", fetched.Mechanism)
	}
}

func TestDifferentialSetBudget(t *testing.T) {
	mgr := NewDifferentialManager()

	err := mgr.SetBudget(2.0, 1e-5)
	if err != nil {
		t.Fatalf("Failed to set budget: %v", err)
	}

	budget := mgr.GetBudget()
	if budget.TotalEpsilon != 2.0 {
		t.Fatalf("Expected total epsilon 2.0, got %f", budget.TotalEpsilon)
	}
	if budget.RemainingEpsilon != 2.0 {
		t.Fatalf("Expected remaining epsilon 2.0, got %f", budget.RemainingEpsilon)
	}
}

func TestDifferentialAddNoise(t *testing.T) {
	mgr := NewDifferentialManager()

	req := AddNoiseRequest{
		Data:      []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		QueryType: "test",
		Config: DifferentialPrivacyConfig{
			Epsilon:     1.0,
			Mechanism:   "laplace",
			Sensitivity: 1.0,
		},
	}

	response, err := mgr.AddNoise(req)
	if err != nil {
		t.Fatalf("Failed to add noise: %v", err)
	}

	if len(response.NoisyData) != 5 {
		t.Fatalf("Expected 5 noisy values, got %d", len(response.NoisyData))
	}
	if response.EpsilonUsed != 1.0 {
		t.Fatalf("Expected epsilon used 1.0, got %f", response.EpsilonUsed)
	}
}

func TestDifferentialBudgetExhaustion(t *testing.T) {
	mgr := NewDifferentialManager()
	mgr.SetBudget(0.5, 1e-5)

	req := AddNoiseRequest{
		Data:      []float64{1.0, 2.0},
		QueryType: "test",
		Config: DifferentialPrivacyConfig{
			Epsilon:     1.0,
			Mechanism:   "laplace",
			Sensitivity: 1.0,
		},
	}

	// 第一次查询应该失败（预算不足）
	_, err := mgr.AddNoise(req)
	if err == nil {
		t.Fatal("Expected error for insufficient budget")
	}
}

func TestDifferentialPrivateMean(t *testing.T) {
	mgr := NewDifferentialManager()

	data := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	mean, err := mgr.PrivateMean(data, 1.0)
	if err != nil {
		t.Fatalf("Failed to compute private mean: %v", err)
	}

	// 真实均值是3.0，噪声均值应该在附近
	if mean < 0 || mean > 6 {
		t.Fatalf("Mean %f is out of expected range [0, 6]", mean)
	}
}

func TestDifferentialPrivateHistogram(t *testing.T) {
	mgr := NewDifferentialManager()

	data := []int{0, 1, 1, 2, 2, 2, 3, 3, 3, 3}
	histogram, err := mgr.PrivateHistogram(data, 4, 1.0)
	if err != nil {
		t.Fatalf("Failed to compute private histogram: %v", err)
	}

	if len(histogram) != 4 {
		t.Fatalf("Expected 4 bins, got %d", len(histogram))
	}
}

func TestDifferentialPrivateCount(t *testing.T) {
	mgr := NewDifferentialManager()

	data := []bool{true, false, true, true, false}
	count, err := mgr.PrivateCount(data, 1.0)
	if err != nil {
		t.Fatalf("Failed to compute private count: %v", err)
	}

	// 真实计数是3，噪声计数应该在附近
	if count < 0 || count > 6 {
		t.Fatalf("Count %f is out of expected range [0, 6]", count)
	}
}

func TestDifferentialClipGradient(t *testing.T) {
	mgr := NewDifferentialManager()

	gradient := []float64{10.0, 20.0, 30.0}
	clipped := mgr.ClipGradient(gradient, 5.0)

	// 计算裁剪后的范数
	norm := 0.0
	for _, g := range clipped {
		norm += g * g
	}
	norm = norm * 0.5 // 简化计算

	if norm > 25.0 {
		t.Fatalf("Gradient not properly clipped")
	}
}

func TestDifferentialResetBudget(t *testing.T) {
	mgr := NewDifferentialManager()
	mgr.SetBudget(1.0, 1e-5)

	// 使用一些预算
	req := AddNoiseRequest{
		Data:      []float64{1.0},
		QueryType: "test",
		Config: DifferentialPrivacyConfig{
			Epsilon:     0.3,
			Mechanism:   "laplace",
			Sensitivity: 1.0,
		},
	}
	mgr.AddNoise(req)

	// 重置预算
	mgr.ResetBudget()

	budget := mgr.GetBudget()
	if budget.UsedEpsilon != 0 {
		t.Fatalf("Expected used epsilon 0, got %f", budget.UsedEpsilon)
	}
	if budget.RemainingEpsilon != 1.0 {
		t.Fatalf("Expected remaining epsilon 1.0, got %f", budget.RemainingEpsilon)
	}
}

func TestDifferentialGetQueryLogs(t *testing.T) {
	mgr := NewDifferentialManager()

	req := AddNoiseRequest{
		Data:      []float64{1.0, 2.0},
		QueryType: "test_query",
		Config: DifferentialPrivacyConfig{
			Epsilon:     0.1,
			Mechanism:   "laplace",
			Sensitivity: 1.0,
		},
	}
	mgr.AddNoise(req)

	logs := mgr.GetQueryLogs()
	if len(logs) == 0 {
		t.Fatal("Expected at least one query log")
	}
	if logs[0].QueryType != "test_query" {
		t.Fatalf("Expected query type 'test_query', got '%s'", logs[0].QueryType)
	}
}

// ==================== 数据脱敏测试 ====================

func TestMaskingNewManager(t *testing.T) {
	mgr := NewMaskingManager()
	if mgr == nil {
		t.Fatal("Failed to create MaskingManager")
	}

	rules := mgr.ListRules()
	if len(rules) < 5 {
		t.Fatalf("Expected at least 5 default rules, got %d", len(rules))
	}
}

func TestMaskingCreateRule(t *testing.T) {
	mgr := NewMaskingManager()

	req := CreateMaskRuleRequest{
		Name:     "自定义规则",
		Type:     "regex",
		Pattern:  `\d{6}`,
		Strategy: "mask",
		Config: map[string]interface{}{
			"mask_char": "#",
		},
	}

	rule, err := mgr.CreateRule(req)
	if err != nil {
		t.Fatalf("Failed to create rule: %v", err)
	}

	if rule.ID == "" {
		t.Fatal("Rule ID is empty")
	}
	if rule.Name != "自定义规则" {
		t.Fatalf("Expected name '自定义规则', got '%s'", rule.Name)
	}
}

func TestMaskingGetRule(t *testing.T) {
	mgr := NewMaskingManager()

	req := CreateMaskRuleRequest{
		Name:     "测试规则",
		Type:     "regex",
		Pattern:  `\d+`,
		Strategy: "hash",
	}

	rule, _ := mgr.CreateRule(req)

	fetched, err := mgr.GetRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to get rule: %v", err)
	}
	if fetched.ID != rule.ID {
		t.Fatalf("Expected rule ID %s, got %s", rule.ID, fetched.ID)
	}
}

func TestMaskingUpdateRule(t *testing.T) {
	mgr := NewMaskingManager()

	req := CreateMaskRuleRequest{
		Name:     "原始规则",
		Type:     "regex",
		Pattern:  `\d+`,
		Strategy: "mask",
	}

	rule, _ := mgr.CreateRule(req)

	updateReq := CreateMaskRuleRequest{
		Name: "更新后的规则",
	}

	updated, err := mgr.UpdateRule(rule.ID, updateReq)
	if err != nil {
		t.Fatalf("Failed to update rule: %v", err)
	}
	if updated.Name != "更新后的规则" {
		t.Fatalf("Expected name '更新后的规则', got '%s'", updated.Name)
	}
}

func TestMaskingDeleteRule(t *testing.T) {
	mgr := NewMaskingManager()

	req := CreateMaskRuleRequest{
		Name:     "待删除规则",
		Type:     "regex",
		Pattern:  `\d+`,
		Strategy: "mask",
	}

	rule, _ := mgr.CreateRule(req)

	err := mgr.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("Failed to delete rule: %v", err)
	}

	_, err = mgr.GetRule(rule.ID)
	if err == nil {
		t.Fatal("Expected error for deleted rule")
	}
}

func TestMaskingApplyMask(t *testing.T) {
	mgr := NewMaskingManager()

	req := ApplyMaskRequest{
		Content: "我的手机号是13800138000，邮箱是test@example.com",
	}

	result, err := mgr.ApplyMask(req)
	if err != nil {
		t.Fatalf("Failed to apply mask: %v", err)
	}

	if len(result.RulesApplied) == 0 {
		t.Fatal("No rules were applied")
	}
	if result.MaskedCount == 0 {
		t.Fatal("No content was masked")
	}
}

func TestMaskingApplyTableMask(t *testing.T) {
	mgr := NewMaskingManager()

	// 创建规则
	rule, _ := mgr.CreateRule(CreateMaskRuleRequest{
		Name:     "手机号规则",
		Type:     "regex",
		Pattern:  `1[3-9]\d{9}`,
		Strategy: "partial",
		Config: map[string]interface{}{
			"prefix_keep": 3,
			"suffix_keep": 4,
			"mask_char":   "*",
		},
	})

	req := ApplyTableMaskRequest{
		Table: "users",
		Data: []map[string]interface{}{
			{"name": "张三", "phone": "13800138000"},
			{"name": "李四", "phone": "13900139000"},
		},
		Rules: map[string]string{
			"phone": rule.ID,
		},
	}

	result, err := mgr.ApplyTableMask(req)
	if err != nil {
		t.Fatalf("Failed to apply table mask: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(result))
	}
}

func TestMaskingToggleRule(t *testing.T) {
	mgr := NewMaskingManager()

	req := CreateMaskRuleRequest{
		Name:     "可切换规则",
		Type:     "regex",
		Pattern:  `\d+`,
		Strategy: "mask",
	}

	rule, _ := mgr.CreateRule(req)

	// 禁用规则
	err := mgr.ToggleRule(rule.ID, false)
	if err != nil {
		t.Fatalf("Failed to toggle rule: %v", err)
	}

	fetched, _ := mgr.GetRule(rule.ID)
	if fetched.Enabled {
		t.Fatal("Expected rule to be disabled")
	}

	// 启用规则
	err = mgr.ToggleRule(rule.ID, true)
	if err != nil {
		t.Fatalf("Failed to toggle rule: %v", err)
	}

	fetched, _ = mgr.GetRule(rule.ID)
	if !fetched.Enabled {
		t.Fatal("Expected rule to be enabled")
	}
}

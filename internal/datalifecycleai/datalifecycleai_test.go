package datalifecycleai

import (
	"log/slog"
	"testing"
	"time"
)

func TestNewLifecycleEngine(t *testing.T) {
	logger := slog.Default()

	t.Run("valid config", func(t *testing.T) {
		config := &LifecycleConfig{
			ScanInterval:      1 * time.Hour,
			AutoExecute:       true,
			DryRun:            false,
			MaxConcurrent:     10,
			ComplianceMode:    true,
			AIDecisionEnabled: true,
		}

		engine, err := NewLifecycleEngine(config, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if engine == nil {
			t.Fatal("engine is nil")
		}
		if engine.config.MaxConcurrent != 10 {
			t.Errorf("expected MaxConcurrent 10, got %d", engine.config.MaxConcurrent)
		}
	})

	t.Run("nil config", func(t *testing.T) {
		_, err := NewLifecycleEngine(nil, logger)
		if err != ErrConfigInvalid {
			t.Errorf("expected ErrConfigInvalid, got %v", err)
		}
	})

	t.Run("default values", func(t *testing.T) {
		config := &LifecycleConfig{}

		engine, err := NewLifecycleEngine(config, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if engine.config.MaxConcurrent != 10 {
			t.Errorf("expected default MaxConcurrent 10, got %d", engine.config.MaxConcurrent)
		}
		if engine.config.ScanInterval != 1*time.Hour {
			t.Errorf("expected default ScanInterval 1h, got %v", engine.config.ScanInterval)
		}
	})
}

func TestAddPolicy(t *testing.T) {
	engine := createTestEngine()

	t.Run("add valid policy", func(t *testing.T) {
		policy := &LifecyclePolicy{
			ID:          "policy-1",
			Name:        "Archive Old Files",
			Description: "Archive files older than 180 days",
			Rules:       []string{"rule-1", "rule-2"},
			Priority:    1,
			Enabled:     true,
		}

		err := engine.AddPolicy(policy)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 检查策略已添加
		if _, exists := engine.policies["policy-1"]; !exists {
			t.Error("policy not found")
		}
	})

	t.Run("add duplicate policy", func(t *testing.T) {
		policy := &LifecyclePolicy{
			ID:   "policy-1",
			Name: "Duplicate",
		}

		err := engine.AddPolicy(policy)
		if err != ErrPolicyAlreadyExists {
			t.Errorf("expected ErrPolicyAlreadyExists, got %v", err)
		}
	})

	t.Run("add nil policy", func(t *testing.T) {
		err := engine.AddPolicy(nil)
		if err != ErrPolicyInvalid {
			t.Errorf("expected ErrPolicyInvalid, got %v", err)
		}
	})

	t.Run("add policy without ID", func(t *testing.T) {
		policy := &LifecyclePolicy{
			Name: "No ID",
		}

		err := engine.AddPolicy(policy)
		if err != ErrPolicyInvalid {
			t.Errorf("expected ErrPolicyInvalid, got %v", err)
		}
	})
}

func TestAddRule(t *testing.T) {
	engine := createTestEngine()

	t.Run("add valid rule", func(t *testing.T) {
		rule := &LifecycleRule{
			ID:   "rule-1",
			Name: "Archive after 180 days",
			Condition: &RuleCondition{
				Type:     ConditionAge,
				Operator: OpGreaterThan,
				Value:    180,
			},
			Action:  ActionArchive,
			Enabled: true,
		}

		err := engine.AddRule(rule)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, exists := engine.rules["rule-1"]; !exists {
			t.Error("rule not found")
		}
	})

	t.Run("add duplicate rule", func(t *testing.T) {
		rule := &LifecycleRule{
			ID:   "rule-1",
			Name: "Duplicate",
			Condition: &RuleCondition{
				Type:     ConditionAge,
				Operator: OpGreaterThan,
				Value:    365,
			},
		}

		err := engine.AddRule(rule)
		if err != ErrRuleAlreadyExists {
			t.Errorf("expected ErrRuleAlreadyExists, got %v", err)
		}
	})

	t.Run("add rule without condition", func(t *testing.T) {
		rule := &LifecycleRule{
			ID:   "rule-2",
			Name: "No Condition",
		}

		err := engine.AddRule(rule)
		if err != ErrRuleInvalid {
			t.Errorf("expected ErrRuleInvalid, got %v", err)
		}
	})
}

func TestRegisterAsset(t *testing.T) {
	engine := createTestEngine()

	t.Run("register valid asset", func(t *testing.T) {
		asset := &DataAsset{
			ID:       "asset-1",
			Path:     "/data/file1.txt",
			Size:     1024,
			MimeType: "text/plain",
			Owner:    "user1",
			Tier:     DataTierHot,
		}

		err := engine.RegisterAsset(asset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, exists := engine.dataAssets["asset-1"]; !exists {
			t.Error("asset not found")
		}
	})

	t.Run("register duplicate asset", func(t *testing.T) {
		asset := &DataAsset{
			ID:   "asset-1",
			Path: "/data/file1.txt",
		}

		err := engine.RegisterAsset(asset)
		if err != ErrAssetAlreadyExists {
			t.Errorf("expected ErrAssetAlreadyExists, got %v", err)
		}
	})

	t.Run("register asset without ID", func(t *testing.T) {
		asset := &DataAsset{
			Path: "/data/file2.txt",
		}

		err := engine.RegisterAsset(asset)
		if err != ErrAssetInvalid {
			t.Errorf("expected ErrAssetInvalid, got %v", err)
		}
	})
}

func TestEvaluateAsset(t *testing.T) {
	engine := createTestEngine()

	// 添加规则
	rule := &LifecycleRule{
		ID:   "rule-archive",
		Name: "Archive after 180 days",
		Condition: &RuleCondition{
			Type:     ConditionAge,
			Operator: OpGreaterThan,
			Value:    180,
		},
		Action:  ActionArchive,
		Enabled: true,
	}
	engine.AddRule(rule)

	t.Run("evaluate with rule match", func(t *testing.T) {
		asset := &DataAsset{
			ID:        "asset-old",
			Path:      "/data/old.txt",
			Size:      1024,
			CreatedAt: time.Now().AddDate(0, -7, 0), // 7个月前
			Tier:      DataTierHot,
		}
		engine.RegisterAsset(asset)

		engine.config.AIDecisionEnabled = false
		action, err := engine.EvaluateAsset("asset-old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action == nil {
			t.Fatal("expected action, got nil")
		}
		if action.ActionType != ActionTypeArchive {
			t.Errorf("expected ActionTypeArchive, got %v", action.ActionType)
		}
	})

	t.Run("evaluate with no rule match", func(t *testing.T) {
		asset := &DataAsset{
			ID:        "asset-new",
			Path:      "/data/new.txt",
			Size:      1024,
			CreatedAt: time.Now().AddDate(0, 0, -1), // 1天前
			Tier:      DataTierHot,
		}
		engine.RegisterAsset(asset)

		action, err := engine.EvaluateAsset("asset-new")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action != nil {
			t.Errorf("expected nil action, got %v", action)
		}
	})

	t.Run("evaluate non-existent asset", func(t *testing.T) {
		_, err := engine.EvaluateAsset("non-existent")
		if err != ErrAssetNotFound {
			t.Errorf("expected ErrAssetNotFound, got %v", err)
		}
	})
}

func TestAIEvaluate(t *testing.T) {
	engine := createTestEngine()
	engine.config.AIDecisionEnabled = true

	t.Run("hot asset stays hot", func(t *testing.T) {
		asset := &DataAsset{
			ID:          "asset-hot",
			Path:        "/data/hot.txt",
			Size:        1024,
			CreatedAt:   time.Now().AddDate(0, 0, -5), // 5天前
			AccessedAt:  time.Now().Add(-1 * time.Hour),
			AccessCount: 150,
			Tier:        DataTierHot,
		}
		engine.RegisterAsset(asset)

		action, err := engine.aiEvaluate(asset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// 热数据应该保持热
		if action != nil && action.ActionType == ActionTypeMigrate {
			t.Error("hot asset should not be migrated")
		}
	})

	t.Run("cold asset migrated to archive", func(t *testing.T) {
		asset := &DataAsset{
			ID:          "asset-cold",
			Path:        "/data/cold.txt",
			Size:        1024 * 1024,
			CreatedAt:   time.Now().AddDate(-2, 0, 0), // 2年前
			AccessedAt:  time.Now().AddDate(-1, 0, 0),  // 1年前
			AccessCount: 2,
			Tier:        DataTierCold,
		}
		engine.RegisterAsset(asset)

		action, err := engine.aiEvaluate(asset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action == nil {
			t.Fatal("expected action for cold asset")
		}
	})

	t.Run("immutable asset not migrated", func(t *testing.T) {
		asset := &DataAsset{
			ID:        "asset-immutable",
			Path:      "/data/immutable.txt",
			Size:      1024,
			CreatedAt: time.Now().AddDate(-3, 0, 0),
			Tier:      DataTierCold,
			Compliance: &ComplianceInfo{
				Immutable: true,
			},
		}
		engine.RegisterAsset(asset)

		action, err := engine.aiEvaluate(asset)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if action != nil {
			t.Error("immutable asset should not be migrated")
		}
	})
}

func TestExecuteAction(t *testing.T) {
	engine := createTestEngine()

	t.Run("execute action in dry run", func(t *testing.T) {
		engine.config.DryRun = true

		asset := &DataAsset{
			ID:   "asset-dry",
			Path: "/data/dry.txt",
			Size: 1024,
			Tier: DataTierHot,
		}
		engine.RegisterAsset(asset)

		action := &LifecycleAction{
			ID:         "action-dry",
			AssetID:    "asset-dry",
			ActionType: ActionTypeArchive,
			Status:     ActionPending,
		}

		err := engine.ExecuteAction(action)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 状态不应该改变（干运行）
		if action.Status != ActionPending {
			t.Errorf("expected ActionPending, got %v", action.Status)
		}
	})

	t.Run("execute action normally", func(t *testing.T) {
		engine.config.DryRun = false

		asset := &DataAsset{
			ID:   "asset-execute",
			Path: "/data/execute.txt",
			Size: 1024,
			Tier: DataTierHot,
		}
		engine.RegisterAsset(asset)

		action := &LifecycleAction{
			ID:         "action-execute",
			AssetID:    "asset-execute",
			ActionType: ActionTypeArchive,
			Status:     ActionPending,
			Savings:    500,
		}

		err := engine.ExecuteAction(action)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if action.Status != ActionCompleted {
			t.Errorf("expected ActionCompleted, got %v", action.Status)
		}

		// 检查资产层级已更新
		if asset.Tier != DataTierArchive {
			t.Errorf("expected DataTierArchive, got %v", asset.Tier)
		}
	})

	t.Run("concurrent limit", func(t *testing.T) {
		engine.config.MaxConcurrent = 1
		engine.config.DryRun = false

		// 先添加一个运行中的操作
		engine.actions = append(engine.actions, &LifecycleAction{
			Status: ActionRunning,
		})

		action := &LifecycleAction{
			ID:         "action-limit",
			AssetID:    "asset-execute",
			ActionType: ActionTypeArchive,
			Status:     ActionPending,
		}

		err := engine.ExecuteAction(action)
		if err != ErrConcurrentLimit {
			t.Errorf("expected ErrConcurrentLimit, got %v", err)
		}
	})
}

func TestScanAndProcess(t *testing.T) {
	engine := createTestEngine()
	engine.config.AutoExecute = true
	engine.config.DryRun = true

	// 添加规则
	rule := &LifecycleRule{
		ID:   "rule-archive",
		Name: "Archive old files",
		Condition: &RuleCondition{
			Type:     ConditionAge,
			Operator: OpGreaterThan,
			Value:    180,
		},
		Action:  ActionArchive,
		Enabled: true,
	}
	engine.AddRule(rule)

	// 添加资产
	assets := []*DataAsset{
		{
			ID:        "asset-old-1",
			Path:      "/data/old1.txt",
			Size:      1024,
			CreatedAt: time.Now().AddDate(0, -8, 0),
			Tier:      DataTierHot,
		},
		{
			ID:        "asset-old-2",
			Path:      "/data/old2.txt",
			Size:      2048,
			CreatedAt: time.Now().AddDate(0, -10, 0),
			Tier:      DataTierWarm,
		},
		{
			ID:        "asset-new",
			Path:      "/data/new.txt",
			Size:      512,
			CreatedAt: time.Now().AddDate(0, 0, -1),
			Tier:      DataTierHot,
		},
	}

	for _, asset := range assets {
		engine.RegisterAsset(asset)
	}

	// 执行扫描
	err := engine.ScanAndProcess()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 检查指标
	metrics := engine.GetMetrics()
	if metrics.TotalAssets != 3 {
		t.Errorf("expected 3 assets, got %d", metrics.TotalAssets)
	}
}

func TestComplianceReport(t *testing.T) {
	engine := createTestEngine()
	engine.config.ComplianceMode = true

	// 添加合规资产
	engine.RegisterAsset(&DataAsset{
		ID:   "asset-compliant",
		Path: "/data/compliant.txt",
		Size: 1024,
		Tier: DataTierHot,
		Compliance: &ComplianceInfo{
			Encrypted:    true,
			Classification: ClassificationConfidential,
		},
	})

	// 添加不合规资产
	engine.RegisterAsset(&DataAsset{
		ID:   "asset-non-compliant",
		Path: "/data/non-compliant.txt",
		Size: 1024,
		Tier: DataTierHot,
		Compliance: &ComplianceInfo{
			Encrypted:    false,
			Classification: ClassificationConfidential,
		},
	})

	report, err := engine.GetComplianceReport()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalAssets != 2 {
		t.Errorf("expected 2 assets, got %d", report.TotalAssets)
	}

	if len(report.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(report.Violations))
	}

	if report.ComplianceRate != 50.0 {
		t.Errorf("expected 50%% compliance rate, got %.2f%%", report.ComplianceRate)
	}
}

func TestGetMetrics(t *testing.T) {
	engine := createTestEngine()

	// 添加不同层级的资产
	engine.RegisterAsset(&DataAsset{ID: "hot-1", Path: "/hot1", Tier: DataTierHot})
	engine.RegisterAsset(&DataAsset{ID: "hot-2", Path: "/hot2", Tier: DataTierHot})
	engine.RegisterAsset(&DataAsset{ID: "warm-1", Path: "/warm1", Tier: DataTierWarm})
	engine.RegisterAsset(&DataAsset{ID: "cold-1", Path: "/cold1", Tier: DataTierCold})
	engine.RegisterAsset(&DataAsset{ID: "archive-1", Path: "/archive1", Tier: DataTierArchive})

	metrics := engine.GetMetrics()

	if metrics.TotalAssets != 5 {
		t.Errorf("expected 5 total assets, got %d", metrics.TotalAssets)
	}
	if metrics.HotData != 2 {
		t.Errorf("expected 2 hot data, got %d", metrics.HotData)
	}
	if metrics.WarmData != 1 {
		t.Errorf("expected 1 warm data, got %d", metrics.WarmData)
	}
	if metrics.ColdData != 1 {
		t.Errorf("expected 1 cold data, got %d", metrics.ColdData)
	}
	if metrics.ArchivedData != 1 {
		t.Errorf("expected 1 archived data, got %d", metrics.ArchivedData)
	}
}

func TestEvaluateCondition(t *testing.T) {
	engine := createTestEngine()

	asset := &DataAsset{
		ID:          "asset-test",
		Path:        "/data/test.txt",
		Size:        1024 * 1024, // 1MB
		CreatedAt:   time.Now().AddDate(0, -6, 0), // 6个月前
		AccessedAt:  time.Now().AddDate(0, 0, -30), // 30天前
		AccessCount: 50,
		Owner:       "user1",
		Tags:        []string{"important", "project-a"},
	}

	t.Run("age condition", func(t *testing.T) {
		condition := &RuleCondition{
			Type:     ConditionAge,
			Operator: OpGreaterThan,
			Value:    90,
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected age > 90 days")
		}
	})

	t.Run("size condition", func(t *testing.T) {
		condition := &RuleCondition{
			Type:     ConditionSize,
			Operator: OpGreaterThan,
			Value:    1024,
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected size > 1024 bytes")
		}
	})

	t.Run("access condition", func(t *testing.T) {
		condition := &RuleCondition{
			Type:     ConditionAccess,
			Operator: OpLessThan,
			Value:    60,
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected last access < 60 days")
		}
	})

	t.Run("tag condition", func(t *testing.T) {
		condition := &RuleCondition{
			Type:     ConditionTag,
			Operator: OpEquals,
			Value:    "important",
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected tag match")
		}
	})

	t.Run("owner condition", func(t *testing.T) {
		condition := &RuleCondition{
			Type:     ConditionOwner,
			Operator: OpEquals,
			Value:    "user1",
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected owner match")
		}
	})

	t.Run("AND condition", func(t *testing.T) {
		condition := &RuleCondition{
			And: []*RuleCondition{
				{Type: ConditionAge, Operator: OpGreaterThan, Value: 30},
				{Type: ConditionSize, Operator: OpGreaterThan, Value: 100},
			},
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected AND condition to be true")
		}
	})

	t.Run("OR condition", func(t *testing.T) {
		condition := &RuleCondition{
			Or: []*RuleCondition{
				{Type: ConditionAge, Operator: OpGreaterThan, Value: 365}, // 不满足
				{Type: ConditionTag, Operator: OpEquals, Value: "important"}, // 满足
			},
		}
		if !engine.evaluateCondition(condition, asset) {
			t.Error("expected OR condition to be true")
		}
	})
}

func TestTierDecision(t *testing.T) {
	engine := createTestEngine()

	testCases := []struct {
		name     string
		features map[string]float64
		expected DataTier
	}{
		{
			name: "hot data",
			features: map[string]float64{
				"age_days":         5,
				"access_frequency": 150,
				"last_access_days": 1,
			},
			expected: DataTierHot,
		},
		{
			name: "warm data",
			features: map[string]float64{
				"age_days":         30,
				"access_frequency": 60,
				"last_access_days": 15,
			},
			expected: DataTierWarm,
		},
		{
			name: "cold data",
			features: map[string]float64{
				"age_days":         100,
				"access_frequency": 5,
				"last_access_days": 60,
			},
			expected: DataTierCold,
		},
		{
			name: "archive data",
			features: map[string]float64{
				"age_days":         365,
				"access_frequency": 0,
				"last_access_days": 300,
			},
			expected: DataTierArchive,
		},
	}

	model := engine.aiEngine.models["tier_decision"]

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := engine.calculateTierDecision(tc.features, model)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestDataTierString(t *testing.T) {
	testCases := []struct {
		tier     DataTier
		expected string
	}{
		{DataTierHot, "hot"},
		{DataTierWarm, "warm"},
		{DataTierCold, "cold"},
		{DataTierArchive, "archive"},
		{DataTierDelete, "delete"},
		{DataTier(99), "unknown"},
	}

	for _, tc := range testCases {
		if tc.tier.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.tier.String())
		}
	}
}

func TestActionTypeString(t *testing.T) {
	testCases := []struct {
		action   ActionType
		expected string
	}{
		{ActionTypeMigrate, "migrate"},
		{ActionTypeArchive, "archive"},
		{ActionTypeDelete, "delete"},
		{ActionTypeCompress, "compress"},
		{ActionTypeEncrypt, "encrypt"},
		{ActionType(99), "unknown"},
	}

	for _, tc := range testCases {
		if tc.action.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.action.String())
		}
	}
}

func TestActionStatusString(t *testing.T) {
	testCases := []struct {
		status   ActionStatus
		expected string
	}{
		{ActionPending, "pending"},
		{ActionRunning, "running"},
		{ActionCompleted, "completed"},
		{ActionFailed, "failed"},
		{ActionCancelled, "cancelled"},
		{ActionStatus(99), "unknown"},
	}

	for _, tc := range testCases {
		if tc.status.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.status.String())
		}
	}
}

func TestRuleActionString(t *testing.T) {
	testCases := []struct {
		action   RuleAction
		expected string
	}{
		{ActionArchive, "archive"},
		{ActionMigrate, "migrate"},
		{ActionCompress, "compress"},
		{ActionEncrypt, "encrypt"},
		{ActionDelete, "delete"},
		{ActionNotify, "notify"},
		{ActionTag, "tag"},
		{RuleAction(99), "unknown"},
	}

	for _, tc := range testCases {
		if tc.action.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.action.String())
		}
	}
}

func TestDataClassificationString(t *testing.T) {
	testCases := []struct {
		class    DataClassification
		expected string
	}{
		{ClassificationPublic, "public"},
		{ClassificationInternal, "internal"},
		{ClassificationConfidential, "confidential"},
		{ClassificationRestricted, "restricted"},
		{ClassificationTopSecret, "top_secret"},
		{DataClassification(99), "unknown"},
	}

	for _, tc := range testCases {
		if tc.class.String() != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.class.String())
		}
	}
}

// createTestEngine 创建测试引擎
func createTestEngine() *LifecycleEngine {
	config := &LifecycleConfig{
		ScanInterval:      1 * time.Hour,
		AutoExecute:       false,
		DryRun:            false,
		MaxConcurrent:     10,
		ComplianceMode:    false,
		AIDecisionEnabled: false,
	}

	engine, _ := NewLifecycleEngine(config, slog.Default())
	return engine
}

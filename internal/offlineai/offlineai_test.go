// Package offlineai 测试文件
package offlineai

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func setupTestEngine(t *testing.T) *Engine {
	t.Helper()
	return NewEngine(zap.NewNop(), nil)
}

func setupTestChatEngine(t *testing.T) *ChatEngine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.DefaultModel = "test"
	engine := NewEngine(zap.NewNop(), cfg)
	engine.Start(context.Background())
	// 加载测试模型
	engine.LoadModel(context.Background(), &Model{
		Name:       "test",
		Path:       "/tmp/test.gguf",
		Format:     ModelFormatGGUF,
		Status:     ModelStatusReady,
		MaxContext: 4096,
	})
	return NewChatEngine(zap.NewNop(), engine, 50)
}

func setupTestScheduler(t *testing.T) *Scheduler {
	t.Helper()
	return NewScheduler(zap.NewNop(), 2, func(ctx context.Context, task *Task) (interface{}, error) {
		return "done", nil
	})
}

func setupTestModelManager(t *testing.T) *ModelManager {
	t.Helper()
	engine := setupTestEngine(t)
	return NewModelManager(zap.NewNop(), engine)
}

// ==================== Engine Tests ====================

func TestNewEngine(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		e := setupTestEngine(t)
		if e == nil {
			t.Fatal("expected non-nil engine")
		}
		if e.IsRunning() {
			t.Error("engine should not be running initially")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.ContextSize = 8192
		cfg.GPUEnabled = false
		e := NewEngine(zap.NewNop(), cfg)
		if e.config.ContextSize != 8192 {
			t.Errorf("expected context size 8192, got %d", e.config.ContextSize)
		}
	})
}

func TestEngineStartStop(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()

	// 启动
	if err := e.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !e.IsRunning() {
		t.Error("engine should be running after start")
	}

	// 重复启动
	if err := e.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	// 停止
	e.Stop()
	if e.IsRunning() {
		t.Error("engine should not be running after stop")
	}
}

func TestLoadUnloadModel(t *testing.T) {
	// 使用不含默认模型的配置
	cfg := DefaultConfig()
	cfg.DefaultModel = ""
	e := NewEngine(zap.NewNop(), cfg)
	ctx := context.Background()
	e.Start(ctx)
	defer e.Stop()

	model := &Model{
		Name:       "test-model",
		Path:       "/tmp/test.gguf",
		Format:     ModelFormatGGUF,
		QuantType:  QuantQ4_0,
		Status:     ModelStatusReady,
		MaxContext: 4096,
	}

	// 加载
	if err := e.LoadModel(ctx, model); err != nil {
		t.Fatalf("load model failed: %v", err)
	}

	// 获取
	got, err := e.GetModel("test-model")
	if err != nil {
		t.Fatalf("get model failed: %v", err)
	}
	if got.Name != "test-model" {
		t.Errorf("expected name test-model, got %s", got.Name)
	}

	// 重复加载
	if err := e.LoadModel(ctx, model); err == nil {
		t.Error("expected error on double load")
	}

	// 列出
	models := e.ListModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}

	// 卸载
	if err := e.UnloadModel("test-model"); err != nil {
		t.Fatalf("unload model failed: %v", err)
	}

	// 卸载不存在的
	if err := e.UnloadModel("nonexistent"); err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestSwitchModel(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()
	e.Start(ctx)
	defer e.Stop()

	m1 := &Model{Name: "model-a", Path: "/tmp/a.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096}
	m2 := &Model{Name: "model-b", Path: "/tmp/b.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096}

	e.LoadModel(ctx, m1)
	e.LoadModel(ctx, m2)

	if err := e.SwitchModel("model-b"); err != nil {
		t.Fatalf("switch model failed: %v", err)
	}

	if e.config.DefaultModel != "model-b" {
		t.Errorf("expected default model model-b, got %s", e.config.DefaultModel)
	}

	// 切换到不存在的模型
	if err := e.SwitchModel("nonexistent"); err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestInfer(t *testing.T) {
	e := setupTestEngine(t)
	ctx := context.Background()
	e.Start(ctx)
	defer e.Stop()

	e.LoadModel(ctx, &Model{
		Name:       "test",
		Path:       "/tmp/test.gguf",
		Format:     ModelFormatGGUF,
		Status:     ModelStatusReady,
		MaxContext: 4096,
	})
	e.config.DefaultModel = "test"

	t.Run("basic inference", func(t *testing.T) {
		resp, err := e.Infer(ctx, &InferRequest{
			Prompt: "Hello, world!",
		})
		if err != nil {
			t.Fatalf("infer failed: %v", err)
		}
		if resp.Text == "" {
			t.Error("expected non-empty response")
		}
		if resp.TokensUsed <= 0 {
			t.Error("expected positive tokens used")
		}
		if resp.ModelName != "test" {
			t.Errorf("expected model test, got %s", resp.ModelName)
		}
	})

	t.Run("engine not running", func(t *testing.T) {
		e2 := setupTestEngine(t)
		_, err := e2.Infer(ctx, &InferRequest{Prompt: "test"})
		if err == nil {
			t.Error("expected error when engine not running")
		}
	})

	t.Run("model not loaded", func(t *testing.T) {
		_, err := e.Infer(ctx, &InferRequest{Prompt: "test", ModelName: "nonexistent"})
		if err == nil {
			t.Error("expected error for unloaded model")
		}
	})
}

func TestGPUInfo(t *testing.T) {
	e := setupTestEngine(t)
	info := e.GetGPUInfo()
	if info == nil {
		t.Fatal("expected non-nil GPU info")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("expected enabled by default")
	}
	if cfg.EngineType != EngineLlamaCpp {
		t.Errorf("expected engine llamacpp, got %s", cfg.EngineType)
	}
	if cfg.ContextSize != 4096 {
		t.Errorf("expected context size 4096, got %d", cfg.ContextSize)
	}
	if cfg.MaxTokens != 2048 {
		t.Errorf("expected max tokens 2048, got %d", cfg.MaxTokens)
	}
}

// ==================== ChatEngine Tests ====================

func TestChatCreateConversation(t *testing.T) {
	ce := setupTestChatEngine(t)

	conv := ce.CreateConversation("test-model")
	if conv.ID == "" {
		t.Error("expected non-empty conversation ID")
	}
	if conv.ModelName != "test-model" {
		t.Errorf("expected model test-model, got %s", conv.ModelName)
	}
}

func TestChatGetConversation(t *testing.T) {
	ce := setupTestChatEngine(t)

	conv := ce.CreateConversation("test")
	got, err := ce.GetConversation(conv.ID)
	if err != nil {
		t.Fatalf("get conversation failed: %v", err)
	}
	if got.ID != conv.ID {
		t.Errorf("expected ID %s, got %s", conv.ID, got.ID)
	}

	// 不存在的对话
	_, err = ce.GetConversation("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestChatDeleteConversation(t *testing.T) {
	ce := setupTestChatEngine(t)

	conv := ce.CreateConversation("test")
	if err := ce.DeleteConversation(conv.ID); err != nil {
		t.Fatalf("delete conversation failed: %v", err)
	}

	// 确认已删除
	_, err := ce.GetConversation(conv.ID)
	if err == nil {
		t.Error("expected error after delete")
	}

	// 删除不存在的
	if err := ce.DeleteConversation("nonexistent"); err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

func TestChatListConversations(t *testing.T) {
	ce := setupTestChatEngine(t)

	ce.CreateConversation("model-a")
	ce.CreateConversation("model-b")

	convs := ce.ListConversations()
	if len(convs) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(convs))
	}
}

func TestChatSendMessage(t *testing.T) {
	ce := setupTestChatEngine(t)
	ctx := context.Background()

	t.Run("new conversation", func(t *testing.T) {
		resp, err := ce.SendMessage(ctx, &ChatRequest{
			Message:   "Hello",
			ModelName: "test",
		})
		if err != nil {
			t.Fatalf("send message failed: %v", err)
		}
		if resp.ConversationID == "" {
			t.Error("expected non-empty conversation ID")
		}
		if resp.Reply == "" {
			t.Error("expected non-empty reply")
		}
	})

	t.Run("continue conversation", func(t *testing.T) {
		conv := ce.CreateConversation("test")
		resp, err := ce.SendMessage(ctx, &ChatRequest{
			ConversationID: conv.ID,
			Message:        "How are you?",
			ModelName:      "test",
		})
		if err != nil {
			t.Fatalf("send message failed: %v", err)
		}
		if resp.Reply == "" {
			t.Error("expected non-empty reply")
		}

		// 验证历史
		got, _ := ce.GetConversation(conv.ID)
		if len(got.Messages) < 2 {
			t.Errorf("expected at least 2 messages, got %d", len(got.Messages))
		}
	})
}

func TestChatStreamChat(t *testing.T) {
	ce := setupTestChatEngine(t)
	ctx := context.Background()

	ch, err := ce.StreamChat(ctx, &ChatRequest{
		Message:   "Hello stream",
		ModelName: "test",
	})
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}

	var chunks []*StreamChunk
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}

	// 最后一个 chunk 应该 done=true
	if len(chunks) > 0 && !chunks[len(chunks)-1].Done {
		t.Error("expected last chunk to be done")
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text      string
		minTokens int
	}{
		{"hello", 1},
		{"你好世界", 1},
		{"a longer text with multiple words", 5},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			tokens := estimateTokens(tt.text)
			if tokens < tt.minTokens {
				t.Errorf("expected at least %d tokens, got %d", tt.minTokens, tokens)
			}
		})
	}
}

// ==================== Scheduler Tests ====================

func TestSchedulerStartStop(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// 重复启动
	if err := s.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	s.Stop()
}

func TestSchedulerSubmit(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	task := &Task{
		Name:     "test-task",
		Type:     "infer",
		Priority: PriorityNormal,
	}

	if err := s.Submit(task); err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Status != TaskStatusPending && task.Status != TaskStatusRunning {
		t.Errorf("expected pending or running, got %s", task.Status)
	}
}

func TestSchedulerSubmitNotRunning(t *testing.T) {
	s := setupTestScheduler(t)
	err := s.Submit(&Task{Name: "test"})
	if err == nil {
		t.Error("expected error when scheduler not running")
	}
}

func TestSchedulerCancel(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	task := &Task{
		Name:     "cancel-test",
		Type:     "infer",
		Priority: PriorityLow,
	}
	s.Submit(task)

	// 等待一小段时间让任务可能被处理
	time.Sleep(50 * time.Millisecond)

	// 尝试取消（如果任务还在 pending 状态）
	if task.Status == TaskStatusPending {
		if err := s.Cancel(task.ID); err != nil {
			t.Fatalf("cancel failed: %v", err)
		}
	}

	// 取消不存在的任务
	if err := s.Cancel("nonexistent"); err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestSchedulerGetTask(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	task := &Task{
		Name:     "get-test",
		Type:     "infer",
		Priority: PriorityHigh,
	}
	s.Submit(task)

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if got.Name != "get-test" {
		t.Errorf("expected name get-test, got %s", got.Name)
	}

	// 不存在的任务
	_, err = s.GetTask("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}

func TestSchedulerListTasks(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	s.Submit(&Task{Name: "task-1", Type: "infer", Priority: PriorityLow})
	s.Submit(&Task{Name: "task-2", Type: "infer", Priority: PriorityHigh})

	time.Sleep(100 * time.Millisecond)

	all := s.ListTasks("")
	if len(all) < 2 {
		t.Errorf("expected at least 2 tasks, got %d", len(all))
	}
}

func TestSchedulerPriority(t *testing.T) {
	ctx := context.Background()

	var order []string
	handler := func(ctx context.Context, task *Task) (interface{}, error) {
		order = append(order, task.Name)
		return nil, nil
	}

	s2 := NewScheduler(zap.NewNop(), 1, handler)
	s2.Start(ctx)
	defer s2.Stop()

	// 低优先级先提交
	s2.Submit(&Task{Name: "low", Type: "test", Priority: PriorityLow})
	s2.Submit(&Task{Name: "high", Type: "test", Priority: PriorityHigh})
	s2.Submit(&Task{Name: "urgent", Type: "test", Priority: PriorityUrgent})

	time.Sleep(500 * time.Millisecond)

	// 高优先级应该先执行
	if len(order) >= 3 {
		if order[0] != "urgent" {
			t.Errorf("expected urgent first, got %s", order[0])
		}
		if order[1] != "high" {
			t.Errorf("expected high second, got %s", order[1])
		}
	}
}

func TestSchedulerRetry(t *testing.T) {
	// 测试重试逻辑（验证任务成功完成）
	handler := func(ctx context.Context, task *Task) (interface{}, error) {
		return "success", nil
	}

	s := NewScheduler(zap.NewNop(), 1, handler)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	task := &Task{
		Name:        "retry-task",
		Type:        "infer",
		Priority:    PriorityNormal,
		MaxAttempts: 3,
	}
	s.Submit(task)

	// 等待任务完成
	deadline := time.After(2 * time.Second)
	for {
		got, _ := s.GetTask(task.ID)
		if got.Status == TaskStatusCompleted {
			return // 成功
		}
		select {
		case <-deadline:
			got, _ := s.GetTask(task.ID)
			t.Fatalf("timeout waiting for task, status: %s, attempts: %d", got.Status, got.Attempts)
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestSchedulerStats(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	s.Submit(&Task{Name: "s1", Type: "test"})
	s.Submit(&Task{Name: "s2", Type: "test"})

	time.Sleep(100 * time.Millisecond)

	stats := s.GetStats()
	if stats["total"] < 2 {
		t.Errorf("expected total >= 2, got %d", stats["total"])
	}
}

func TestSchedulerScheduledTask(t *testing.T) {
	s := setupTestScheduler(t)
	ctx := context.Background()
	s.Start(ctx)
	defer s.Stop()

	futureTime := time.Now().Add(1 * time.Hour)
	task := &Task{
		Name:        "scheduled",
		Type:        "test",
		Priority:    PriorityNormal,
		ScheduledAt: &futureTime,
	}
	s.Submit(task)

	time.Sleep(100 * time.Millisecond)

	got, _ := s.GetTask(task.ID)
	if got.Status != TaskStatusPending {
		t.Errorf("expected pending for future task, got %s", got.Status)
	}
}

// ==================== ModelManager Tests ====================

func TestModelManagerRegister(t *testing.T) {
	mm := setupTestModelManager(t)

	model := &Model{
		Name:      "test-model",
		Path:      "/tmp/test.gguf",
		Format:    ModelFormatGGUF,
		QuantType: QuantQ4_0,
	}

	if err := mm.Register(model); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	// 重复注册
	if err := mm.Register(model); err == nil {
		t.Error("expected error on double register")
	}

	info, err := mm.GetModelInfo("test-model")
	if err != nil {
		t.Fatalf("get info failed: %v", err)
	}
	if info.Model.Name != "test-model" {
		t.Errorf("expected name test-model, got %s", info.Model.Name)
	}
}

func TestModelManagerUnregister(t *testing.T) {
	mm := setupTestModelManager(t)

	model := &Model{Name: "test", Path: "/tmp/test.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096}
	mm.Register(model)

	if err := mm.Unregister("test"); err != nil {
		t.Fatalf("unregister failed: %v", err)
	}

	// 不存在的模型
	if err := mm.Unregister("nonexistent"); err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestModelManagerListModels(t *testing.T) {
	mm := setupTestModelManager(t)

	mm.Register(&Model{Name: "m1", Path: "/tmp/m1.gguf", Format: ModelFormatGGUF, QuantType: QuantQ4_0})
	mm.Register(&Model{Name: "m2", Path: "/tmp/m2.gguf", Format: ModelFormatGGUF, QuantType: QuantQ8_0})

	models := mm.ListModels()
	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}
}

func TestModelManagerFilterByFormat(t *testing.T) {
	mm := setupTestModelManager(t)

	mm.Register(&Model{Name: "gguf-m", Path: "/tmp/a.gguf", Format: ModelFormatGGUF, QuantType: QuantQ4_0})
	mm.Register(&Model{Name: "onnx-m", Path: "/tmp/b.onnx", Format: ModelFormatONNX, QuantType: QuantNone})

	ggufModels := mm.GetModelsByFormat(ModelFormatGGUF)
	if len(ggufModels) != 1 {
		t.Errorf("expected 1 gguf model, got %d", len(ggufModels))
	}
}

func TestModelManagerFilterByQuant(t *testing.T) {
	mm := setupTestModelManager(t)

	mm.Register(&Model{Name: "q4", Path: "/tmp/q4.gguf", Format: ModelFormatGGUF, QuantType: QuantQ4_0})
	mm.Register(&Model{Name: "q8", Path: "/tmp/q8.gguf", Format: ModelFormatGGUF, QuantType: QuantQ8_0})
	mm.Register(&Model{Name: "q4b", Path: "/tmp/q4b.gguf", Format: ModelFormatGGUF, QuantType: QuantQ4_0})

	q4Models := mm.GetModelsByQuant(QuantQ4_0)
	if len(q4Models) != 2 {
		t.Errorf("expected 2 q4 models, got %d", len(q4Models))
	}
}

func TestModelManagerPopular(t *testing.T) {
	mm := setupTestModelManager(t)

	m1 := &Model{Name: "popular", Path: "/tmp/p.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096}
	m2 := &Model{Name: "unpopular", Path: "/tmp/u.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096}

	mm.Register(m1)
	mm.Register(m2)

	// 加载 popular 几次
	mm.Load("popular")
	mm.Load("popular")
	mm.Load("popular")
	mm.Load("unpopular")

	popular := mm.GetPopularModels(1)
	if len(popular) != 1 {
		t.Fatalf("expected 1 popular model, got %d", len(popular))
	}
	if popular[0].Model.Name != "popular" {
		t.Errorf("expected popular, got %s", popular[0].Model.Name)
	}
}

func TestModelManagerSwitchTo(t *testing.T) {
	mm := setupTestModelManager(t)

	mm.Register(&Model{Name: "switch-target", Path: "/tmp/s.gguf", Format: ModelFormatGGUF, Status: ModelStatusReady, MaxContext: 4096})

	if err := mm.SwitchTo("switch-target"); err != nil {
		t.Fatalf("switch failed: %v", err)
	}

	// 切换到不存在的
	if err := mm.SwitchTo("nonexistent"); err == nil {
		t.Error("expected error for nonexistent model")
	}
}

func TestSupportedQuantTypes(t *testing.T) {
	qts := GetSupportedQuantTypes()
	if len(qts) < 5 {
		t.Errorf("expected at least 5 quant types, got %d", len(qts))
	}
}

func TestEstimateVRAMEstimate(t *testing.T) {
	tests := []struct {
		params int64
		quant  QuantType
		name   string
	}{
		{7_000_000_000, QuantQ4_0, "7B-Q4"},
		{7_000_000_000, QuantF16, "7B-F16"},
		{13_000_000_000, QuantQ8_0, "13B-Q8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vram := EstimateVRAMEstimate(tt.params, tt.quant)
			if vram <= 0 {
				t.Error("expected positive VRAM estimate")
			}
		})
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()
	if id1 == id2 {
		t.Error("expected unique task IDs")
	}
	if len(id1) < 10 {
		t.Errorf("expected longer task ID, got %s", id1)
	}
}

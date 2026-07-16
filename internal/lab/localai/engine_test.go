package localai

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine(5)
	if engine == nil {
		t.Fatal("引擎创建失败")
	}
	if engine.maxModels != 5 {
		t.Errorf("期望最大模型数 5，实际 %d", engine.maxModels)
	}
}

func TestRegisterModel(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:           "test-model-1",
		Name:         "测试模型",
		Type:         ModelTypeLLM,
		Backend:      BackendCPU,
		Quantization: QuantINT8,
	}

	if err := engine.RegisterModel(model); err != nil {
		t.Fatalf("注册模型失败: %v", err)
	}

	if len(engine.models) != 1 {
		t.Errorf("期望模型数 1，实际 %d", len(engine.models))
	}

	// 重复注册应失败
	if err := engine.RegisterModel(model); err != ErrModelAlreadyExists {
		t.Errorf("期望 ErrModelAlreadyExists，实际 %v", err)
	}
}

func TestLoadModel(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:   "test-model-1",
		Name: "测试模型",
		Type: ModelTypeLLM,
	}

	engine.RegisterModel(model)

	if err := engine.LoadModel("test-model-1"); err != nil {
		t.Fatalf("加载模型失败: %v", err)
	}

	loaded, _ := engine.GetModel("test-model-1")
	if loaded.Status != StatusReady {
		t.Errorf("期望状态 ready，实际 %s", loaded.Status)
	}
	if loaded.LoadedAt == nil {
		t.Error("LoadedAt 不应为 nil")
	}
}

func TestUnloadModel(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:   "test-model-1",
		Name: "测试模型",
		Type: ModelTypeLLM,
	}

	engine.RegisterModel(model)
	engine.LoadModel("test-model-1")

	if err := engine.UnloadModel("test-model-1"); err != nil {
		t.Fatalf("卸载模型失败: %v", err)
	}

	unloaded, _ := engine.GetModel("test-model-1")
	if unloaded.Status != StatusAvailable {
		t.Errorf("期望状态 available，实际 %s", unloaded.Status)
	}
}

func TestInference(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:            "test-model-1",
		Name:          "测试模型",
		Type:          ModelTypeLLM,
		Backend:       BackendCPU,
		ContextLength: 4096,
	}

	engine.RegisterModel(model)
	engine.LoadModel("test-model-1")

	req := &InferenceRequest{
		ModelID:   "test-model-1",
		Prompt:    "测试提示词",
		MaxTokens: 100,
	}

	resp, err := engine.Inference(req)
	if err != nil {
		t.Fatalf("推理失败: %v", err)
	}

	if resp.ModelID != "test-model-1" {
		t.Errorf("期望模型ID test-model-1，实际 %s", resp.ModelID)
	}
	if resp.TotalTokens <= 0 {
		t.Error("总token数应大于0")
	}
	if resp.Duration <= 0 {
		t.Error("推理耗时应大于0")
	}
}

func TestInferenceNotLoaded(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:   "test-model-1",
		Name: "测试模型",
		Type: ModelTypeLLM,
	}

	engine.RegisterModel(model)

	req := &InferenceRequest{
		ModelID: "test-model-1",
		Prompt:  "测试",
	}

	_, err := engine.Inference(req)
	if err != ErrModelNotLoaded {
		t.Errorf("期望 ErrModelNotLoaded，实际 %v", err)
	}
}

func TestEmbedding(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{
		ID:   "embed-model-1",
		Name: "嵌入模型",
		Type: ModelTypeEmbedding,
	}

	engine.RegisterModel(model)
	engine.LoadModel("embed-model-1")

	req := &EmbeddingRequest{
		ModelID: "embed-model-1",
		Texts:   []string{"测试文本1", "测试文本2"},
	}

	resp, err := engine.Embedding(req)
	if err != nil {
		t.Fatalf("嵌入计算失败: %v", err)
	}

	if len(resp.Embeddings) != 2 {
		t.Errorf("期望2个嵌入向量，实际 %d", len(resp.Embeddings))
	}
	if resp.Dimensions != 768 {
		t.Errorf("期望维度768，实际 %d", resp.Dimensions)
	}
}

func TestListModels(t *testing.T) {
	engine := NewEngine(10)

	engine.RegisterModel(&Model{ID: "m1", Name: "模型1", Type: ModelTypeLLM})
	engine.RegisterModel(&Model{ID: "m2", Name: "模型2", Type: ModelTypeVision})
	engine.RegisterModel(&Model{ID: "m3", Name: "模型3", Type: ModelTypeLLM})

	// 列出所有
	all := engine.ListModels(nil)
	if len(all) != 3 {
		t.Errorf("期望3个模型，实际 %d", len(all))
	}

	// 按类型筛选
	llmType := ModelTypeLLM
	llms := engine.ListModels(&llmType)
	if len(llms) != 2 {
		t.Errorf("期望2个LLM模型，实际 %d", len(llms))
	}
}

func TestGetResourceInfo(t *testing.T) {
	engine := NewEngine(10)
	info := engine.GetResourceInfo()

	if info.GPUTotalMB <= 0 {
		t.Error("GPU总量应大于0")
	}
	if info.RAMTotalMB <= 0 {
		t.Error("RAM总量应大于0")
	}
}

func TestGetStats(t *testing.T) {
	engine := NewEngine(10)
	stats := engine.GetStats()

	if stats.UptimeSeconds < 0 {
		t.Error("运行时间不应为负")
	}
}

func TestGetInferenceHistory(t *testing.T) {
	engine := NewEngine(10)

	model := &Model{ID: "m1", Name: "模型1", Type: ModelTypeLLM}
	engine.RegisterModel(model)
	engine.LoadModel("m1")

	// 执行几次推理
	for i := 0; i < 3; i++ {
		engine.Inference(&InferenceRequest{
			ModelID: "m1",
			Prompt:  "测试",
		})
	}

	history := engine.GetInferenceHistory("m1", 0)
	if len(history) != 3 {
		t.Errorf("期望3条记录，实际 %d", len(history))
	}

	limited := engine.GetInferenceHistory("m1", 2)
	if len(limited) != 2 {
		t.Errorf("期望2条记录，实际 %d", len(limited))
	}
}

func TestGPUDevice(t *testing.T) {
	engine := NewEngine(10)

	device := GPUDevice{
		Index:       0,
		Name:        "NVIDIA RTX 4090",
		MemoryMB:    24576,
		Temperature: 45,
		Utilization: 10.5,
	}

	engine.SetGPUDevice(device)
	devices := engine.GetGPUDevices()

	if len(devices) != 1 {
		t.Errorf("期望1个GPU设备，实际 %d", len(devices))
	}
	if devices[0].Name != "NVIDIA RTX 4090" {
		t.Errorf("期望设备名 NVIDIA RTX 4090，实际 %s", devices[0].Name)
	}
}

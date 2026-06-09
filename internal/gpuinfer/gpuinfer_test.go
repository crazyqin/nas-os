package gpuinfer

import (
	"testing"
	"time"
)

func TestNewGPUInferService(t *testing.T) {
	config := InferConfig{
		DefaultModel:   "llama-7b",
		MaxConcurrent:  4,
		RequestTimeout: 30 * time.Second,
		GPUMemoryLimit: 0.9,
		EnableCUDA:     true,
		EnableROCm:     false,
		EnableVulkan:   false,
		ModelCacheDir:  "/models",
		AutoUnloadMins: 30,
	}
	svc := NewGPUInferService(config)
	if svc == nil {
		t.Fatal("NewGPUInferService returned nil")
	}
	status := svc.GetServiceStatus()
	if status["gpus"] != 0 {
		t.Errorf("expected 0 gpus before Start, got %v", status["gpus"])
	}
	if status["models"] != 0 {
		t.Errorf("expected 0 models, got %v", status["models"])
	}
}

func TestServiceStartStop(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     false,
		AutoUnloadMins: 0,
	})

	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestServiceStartWithCUDA(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     true,
		AutoUnloadMins: 0,
	})

	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer svc.Stop()

	gpus := svc.GetGPUs()
	if len(gpus) != 1 {
		t.Fatalf("expected 1 GPU, got %d", len(gpus))
	}
	if gpus[0].Type != GPUNvidia {
		t.Errorf("expected GPU type %q, got %q", GPUNvidia, gpus[0].Type)
	}
	if gpus[0].MemoryTotal != 8*1024*1024*1024 {
		t.Errorf("expected 8GB total memory, got %d", gpus[0].MemoryTotal)
	}
	if gpus[0].Status != "available" {
		t.Errorf("expected status 'available', got %q", gpus[0].Status)
	}
}

func TestRegisterModel(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	model, err := svc.RegisterModel("llama-7b", ModelLLM, "ollama", "7B", "Q4_K_M", 4)
	if err != nil {
		t.Fatalf("RegisterModel failed: %v", err)
	}
	if model.Name != "llama-7b" {
		t.Errorf("expected name 'llama-7b', got %q", model.Name)
	}
	if model.Type != ModelLLM {
		t.Errorf("expected type %q, got %q", ModelLLM, model.Type)
	}
	if model.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got %q", model.Provider)
	}
	if model.Parameters != "7B" {
		t.Errorf("expected parameters '7B', got %q", model.Parameters)
	}
	if model.Quantization != "Q4_K_M" {
		t.Errorf("expected quantization 'Q4_K_M', got %q", model.Quantization)
	}
	if model.GPURequired != 4 {
		t.Errorf("expected GPURequired 4, got %d", model.GPURequired)
	}
	if model.ID == "" {
		t.Error("expected non-empty model ID")
	}

	models := svc.GetModels()
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
}

func TestLoadModel(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     true,
		AutoUnloadMins: 0,
	})

	svc.Start()
	// Note: svc.Stop() deadlocks when models are loaded (source code bug: Stop holds mu, unloadModel also locks mu)
	defer svc.cancel()

	model, _ := svc.RegisterModel("test-llm", ModelLLM, "ollama", "7B", "Q4_K_M", 4)

	err := svc.LoadModel(model.ID)
	if err != nil {
		t.Fatalf("LoadModel failed: %v", err)
	}

	// Wait for simulated loading
	time.Sleep(3 * time.Second)

	models := svc.GetModels()
	for _, m := range models {
		if m.ID == model.ID {
			if m.Status != StatusReady {
				t.Errorf("expected model status %q, got %q", StatusReady, m.Status)
			}
			if m.GPUDeviceID == "" {
				t.Error("expected non-empty GPUDeviceID after loading")
			}
			if m.LoadedAt == nil {
				t.Error("expected non-nil LoadedAt after loading")
			}
			return
		}
	}
	t.Error("model not found after loading")
}

func TestLoadModelNotFound(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	err := svc.LoadModel("nonexistent-model")
	if err == nil {
		t.Error("expected error for nonexistent model, got nil")
	}
}

func TestLoadModelNoGPU(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     false,
		AutoUnloadMins: 0,
	})

	svc.Start()
	defer svc.cancel()

	model, _ := svc.RegisterModel("test-llm", ModelLLM, "ollama", "7B", "Q4_K_M", 4)

	err := svc.LoadModel(model.ID)
	if err == nil {
		t.Error("expected error when no GPU available, got nil")
	}
}

func TestSubmitInference(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     true,
		AutoUnloadMins: 0,
	})

	svc.Start()
	defer svc.cancel()

	model, _ := svc.RegisterModel("test-llm", ModelLLM, "ollama", "7B", "Q4_K_M", 4)

	// Load model (wait for completion)
	svc.LoadModel(model.ID)
	time.Sleep(3 * time.Second)

	req, err := svc.SubmitInference(model.ID, "Hello, world!", InferParameters{
		MaxTokens:   100,
		Temperature: 0.7,
	})
	if err != nil {
		t.Fatalf("SubmitInference failed: %v", err)
	}
	if req.ModelID != model.ID {
		t.Errorf("expected modelID %q, got %q", model.ID, req.ModelID)
	}
	if req.Input != "Hello, world!" {
		t.Errorf("expected input 'Hello, world!', got %q", req.Input)
	}
	if req.Parameters.MaxTokens != 100 {
		t.Errorf("expected MaxTokens 100, got %d", req.Parameters.MaxTokens)
	}
	if req.ID == "" {
		t.Error("expected non-empty request ID")
	}
}

func TestSubmitInferenceModelNotFound(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	_, err := svc.SubmitInference("nonexistent", "test", InferParameters{})
	if err == nil {
		t.Error("expected error for nonexistent model, got nil")
	}
}

func TestSubmitInferenceModelNotReady(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	model, _ := svc.RegisterModel("test-llm", ModelLLM, "ollama", "7B", "Q4_K_M", 4)

	_, err := svc.SubmitInference(model.ID, "test", InferParameters{})
	if err == nil {
		t.Error("expected error when model not ready, got nil")
	}
}

func TestGetInferenceResult(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     true,
		AutoUnloadMins: 0,
	})

	svc.Start()
	defer svc.cancel()

	model, _ := svc.RegisterModel("test-llm", ModelLLM, "ollama", "7B", "Q4_K_M", 4)
	svc.LoadModel(model.ID)
	time.Sleep(3 * time.Second)

	req, _ := svc.SubmitInference(model.ID, "What is Go?", InferParameters{MaxTokens: 50})

	// Wait for inference to complete
	time.Sleep(500 * time.Millisecond)

	resp, err := svc.GetInferenceResult(req.ID)
	if err != nil {
		t.Fatalf("GetInferenceResult failed: %v", err)
	}
	if resp.Status != InferCompleted {
		t.Errorf("expected status %q, got %q", InferCompleted, resp.Status)
	}
	if resp.Output == "" {
		t.Error("expected non-empty output")
	}
	if resp.RequestID != req.ID {
		t.Errorf("expected requestID %q, got %q", req.ID, resp.RequestID)
	}
}

func TestGetInferenceResultNotFound(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	_, err := svc.GetInferenceResult("nonexistent-req")
	if err == nil {
		t.Error("expected error for nonexistent request, got nil")
	}
}

func TestGetGPUsEmpty(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     false,
		AutoUnloadMins: 0,
	})

	gpus := svc.GetGPUs()
	if len(gpus) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(gpus))
	}
}

func TestGetModelsEmpty(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		AutoUnloadMins: 0,
	})

	models := svc.GetModels()
	if len(models) != 0 {
		t.Errorf("expected 0 models, got %d", len(models))
	}
}

func TestGetServiceStatus(t *testing.T) {
	svc := NewGPUInferService(InferConfig{
		MaxConcurrent:  2,
		RequestTimeout: 30 * time.Second,
		EnableCUDA:     true,
		AutoUnloadMins: 0,
	})

	svc.Start()
	defer svc.cancel()

	svc.RegisterModel("model-a", ModelLLM, "ollama", "7B", "Q4_K_M", 4)
	svc.RegisterModel("model-b", ModelVision, "local", "3B", "Q8", 2)

	status := svc.GetServiceStatus()
	if status["gpus"] != 1 {
		t.Errorf("expected 1 gpu, got %v", status["gpus"])
	}
	if status["models"] != 2 {
		t.Errorf("expected 2 models, got %v", status["models"])
	}
	if status["gpu_memory_total"].(int64) != 8*1024*1024*1024 {
		t.Errorf("expected 8GB total GPU memory, got %v", status["gpu_memory_total"])
	}
}

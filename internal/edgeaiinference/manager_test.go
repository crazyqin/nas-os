package edgeaiinference

import (
	"os"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "edgeaiinference-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(&ManagerConfig{
		DataDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}

	if mgr.dataDir != tmpDir {
		t.Errorf("DataDir mismatch: got %s, want %s", mgr.dataDir, tmpDir)
	}
}

func TestNewManager_DefaultScheduler(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.schedulerCfg == nil {
		t.Fatal("Expected non-nil scheduler config")
	}

	if mgr.schedulerCfg.Strategy != "priority" {
		t.Errorf("Expected strategy 'priority', got %s", mgr.schedulerCfg.Strategy)
	}
}

func TestNewManager_EmptyDataDir(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{
		DataDir: "",
	})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.dataDir != "" {
		t.Errorf("Expected empty dataDir, got %s", mgr.dataDir)
	}
}

func TestManager_RegisterDevice(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	device := &ComputeDevice{
		ID:       "gpu-0",
		Name:     "NVIDIA Jetson",
		Type:     DeviceTypeGPU,
		MemoryMB: 8192,
	}

	err = mgr.RegisterDevice(device)
	if err != nil {
		t.Fatalf("Failed to register device: %v", err)
	}

	devices := mgr.ListDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}
}

func TestManager_RegisterDevice_EmptyID(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	device := &ComputeDevice{
		Name: "NVIDIA Jetson",
		Type: DeviceTypeGPU,
	}

	err = mgr.RegisterDevice(device)
	if err == nil {
		t.Fatal("Expected error for empty device ID")
	}
}

func TestManager_UnregisterDevice(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU})

	err = mgr.UnregisterDevice("gpu-0")
	if err != nil {
		t.Fatalf("Failed to unregister device: %v", err)
	}

	devices := mgr.ListDevices()
	if len(devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(devices))
	}
}

func TestManager_UnregisterDevice_NotExists(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = mgr.UnregisterDevice("non-existent")
	if err == nil {
		t.Fatal("Expected error for non-existent device")
	}
}

func TestManager_RegisterModel(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	model := &AIModel{
		ID:       "llama-7b",
		Name:     "LLaMA 7B",
		Type:     ModelTypeLLM,
		Version:  "1.0",
		MemoryMB: 4096,
		FilePath: "/models/llama-7b.bin",
	}

	err = mgr.RegisterModel(model)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	models := mgr.ListModels()
	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}
}

func TestManager_RegisterModel_EmptyID(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	model := &AIModel{
		Name: "LLaMA 7B",
		Type: ModelTypeLLM,
	}

	err = mgr.RegisterModel(model)
	if err == nil {
		t.Fatal("Expected error for empty model ID")
	}
}

func TestManager_LoadModel(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 8192})
	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})

	err = mgr.LoadModel("llama-7b", "gpu-0")
	if err != nil {
		t.Fatalf("Failed to load model: %v", err)
	}

	model, _ := mgr.GetModel("llama-7b")
	if model.Status != ModelStatusReady {
		t.Errorf("Expected model status 'ready', got %s", model.Status)
	}

	if model.DeviceID != "gpu-0" {
		t.Errorf("Expected device ID 'gpu-0', got %s", model.DeviceID)
	}
}

func TestManager_LoadModel_MemoryExceeded(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 2048})
	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})

	err = mgr.LoadModel("llama-7b", "gpu-0")
	if err == nil {
		t.Fatal("Expected error for insufficient memory")
	}
}

func TestManager_UnloadModel(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 8192})
	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})
	_ = mgr.LoadModel("llama-7b", "gpu-0")

	err = mgr.UnloadModel("llama-7b")
	if err != nil {
		t.Fatalf("Failed to unload model: %v", err)
	}

	model, _ := mgr.GetModel("llama-7b")
	if model.Status != ModelStatusUnloaded {
		t.Errorf("Expected model status 'unloaded', got %s", model.Status)
	}
}

func TestManager_SubmitInference(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 8192})
	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})
	_ = mgr.LoadModel("llama-7b", "gpu-0")

	req := &InferenceRequest{
		ModelID: "llama-7b",
		Input:   map[string]interface{}{"prompt": "Hello, world!"},
	}

	result, err := mgr.SubmitInference(req)
	if err != nil {
		t.Fatalf("Failed to submit inference: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.RequestID == "" {
		t.Error("Expected non-empty request ID")
	}
}

func TestManager_SubmitInference_ModelNotReady(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})

	req := &InferenceRequest{
		ModelID: "llama-7b",
		Input:   map[string]interface{}{"prompt": "Hello"},
	}

	_, err = mgr.SubmitInference(req)
	if err == nil {
		t.Fatal("Expected error for model not ready")
	}
}

func TestManager_GetMetrics(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	metrics := mgr.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected non-nil metrics")
	}
}

func TestManager_GetEvents(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 8192})
	_ = mgr.RegisterModel(&AIModel{ID: "llama-7b", Name: "LLaMA 7B", Type: ModelTypeLLM, MemoryMB: 4096, FilePath: "/models/llama-7b.bin"})

	events := mgr.GetEvents(10)
	if len(events) < 2 {
		t.Errorf("Expected at least 2 events, got %d", len(events))
	}
}

func TestManager_StartStop(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mgr.Start()
	if !mgr.running {
		t.Error("Expected manager to be running")
	}

	// 启动两次不应出错
	mgr.Start()

	mgr.Stop()
	if mgr.running {
		t.Error("Expected manager to be stopped")
	}
}

func TestManager_Subscribe(t *testing.T) {
	mgr, err := NewManager(&ManagerConfig{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	ch := mgr.Subscribe()
	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	_ = mgr.RegisterDevice(&ComputeDevice{ID: "gpu-0", Name: "GPU-0", Type: DeviceTypeGPU, MemoryMB: 8192})

	select {
	case evt := <-ch:
		if evt.Type != "device_registered" {
			t.Errorf("Expected event type 'device_registered', got %s", evt.Type)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

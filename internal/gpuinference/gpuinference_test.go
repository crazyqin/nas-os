package gpuinference

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelFormats(t *testing.T) {
	assert.Equal(t, "onnx", string(FormatONNX))
	assert.Equal(t, "tensorrt", string(FormatTensorRT))
	assert.Equal(t, "pytorch", string(FormatPyTorch))
	assert.Equal(t, "gguf", string(FormatGGUF))
}

func TestPrecisions(t *testing.T) {
	assert.Equal(t, "fp32", string(PrecisionFP32))
	assert.Equal(t, "fp16", string(PrecisionFP16))
	assert.Equal(t, "int8", string(PrecisionINT8))
	assert.Equal(t, "int4", string(PrecisionINT4))
}

func TestInferenceTasks(t *testing.T) {
	assert.Equal(t, "classification", string(TaskClassification))
	assert.Equal(t, "detection", string(TaskDetection))
	assert.Equal(t, "generation", string(TaskGeneration))
	assert.Equal(t, "embedding", string(TaskEmbedding))
	assert.Equal(t, "ocr", string(TaskOCR))
}

func TestLoadModel(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	gpu := &GPUDevice{
		ID:        0,
		Name:      "RTX 4090",
		TotalVRAM: 24 * 1024 * 1024 * 1024,
		FreeVRAM:  24 * 1024 * 1024 * 1024,
	}
	mgr.RegisterGPU(gpu)

	model := &Model{
		ID:        "model-001",
		Name:      "yolov8",
		Format:    FormatONNX,
		Task:      TaskDetection,
		Precision: PrecisionFP16,
		GPUDevice: 0,
		VRAMUsage: 2 * 1024 * 1024 * 1024,
		MaxBatch:  32,
	}

	err := mgr.LoadModel(model)
	require.NoError(t, err)
	assert.Equal(t, ModelStatusReady, model.Status)
	assert.NotNil(t, model.LoadedAt)

	// Duplicate
	err = mgr.LoadModel(model)
	assert.ErrorIs(t, err, ErrModelExists)
}

func TestUnloadModel(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	gpu := &GPUDevice{
		ID:        0,
		Name:      "RTX 4090",
		TotalVRAM: 24 * 1024 * 1024 * 1024,
		FreeVRAM:  24 * 1024 * 1024 * 1024,
	}
	mgr.RegisterGPU(gpu)

	model := &Model{
		ID:        "model-001",
		Name:      "test-model",
		GPUDevice: 0,
		VRAMUsage: 4 * 1024 * 1024 * 1024,
	}
	_ = mgr.LoadModel(model)

	err := mgr.UnloadModel("model-001")
	assert.NoError(t, err)

	_, err = mgr.GetModel("model-001")
	assert.ErrorIs(t, err, ErrModelNotFound)
}

func TestInsufficientVRAM(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	gpu := &GPUDevice{
		ID:        0,
		Name:      "GTX 1050",
		TotalVRAM: 2 * 1024 * 1024 * 1024,
		FreeVRAM:  2 * 1024 * 1024 * 1024,
	}
	mgr.RegisterGPU(gpu)

	model := &Model{
		ID:        "model-big",
		Name:      "llama-70b",
		GPUDevice: 0,
		VRAMUsage: 40 * 1024 * 1024 * 1024,
	}
	err := mgr.LoadModel(model)
	assert.ErrorIs(t, err, ErrInsufficientVRAM)
}

func TestSubmitInference(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	gpu := &GPUDevice{
		ID:        0,
		Name:      "RTX 4090",
		TotalVRAM: 24 * 1024 * 1024 * 1024,
		FreeVRAM:  24 * 1024 * 1024 * 1024,
	}
	mgr.RegisterGPU(gpu)

	model := &Model{
		ID:        "model-001",
		Name:      "test-model",
		GPUDevice: 0,
		VRAMUsage: 1024 * 1024 * 1024,
	}
	_ = mgr.LoadModel(model)

	req := &InferenceRequest{
		ID:      "req-001",
		ModelID: "model-001",
		Input:   map[string]interface{}{"image": "base64data"},
	}

	result, err := mgr.SubmitInference(req)
	require.NoError(t, err)
	assert.Equal(t, "req-001", result.RequestID)
	assert.Equal(t, "model-001", result.ModelID)
}

func TestSubmitToNonexistentModel(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	req := &InferenceRequest{
		ID:      "req-001",
		ModelID: "nonexistent",
		Input:   map[string]interface{}{},
	}

	_, err := mgr.SubmitInference(req)
	assert.ErrorIs(t, err, ErrModelNotFound)
}

func TestListModels(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	gpu := &GPUDevice{
		ID:        0,
		Name:      "GPU-0",
		TotalVRAM: 100 * 1024 * 1024 * 1024,
		FreeVRAM:  100 * 1024 * 1024 * 1024,
	}
	mgr.RegisterGPU(gpu)

	_ = mgr.LoadModel(&Model{ID: "m1", GPUDevice: 0, VRAMUsage: 1024})
	_ = mgr.LoadModel(&Model{ID: "m2", GPUDevice: 0, VRAMUsage: 1024})

	models := mgr.ListModels()
	assert.Len(t, models, 2)
}

func TestListGPUs(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	mgr.RegisterGPU(&GPUDevice{ID: 0, Name: "GPU-0", TotalVRAM: 8 * 1024 * 1024 * 1024, FreeVRAM: 8 * 1024 * 1024 * 1024})
	mgr.RegisterGPU(&GPUDevice{ID: 1, Name: "GPU-1", TotalVRAM: 8 * 1024 * 1024 * 1024, FreeVRAM: 8 * 1024 * 1024 * 1024})

	gpus := mgr.ListGPUs()
	assert.Len(t, gpus, 2)
}

func TestManagerClosed(t *testing.T) {
	mgr := NewManager(100)
	mgr.Close()

	err := mgr.LoadModel(&Model{ID: "m1"})
	assert.ErrorIs(t, err, ErrManagerClosed)
}

func TestUnloadNonexistentModel(t *testing.T) {
	mgr := NewManager(100)
	defer mgr.Close()

	err := mgr.UnloadModel("nonexistent")
	assert.ErrorIs(t, err, ErrModelNotFound)
}

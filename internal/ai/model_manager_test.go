// Package ai provides AI service integration for NAS-OS
package ai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================== ModelRegistry Extended Tests ====================

func TestNewModelRegistry_Extended(t *testing.T) {
	registry := NewModelRegistry()

	require.NotNil(t, registry)
	assert.NotNil(t, registry.models)
}

func TestModelRegistry_Get_Extended(t *testing.T) {
	registry := NewModelRegistry()

	model := &LocalModel{Name: "test-model", Source: "ollama"}
	registry.Register("test-model", model)

	// Get existing model
	retrieved := registry.Get("test-model")
	assert.Equal(t, "test-model", retrieved.Name)

	// Get non-existent model
	retrieved = registry.Get("unknown")
	assert.Nil(t, retrieved)
}

// ==================== LocalModel Tests ====================

func TestLocalModel(t *testing.T) {
	now := time.Now()
	model := LocalModel{
		Name:       "llama2",
		Source:     "ollama",
		Version:    "7b",
		LocalPath:  "/models/llama2",
		Size:       4072257280,
		Installed:  true,
		ModifiedAt: now,
	}

	assert.Equal(t, "llama2", model.Name)
	assert.Equal(t, "ollama", model.Source)
	assert.Equal(t, "7b", model.Version)
	assert.Equal(t, "/models/llama2", model.LocalPath)
	assert.Equal(t, int64(4072257280), model.Size)
	assert.True(t, model.Installed)
}

// ==================== ModelInfo Tests ====================

func TestModelInfo(t *testing.T) {
	info := ModelInfo{
		Name:         "gpt-4",
		ID:           "gpt-4-0613",
		Size:         1024000000,
		Digest:       "abc123",
		Parameters:   "175B",
		Quantization: "fp16",
		Details: map[string]any{
			"backend": "openai",
		},
		Capabilities: []ModelCapability{{Type: "chat"}},
	}

	assert.Equal(t, "gpt-4", info.Name)
	assert.Equal(t, "gpt-4-0613", info.ID)
	assert.Equal(t, "175B", info.Parameters)
	assert.Len(t, info.Capabilities, 1)
}

// ==================== DownloadProgress Tests ====================

func TestDownloadProgress(t *testing.T) {
	progress := DownloadProgress{
		ModelName:  "llama2",
		Source:     "ollama",
		TotalBytes: 4072257280,
		Downloaded: 2000000000,
		Percentage: 49.1,
		Speed:      "50 MB/s",
		Status:     "downloading",
		StartedAt:  time.Now(),
	}

	assert.Equal(t, "llama2", progress.ModelName)
	assert.Equal(t, "ollama", progress.Source)
	assert.Equal(t, int64(4072257280), progress.TotalBytes)
	assert.Equal(t, 49.1, progress.Percentage)
	assert.Equal(t, "downloading", progress.Status)
}

func TestDownloadProgress_Completed(t *testing.T) {
	now := time.Now()
	progress := DownloadProgress{
		ModelName:   "test-model",
		Status:      "completed",
		Percentage:  100,
		StartedAt:   now.Add(-5 * time.Minute),
		CompletedAt: now,
	}

	assert.Equal(t, "completed", progress.Status)
	assert.Equal(t, 100.0, progress.Percentage)
}

func TestDownloadProgress_Failed(t *testing.T) {
	progress := DownloadProgress{
		ModelName: "test-model",
		Status:    "failed",
		Error:     "connection refused",
	}

	assert.Equal(t, "failed", progress.Status)
	assert.Equal(t, "connection refused", progress.Error)
}

// ==================== ModelDownloadRequest Tests ====================

func TestModelDownloadRequest(t *testing.T) {
	req := ModelDownloadRequest{
		ModelName: "llama2",
		Source:    "ollama",
		Version:   "7b-chat",
	}

	assert.Equal(t, "llama2", req.ModelName)
	assert.Equal(t, "ollama", req.Source)
	assert.Equal(t, "7b-chat", req.Version)
}
package amdaccel

import (
	"testing"
)

func TestNewAMDAccelerator(t *testing.T) {
	config := AcceleratorConfig{
		EnableVideoTranscode: true,
		EnableAIInference:    true,
		MemoryLimit:          4 * 1024 * 1024 * 1024, // 4GB
	}

	accel := NewAMDAccelerator(config)
	if accel == nil {
		t.Fatal("NewAMDAccelerator returned nil")
	}
}

func TestAMDAccelerator_GetGPUCount(t *testing.T) {
	config := AcceleratorConfig{
		EnableVideoTranscode: true,
		EnableAIInference:    true,
	}

	accel := NewAMDAccelerator(config)
	count := accel.GetGPUCount()

	// 在没有AMD显卡的环境中，count应该为0
	t.Logf("GPU count: %d", count)
}

func TestAMDAccelerator_IsAvailable(t *testing.T) {
	config := AcceleratorConfig{
		EnableVideoTranscode: true,
		EnableAIInference:    true,
	}

	accel := NewAMDAccelerator(config)
	available := accel.IsAvailable()

	t.Logf("AMD GPU available: %v", available)
}

func TestAMDAccelerator_TranscodeVideo_Disabled(t *testing.T) {
	config := AcceleratorConfig{
		EnableVideoTranscode: false,
	}

	accel := NewAMDAccelerator(config)
	err := accel.TranscodeVideo("input.mp4", "output.mp4", TranscodeOptions{})

	if err == nil {
		t.Error("Expected error when video transcode is disabled")
	}
}

func TestAMDAccelerator_TranscodeVideo_NoGPU(t *testing.T) {
	config := AcceleratorConfig{
		EnableVideoTranscode: true,
	}

	accel := NewAMDAccelerator(config)

	// 在没有GPU的环境中，应该返回错误
	if accel.GetGPUCount() == 0 {
		err := accel.TranscodeVideo("input.mp4", "output.mp4", TranscodeOptions{})
		if err == nil {
			t.Error("Expected error when no GPU available")
		}
	}
}

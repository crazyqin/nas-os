package gpu

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Scheduler manages GPU resource allocation for AI tasks
type Scheduler struct {
	gpuType     GPUType
	queue       []GPUTask
	active      map[string]*GPUTask
	allocations map[string]GPUAllocation
	maxSlots    int
	mu          sync.Mutex
}

// GPUType defines GPU type
type GPUType string

const (
	GPUTypeIntel  GPUType = "intel"  // Intel QuickSync
	GPUTypeNVIDIA GPUType = "nvidia" // NVIDIA CUDA
	GPUTypeAMD    GPUType = "amd"    // AMD ROCm
	GPUTypeCPU    GPUType = "cpu"    // CPU fallback
)

// GPUTask represents a GPU task
type GPUTask struct {
	ID       string
	Type     TaskType
	Priority int
	MemoryMB int
	Duration int // estimated duration in seconds
	Context  context.Context
	Callback func(error)
}

// TaskType defines task type
type TaskType string

const (
	TaskTypeFaceDetection   TaskType = "face_detection"
	TaskTypeFaceEmbedding   TaskType = "face_embedding"
	TaskTypeOCR             TaskType = "ocr"
	TaskTypeLLMInference    TaskType = "llm_inference"
	TaskTypeEmbedding       TaskType = "embedding"
	TaskTypeImageProcessing TaskType = "image_processing"
)

// GPUAllocation represents GPU resource allocation
type GPUAllocation struct {
	TaskID   string
	MemoryMB int
	Slots    int
	Start    int64
	End      int64
}

// SchedulerConfig for GPU scheduler
type SchedulerConfig struct {
	GPUType     GPUType
	MaxSlots    int
	MaxMemoryMB int
}

// NewScheduler creates a new GPU scheduler
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	maxSlots := cfg.MaxSlots
	if maxSlots <= 0 {
		maxSlots = 4 // Default to 4 concurrent slots
	}

	return &Scheduler{
		gpuType:     cfg.GPUType,
		queue:       make([]GPUTask, 0),
		active:      make(map[string]*GPUTask),
		allocations: make(map[string]GPUAllocation),
		maxSlots:    maxSlots,
	}
}

// Submit submits a task to the scheduler
func (s *Scheduler) Submit(task *GPUTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we can start immediately
	if len(s.active) < s.maxSlots && s.canAllocate(task) {
		s.startTask(task)
		return nil
	}

	// Add to queue
	s.queue = append(s.queue, *task)
	s.sortQueue()

	return nil
}

// canAllocate checks if task can be allocated
func (s *Scheduler) canAllocate(task *GPUTask) bool {
	// Simple check - can always allocate if slots available
	// More complex logic would check memory, etc.
	return true
}

// startTask starts a GPU task
func (s *Scheduler) startTask(task *GPUTask) {
	s.active[task.ID] = task
	s.allocations[task.ID] = GPUAllocation{
		TaskID:   task.ID,
		Start:    now(),
		MemoryMB: task.MemoryMB,
	}

	// Execute task asynchronously
	go s.executeTask(task)
}

// executeTask executes a GPU task
func (s *Scheduler) executeTask(task *GPUTask) {
	// Simulate execution based on GPU type
	var err error

	switch s.gpuType {
	case GPUTypeIntel:
		err = s.executeWithIntel(task)
	case GPUTypeNVIDIA:
		err = s.executeWithNVIDIA(task)
	case GPUTypeAMD:
		err = s.executeWithAMD(task)
	default:
		err = s.executeWithCPU(task)
	}

	// Call callback and cleanup
	s.mu.Lock()
	delete(s.active, task.ID)
	delete(s.allocations, task.ID)
	s.mu.Unlock()

	if task.Callback != nil {
		task.Callback(err)
	}

	// Try to start next queued task
	s.tryStartNext()
}

// executeWithIntel executes on Intel GPU
func (s *Scheduler) executeWithIntel(task *GPUTask) error {
	return runTaskWithContext(task)
}

// executeWithNVIDIA executes on NVIDIA GPU
func (s *Scheduler) executeWithNVIDIA(task *GPUTask) error {
	return runTaskWithContext(task)
}

// executeWithAMD executes on AMD GPU
func (s *Scheduler) executeWithAMD(task *GPUTask) error {
	return runTaskWithContext(task)
}

// executeWithCPU executes on CPU
func (s *Scheduler) executeWithCPU(task *GPUTask) error {
	return runTaskWithContext(task)
}

// tryStartNext tries to start next queued task
func (s *Scheduler) tryStartNext() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) == 0 {
		return
	}

	if len(s.active) >= s.maxSlots {
		return
	}

	// Get highest priority task
	next := s.queue[0]
	s.queue = s.queue[1:]

	s.startTask(&next)
}

// sortQueue sorts queue by priority
func (s *Scheduler) sortQueue() {
	// Simple priority sort (higher priority first)
	for i := 0; i < len(s.queue)-1; i++ {
		for j := i + 1; j < len(s.queue); j++ {
			if s.queue[j].Priority > s.queue[i].Priority {
				s.queue[i], s.queue[j] = s.queue[j], s.queue[i]
			}
		}
	}
}

// Cancel cancels a task
func (s *Scheduler) Cancel(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if active
	if _, exists := s.active[taskID]; exists {
		return ErrTaskActive
	}

	// Remove from queue
	for i, task := range s.queue {
		if task.ID == taskID {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return nil
		}
	}

	return ErrTaskNotFound
}

// GetStatus returns scheduler status
func (s *Scheduler) GetStatus() SchedulerStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	return SchedulerStatus{
		GPUType:        s.gpuType,
		ActiveTasks:    len(s.active),
		QueuedTasks:    len(s.queue),
		MaxSlots:       s.maxSlots,
		AvailableSlots: s.maxSlots - len(s.active),
	}
}

// SchedulerStatus holds scheduler status
type SchedulerStatus struct {
	GPUType        GPUType `json:"gpu_type"`
	ActiveTasks    int     `json:"active_tasks"`
	QueuedTasks    int     `json:"queued_tasks"`
	MaxSlots       int     `json:"max_slots"`
	AvailableSlots int     `json:"available_slots"`
}

// SetGPUType changes GPU type
func (s *Scheduler) SetGPUType(gpuType GPUType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gpuType = gpuType
}

// GetGPUType returns current GPU type
func (s *Scheduler) GetGPUType() GPUType {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gpuType
}

// ClearQueue clears the task queue
func (s *Scheduler) ClearQueue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = make([]GPUTask, 0)
}

// DetectGPUType auto-detects available GPU
func DetectGPUType() GPUType {
	// Check for Intel GPU
	if hasIntelGPU() {
		return GPUTypeIntel
	}

	// Check for NVIDIA GPU
	if hasNVIDIAGPU() {
		return GPUTypeNVIDIA
	}

	// Check for AMD GPU
	if hasAMDGPU() {
		return GPUTypeAMD
	}

	// Fallback to CPU
	return GPUTypeCPU
}

// GPU detection helpers
func hasIntelGPU() bool {
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return true
	}
	_, err := os.Stat("/sys/module/i915")
	return err == nil
}

func hasNVIDIAGPU() bool {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}
	_, err := os.Stat("/proc/driver/nvidia/version")
	return err == nil
}

func hasAMDGPU() bool {
	if _, err := os.Stat("/sys/module/amdgpu"); err == nil {
		return true
	}
	_, err := os.Stat("/dev/dri/renderD129")
	return err == nil
}

func runTaskWithContext(task *GPUTask) error {
	ctx := task.Context
	if ctx == nil {
		ctx = context.Background()
	}
	d := time.Duration(task.Duration) * time.Second
	if d <= 0 {
		d = time.Millisecond
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Helper functions
func now() int64 {
	return time.Now().Unix()
}

// Errors
var (
	ErrTaskNotFound = errors.New("task not found")
	ErrTaskActive   = errors.New("task is already active, cannot cancel")
)

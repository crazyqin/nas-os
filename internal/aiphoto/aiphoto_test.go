package aiphoto

import (
	"context"
	"image"
	"image/color"
	"os"
	"testing"
)

// 创建测试用的简单图像
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x + y) % 256),
				G: uint8((x * 2) % 256),
				B: uint8((y * 2) % 256),
				A: 255,
			})
		}
	}
	return img
}

// 创建带噪声的测试图像
func createNoisyTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 基础颜色 + 简单噪声模拟
			base := uint8((x + y) % 128)
			noise := uint8((x*7 + y*13) % 50)
			img.Set(x, y, color.RGBA{
				R: base + noise,
				G: base,
				B: base,
				A: 255,
			})
		}
	}
	return img
}

// ========== Denoise 测试 ==========

func TestDenoiser_NewDenoiser(t *testing.T) {
	tests := []struct {
		name string
		opts *DenoiseOptions
	}{
		{
			name: "nil options",
			opts: nil,
		},
		{
			name: "default options",
			opts: DefaultDenoiseOptions(),
		},
		{
			name: "custom options",
			opts: &DenoiseOptions{
				Strength:       0.7,
				PreserveDetail: true,
				Algorithm:      "bilateral",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDenoiser(tt.opts)
			if d == nil {
				t.Fatal("NewDenoiser returned nil")
			}
		})
	}
}

func TestDenoiser_Denoise(t *testing.T) {
	d := NewDenoiser(DefaultDenoiseOptions())
	ctx := context.Background()
	src := createNoisyTestImage(32, 32)

	dst, err := d.Denoise(ctx, src)
	if err != nil {
		t.Fatalf("Denoise failed: %v", err)
	}

	if dst == nil {
		t.Fatal("Denoise returned nil image")
	}

	bounds := dst.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Fatalf("Output size mismatch: got %dx%d, want 32x32", bounds.Dx(), bounds.Dy())
	}
}

func TestDenoiser_DenoiseNil(t *testing.T) {
	d := NewDenoiser(nil)
	_, err := d.Denoise(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil image")
	}
}

func TestDenoiser_DenoiseCancel(t *testing.T) {
	d := NewDenoiser(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := d.Denoise(ctx, createTestImage(16, 16))
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestDenoiser_Algorithms(t *testing.T) {
	algorithms := []string{"nlm", "bilateral", "bm3d"}
	src := createNoisyTestImage(16, 16)

	for _, algo := range algorithms {
		t.Run(algo, func(t *testing.T) {
			d := NewDenoiser(&DenoiseOptions{
				Strength:       0.5,
				PreserveDetail: true,
				Algorithm:      algo,
			})
			dst, err := d.Denoise(context.Background(), src)
			if err != nil {
				t.Fatalf("Denoise with %s failed: %v", algo, err)
			}
			if dst == nil {
				t.Fatal("Denoise returned nil")
			}
		})
	}
}

func TestDenoiseOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *DenoiseOptions
		wantErr bool
	}{
		{"valid default", DefaultDenoiseOptions(), false},
		{"valid custom", &DenoiseOptions{Strength: 0.7, Algorithm: "bilateral"}, false},
		{"invalid strength high", &DenoiseOptions{Strength: 1.5}, true},
		{"invalid strength low", &DenoiseOptions{Strength: -0.1}, true},
		{"invalid algorithm", &DenoiseOptions{Algorithm: "invalid"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== Upscale 测试 ==========

func TestUpscaler_NewUpscaler(t *testing.T) {
	tests := []struct {
		name string
		opts *UpscaleOptions
	}{
		{"nil options", nil},
		{"default options", DefaultUpscaleOptions()},
		{"scale 4", &UpscaleOptions{ScaleFactor: 4, Model: "lanczos"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewUpscaler(tt.opts)
			if u == nil {
				t.Fatal("NewUpscaler returned nil")
			}
		})
	}
}

func TestUpscaler_Upscale(t *testing.T) {
	u := NewUpscaler(&UpscaleOptions{ScaleFactor: 2, Model: "lanczos"})
	ctx := context.Background()
	src := createTestImage(16, 16)

	dst, err := u.Upscale(ctx, src)
	if err != nil {
		t.Fatalf("Upscale failed: %v", err)
	}

	bounds := dst.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Fatalf("Output size mismatch: got %dx%d, want 32x32", bounds.Dx(), bounds.Dy())
	}
}

func TestUpscaler_Upscale4x(t *testing.T) {
	u := NewUpscaler(&UpscaleOptions{ScaleFactor: 4, Model: "lanczos"})
	src := createTestImage(8, 8)

	dst, err := u.Upscale(context.Background(), src)
	if err != nil {
		t.Fatalf("Upscale 4x failed: %v", err)
	}

	bounds := dst.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Fatalf("Output size mismatch: got %dx%d, want 32x32", bounds.Dx(), bounds.Dy())
	}
}

func TestUpscaler_UpscaleNil(t *testing.T) {
	u := NewUpscaler(nil)
	_, err := u.Upscale(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil image")
	}
}

func TestUpscaler_Models(t *testing.T) {
	models := []string{"realesrgan", "esrgan", "lanczos"}
	src := createTestImage(8, 8)

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			u := NewUpscaler(&UpscaleOptions{ScaleFactor: 2, Model: model})
			dst, err := u.Upscale(context.Background(), src)
			if err != nil {
				t.Fatalf("Upscale with %s failed: %v", model, err)
			}
			if dst == nil {
				t.Fatal("Upscale returned nil")
			}
		})
	}
}

func TestUpscaleOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *UpscaleOptions
		wantErr bool
	}{
		{"valid default", DefaultUpscaleOptions(), false},
		{"valid 4x", &UpscaleOptions{ScaleFactor: 4}, false},
		{"invalid scale", &UpscaleOptions{ScaleFactor: 3}, true},
		{"invalid model", &UpscaleOptions{Model: "invalid"}, true},
		{"invalid format", &UpscaleOptions{Format: "bmp"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== Restore 测试 ==========

func TestRestorer_NewRestorer(t *testing.T) {
	tests := []struct {
		name string
		opts *RestoreOptions
	}{
		{"nil options", nil},
		{"default options", DefaultRestoreOptions()},
		{"custom options", &RestoreOptions{RepairScratches: true, Strength: 0.8}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRestorer(tt.opts)
			if r == nil {
				t.Fatal("NewRestorer returned nil")
			}
		})
	}
}

func TestRestorer_Restore(t *testing.T) {
	r := NewRestorer(DefaultRestoreOptions())
	src := createTestImage(64, 64)

	dst, err := r.Restore(context.Background(), src)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if dst == nil {
		t.Fatal("Restore returned nil image")
	}

	bounds := dst.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Fatalf("Output size mismatch: got %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestRestorer_RestoreNil(t *testing.T) {
	r := NewRestorer(nil)
	_, err := r.Restore(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil image")
	}
}

func TestRestorer_RestoreCancel(t *testing.T) {
	r := NewRestorer(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Restore(ctx, createTestImage(16, 16))
	if err == nil {
		t.Fatal("Expected error for cancelled context")
	}
}

func TestRestoreOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *RestoreOptions
		wantErr bool
	}{
		{"valid default", DefaultRestoreOptions(), false},
		{"valid custom", &RestoreOptions{Strength: 0.5}, false},
		{"invalid strength high", &RestoreOptions{Strength: 1.5}, true},
		{"invalid strength low", &RestoreOptions{Strength: -0.1}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== SmartCrop 测试 ==========

func TestSmartCropper_NewSmartCropper(t *testing.T) {
	tests := []struct {
		name string
		opts *SmartCropOptions
	}{
		{"nil options", nil},
		{"default options", DefaultSmartCropOptions()},
		{"face strategy", &SmartCropOptions{TargetWidth: 100, TargetHeight: 100, Strategy: "face"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := NewSmartCropper(tt.opts)
			if sc == nil {
				t.Fatal("NewSmartCropper returned nil")
			}
		})
	}
}

func TestSmartCropper_SmartCrop(t *testing.T) {
	sc := NewSmartCropper(&SmartCropOptions{
		TargetWidth:  16,
		TargetHeight: 16,
		Strategy:     "center",
	})
	src := createTestImage(32, 32)

	result, err := sc.SmartCrop(context.Background(), src)
	if err != nil {
		t.Fatalf("SmartCrop failed: %v", err)
	}

	if result == nil {
		t.Fatal("SmartCrop returned nil result")
	}

	if result.Image == nil {
		t.Fatal("SmartCrop result has nil image")
	}

	bounds := result.Image.Bounds()
	if bounds.Dx() != 16 || bounds.Dy() != 16 {
		t.Fatalf("Output size mismatch: got %dx%d, want 16x16", bounds.Dx(), bounds.Dy())
	}
}

func TestSmartCropper_SmartCropNil(t *testing.T) {
	sc := NewSmartCropper(nil)
	_, err := sc.SmartCrop(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error for nil image")
	}
}

func TestSmartCropper_SmartCropTooLarge(t *testing.T) {
	sc := NewSmartCropper(&SmartCropOptions{
		TargetWidth:  100,
		TargetHeight: 100,
	})
	src := createTestImage(32, 32)

	_, err := sc.SmartCrop(context.Background(), src)
	if err == nil {
		t.Fatal("Expected error for target larger than source")
	}
}

func TestSmartCropper_Strategies(t *testing.T) {
	strategies := []string{"entropy", "face", "center", "attention"}
	src := createTestImage(64, 64)

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			sc := NewSmartCropper(&SmartCropOptions{
				TargetWidth:  32,
				TargetHeight: 32,
				Strategy:     strategy,
			})
			result, err := sc.SmartCrop(context.Background(), src)
			if err != nil {
				t.Fatalf("SmartCrop with %s failed: %v", strategy, err)
			}
			if result == nil || result.Image == nil {
				t.Fatal("SmartCrop returned nil")
			}
		})
	}
}

func TestSmartCropOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    *SmartCropOptions
		wantErr bool
	}{
		{"valid default", DefaultSmartCropOptions(), false},
		{"valid custom", &SmartCropOptions{TargetWidth: 100, TargetHeight: 100, Strategy: "face"}, false},
		{"zero width", &SmartCropOptions{TargetWidth: 0, TargetHeight: 100}, true},
		{"zero height", &SmartCropOptions{TargetWidth: 100, TargetHeight: 0}, true},
		{"invalid strategy", &SmartCropOptions{TargetWidth: 100, TargetHeight: 100, Strategy: "invalid"}, true},
		{"invalid padding high", &SmartCropOptions{TargetWidth: 100, TargetHeight: 100, PaddingRatio: 0.5}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== Queue 测试 ==========

func TestQueue_NewQueue(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		q := NewQueue(t.TempDir(), nil)
		if q == nil {
			t.Fatal("NewQueue returned nil")
		}
	})

	t.Run("default config", func(t *testing.T) {
		q := NewQueue(t.TempDir(), DefaultQueueConfig())
		if q == nil {
			t.Fatal("NewQueue returned nil")
		}
	})
}

func TestQueue_GetState(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	if q.GetState() != QueueStateIdle {
		t.Fatalf("Expected idle state, got %s", q.GetState())
	}
}

func TestQueue_GetStats(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	stats := q.GetStats()
	if stats["total"] != 0 {
		t.Fatalf("Expected 0 total tasks, got %d", stats["total"])
	}
}

func TestQueue_Submit(t *testing.T) {
	// 创建临时测试文件
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.jpg"
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	q := NewQueue(tmpDir, nil)

	task, err := q.Submit(TaskTypeDenoise, tmpFile, DefaultDenoiseOptions())
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	if task == nil {
		t.Fatal("Submit returned nil task")
	}

	if task.Status != TaskStatusPending {
		t.Fatalf("Expected pending status, got %s", task.Status)
	}
}

func TestQueue_SubmitNonExistent(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	_, err := q.Submit(TaskTypeDenoise, "/nonexistent/file.jpg", nil)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestQueue_GetTask(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.jpg"
	os.WriteFile(tmpFile, []byte("test"), 0644)

	q := NewQueue(tmpDir, nil)
	task, _ := q.Submit(TaskTypeDenoise, tmpFile, nil)

	fetched, err := q.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if fetched.ID != task.ID {
		t.Fatalf("Task ID mismatch: got %s, want %s", fetched.ID, task.ID)
	}
}

func TestQueue_GetTaskNotFound(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	_, err := q.GetTask("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent task")
	}
}

func TestQueue_ListTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.jpg"
	os.WriteFile(tmpFile, []byte("test"), 0644)

	q := NewQueue(tmpDir, nil)
	q.Submit(TaskTypeDenoise, tmpFile, nil)

	tasks := q.ListTasks(TaskStatusPending)
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 pending task, got %d", len(tasks))
	}

	tasks = q.ListTasks(TaskStatusCompleted)
	if len(tasks) != 0 {
		t.Fatalf("Expected 0 completed tasks, got %d", len(tasks))
	}
}

func TestQueue_CancelTask(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.jpg"
	os.WriteFile(tmpFile, []byte("test"), 0644)

	q := NewQueue(tmpDir, nil)
	task, _ := q.Submit(TaskTypeDenoise, tmpFile, nil)

	err := q.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask failed: %v", err)
	}

	fetched, _ := q.GetTask(task.ID)
	if fetched.Status != TaskStatusCancelled {
		t.Fatalf("Expected cancelled status, got %s", fetched.Status)
	}
}

func TestQueue_CancelTaskNotFound(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	err := q.CancelTask("nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent task")
	}
}

func TestQueue_RegisterProcessor(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)
	processor := func(ctx context.Context, task *PhotoTask) (*ProcessResult, error) {
		return &ProcessResult{TaskID: task.ID, Success: true}, nil
	}

	q.RegisterProcessor(TaskTypeDenoise, processor)
	// 没有 panic 就算成功
}

func TestQueue_SetCallbacks(t *testing.T) {
	q := NewQueue(t.TempDir(), nil)

	q.SetOnComplete(func(task *PhotoTask, result *ProcessResult) {})
	q.SetOnProgress(func(task *PhotoTask, progress float64) {})
	// 没有 panic 就算成功
}

func TestDefaultQueueConfig(t *testing.T) {
	c := DefaultQueueConfig()
	if c.MaxConcurrent != 2 {
		t.Fatalf("Expected MaxConcurrent 2, got %d", c.MaxConcurrent)
	}
	if c.MaxRetries != 3 {
		t.Fatalf("Expected MaxRetries 3, got %d", c.MaxRetries)
	}
}

// ========== 辅助函数测试 ==========

func TestClampUint8(t *testing.T) {
	tests := []struct {
		input int
		want  uint8
	}{
		{0, 0},
		{128, 128},
		{255, 255},
		{-1, 0},
		{256, 255},
		{-100, 0},
		{1000, 255},
	}

	for _, tt := range tests {
		got := clampUint8(tt.input)
		if got != tt.want {
			t.Errorf("clampUint8(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestMaxMinInt(t *testing.T) {
	if maxInt(1, 2) != 2 {
		t.Error("maxInt(1,2) should be 2")
	}
	if maxInt(2, 1) != 2 {
		t.Error("maxInt(2,1) should be 2")
	}
	if minInt(1, 2) != 1 {
		t.Error("minInt(1,2) should be 1")
	}
	if minInt(2, 1) != 1 {
		t.Error("minInt(2,1) should be 1")
	}
}

func TestAbsInt(t *testing.T) {
	if absInt(5) != 5 {
		t.Error("absInt(5) should be 5")
	}
	if absInt(-5) != 5 {
		t.Error("absInt(-5) should be 5")
	}
	if absInt(0) != 0 {
		t.Error("absInt(0) should be 0")
	}
}

func TestCubicWeight(t *testing.T) {
	// cubicWeight(0) 应该接近 1
	w := cubicWeight(0)
	if w < 0.9 || w > 1.1 {
		t.Errorf("cubicWeight(0) = %f, want ~1", w)
	}

	// cubicWeight(2) 应该接近 0
	w = cubicWeight(2)
	if w < -0.1 || w > 0.1 {
		t.Errorf("cubicWeight(2) = %f, want ~0", w)
	}
}

func TestLanczosKernel(t *testing.T) {
	// lanczos(0) = 1
	w := lanczosKernel(0, 3)
	if w < 0.9 || w > 1.1 {
		t.Errorf("lanczosKernel(0,3) = %f, want ~1", w)
	}

	// lanczos(3) = 0
	w = lanczosKernel(3, 3)
	if w < -0.1 || w > 0.1 {
		t.Errorf("lanczosKernel(3,3) = %f, want ~0", w)
	}
}

func TestCalculateEntropy(t *testing.T) {
	// 均匀分布的图像应该有较高熵
	img := createTestImage(32, 32)
	rect := image.Rect(0, 0, 32, 32)
	entropy := calculateEntropy(img, rect)
	if entropy <= 0 {
		t.Errorf("Expected positive entropy, got %f", entropy)
	}

	// 纯色图像应该有较低熵
	solidImg := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			solidImg.Set(x, y, color.RGBA{R: 128, G: 128, B: 128, A: 255})
		}
	}
	solidEntropy := calculateEntropy(solidImg, rect)
	if solidEntropy > entropy {
		t.Errorf("Solid image entropy (%f) should be less than gradient entropy (%f)", solidEntropy, entropy)
	}
}

// ========== 类型测试 ==========

func TestTaskTypes(t *testing.T) {
	if TaskTypeDenoise != "denoise" {
		t.Error("TaskTypeDenoise should be 'denoise'")
	}
	if TaskTypeUpscale != "upscale" {
		t.Error("TaskTypeUpscale should be 'upscale'")
	}
	if TaskTypeRestore != "restore" {
		t.Error("TaskTypeRestore should be 'restore'")
	}
	if TaskTypeSmartCrop != "smartcrop" {
		t.Error("TaskTypeSmartCrop should be 'smartcrop'")
	}
}

func TestTaskStatuses(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusProcessing,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCancelled,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("TaskStatus should not be empty")
		}
	}
}

func TestQueueStates(t *testing.T) {
	states := []QueueState{
		QueueStateIdle,
		QueueStateRunning,
		QueueStatePaused,
		QueueStateStopping,
	}

	for _, s := range states {
		if s == "" {
			t.Error("QueueState should not be empty")
		}
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test",
		Message: "test error",
	}

	expected := "参数校验失败 [test]: test error"
	if err.Error() != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestDefaultOptions(t *testing.T) {
	d := DefaultDenoiseOptions()
	if d.Strength != 0.5 {
		t.Errorf("DefaultDenoiseOptions.Strength = %f, want 0.5", d.Strength)
	}

	u := DefaultUpscaleOptions()
	if u.ScaleFactor != 2 {
		t.Errorf("DefaultUpscaleOptions.ScaleFactor = %d, want 2", u.ScaleFactor)
	}

	r := DefaultRestoreOptions()
	if r.Strength != 0.7 {
		t.Errorf("DefaultRestoreOptions.Strength = %f, want 0.7", r.Strength)
	}

	sc := DefaultSmartCropOptions()
	if sc.TargetWidth != 1024 {
		t.Errorf("DefaultSmartCropOptions.TargetWidth = %d, want 1024", sc.TargetWidth)
	}
}

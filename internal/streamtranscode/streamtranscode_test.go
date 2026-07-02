package streamtranscode

import (
	"testing"
	"time"
)

// 辅助函数：创建测试用配置.
func createTestConfig() TranscodeConfig {
	return TranscodeConfig{
		Resolution:   Resolution1080p,
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		VideoBitrate: 8000,
		AudioBitrate: 192,
		FPS:          30,
	}
}

func TestNewEngine(t *testing.T) {
	engine := NewEngine(4, 50)
	if engine == nil {
		t.Fatal("NewEngine 返回 nil")
	}

	if engine.maxConcurrency != 4 {
		t.Errorf("期望 maxConcurrency=4, 实际=%d", engine.maxConcurrency)
	}

	if engine.maxQueueSize != 50 {
		t.Errorf("期望 maxQueueSize=50, 实际=%d", engine.maxQueueSize)
	}

	// 检查内置预设是否加载
	if len(engine.presets) < 5 {
		t.Errorf("期望至少5个内置预设, 实际=%d", len(engine.presets))
	}
}

func TestNewEngineDefaults(t *testing.T) {
	engine := NewEngine(0, 0)
	if engine.maxConcurrency != 2 {
		t.Errorf("期望默认 maxConcurrency=2, 实际=%d", engine.maxConcurrency)
	}
	if engine.maxQueueSize != 100 {
		t.Errorf("期望默认 maxQueueSize=100, 实际=%d", engine.maxQueueSize)
	}
}

func TestCreateTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	task, err := engine.CreateTask("/input/video.mp4", "/output/video.mp4", config, PriorityNormal)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if task.ID == "" {
		t.Error("任务ID不应为空")
	}
	if task.InputFile != "/input/video.mp4" {
		t.Errorf("期望 InputFile=/input/video.mp4, 实际=%s", task.InputFile)
	}
	if task.Status != TaskStatusPending {
		t.Errorf("期望状态=pending, 实际=%s", task.Status)
	}
	if task.Priority != PriorityNormal {
		t.Errorf("期望优先级=%d, 实际=%d", PriorityNormal, task.Priority)
	}
	if task.Progress != 0 {
		t.Errorf("期望进度=0, 实际=%f", task.Progress)
	}
}

func TestCreateTaskInvalidConfig(t *testing.T) {
	engine := NewEngine(2, 10)

	// 无效分辨率
	config := TranscodeConfig{
		Resolution:   "123x456",
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		VideoBitrate: 8000,
		AudioBitrate: 192,
		FPS:          30,
	}

	_, err := engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	if err == nil {
		t.Error("期望返回错误，但没有")
	}

	// 无效视频编码
	config.Resolution = Resolution1080p
	config.VideoCodec = "INVALID"
	_, err = engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	if err == nil {
		t.Error("期望返回错误，但没有")
	}

	// 无效码率
	config.VideoCodec = CodecH264
	config.VideoBitrate = 0
	_, err = engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	if err == nil {
		t.Error("期望返回错误，但没有")
	}
}

func TestCreateTaskFromPreset(t *testing.T) {
	engine := NewEngine(2, 10)

	task, err := engine.CreateTaskFromPreset("/input.mp4", "/output.mp4", "1080p-标准", PriorityNormal)
	if err != nil {
		t.Fatalf("CreateTaskFromPreset 失败: %v", err)
	}

	if task.Preset != "1080p-标准" {
		t.Errorf("期望预设=1080p-标准, 实际=%s", task.Preset)
	}
	if task.Config.Resolution != Resolution1080p {
		t.Errorf("期望分辨率=%s, 实际=%s", Resolution1080p, task.Config.Resolution)
	}
}

func TestCreateTaskFromPresetNotFound(t *testing.T) {
	engine := NewEngine(2, 10)

	_, err := engine.CreateTaskFromPreset("/input.mp4", "/output.mp4", "不存在的预设", PriorityNormal)
	if err != ErrPresetNotFound {
		t.Errorf("期望 ErrPresetNotFound, 实际=%v", err)
	}
}

func TestGetTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	created, _ := engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)

	fetched, err := engine.GetTask(created.ID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("期望 ID=%s, 实际=%s", created.ID, fetched.ID)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	engine := NewEngine(2, 10)

	_, err := engine.GetTask("不存在")
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound, 实际=%v", err)
	}
}

func TestCancelTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	task, _ := engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)

	err := engine.CancelTask(task.ID)
	if err != nil {
		t.Fatalf("CancelTask 失败: %v", err)
	}

	fetched, _ := engine.GetTask(task.ID)
	if fetched.Status != TaskStatusCancelled {
		t.Errorf("期望状态=cancelled, 实际=%s", fetched.Status)
	}
	if fetched.Error != "用户取消" {
		t.Errorf("期望错误信息=用户取消, 实际=%s", fetched.Error)
	}
}

func TestCancelTaskNotFound(t *testing.T) {
	engine := NewEngine(2, 10)

	err := engine.CancelTask("不存在")
	if err != ErrTaskNotFound {
		t.Errorf("期望 ErrTaskNotFound, 实际=%v", err)
	}
}

func TestCancelCompletedTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	task, _ := engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)

	// 手动设置为已完成
	engine.mu.Lock()
	task.Status = TaskStatusCompleted
	engine.mu.Unlock()

	err := engine.CancelTask(task.ID)
	if err != ErrTaskNotCancellable {
		t.Errorf("期望 ErrTaskNotCancellable, 实际=%v", err)
	}
}

func TestListTasks(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input1.mp4", "/output1.mp4", config, PriorityNormal)
	engine.CreateTask("/input2.mp4", "/output2.mp4", config, PriorityHigh)

	tasks := engine.ListTasks("")
	if len(tasks) != 2 {
		t.Errorf("期望2个任务, 实际=%d", len(tasks))
	}

	// 测试状态过滤
	tasks = engine.ListTasks(TaskStatusPending)
	if len(tasks) != 2 {
		t.Errorf("期望2个pending任务, 实际=%d", len(tasks))
	}

	tasks = engine.ListTasks(TaskStatusCompleted)
	if len(tasks) != 0 {
		t.Errorf("期望0个completed任务, 实际=%d", len(tasks))
	}
}

func TestPriorityQueue(t *testing.T) {
	engine := NewEngine(1, 10)
	config := createTestConfig()

	// 创建不同优先级的任务
	engine.CreateTask("/low.mp4", "/out.mp4", config, PriorityLow)
	engine.CreateTask("/high.mp4", "/out.mp4", config, PriorityHigh)
	engine.CreateTask("/normal.mp4", "/out.mp4", config, PriorityNormal)

	// 应该按优先级顺序获取
	task1 := engine.GetNextTask()
	if task1.InputFile != "/high.mp4" {
		t.Errorf("期望第一个任务是高优先级, 实际=%s", task1.InputFile)
	}

	// 完成第一个任务
	engine.CompleteTask(task1.ID)

	task2 := engine.GetNextTask()
	if task2.InputFile != "/normal.mp4" {
		t.Errorf("期望第二个任务是普通优先级, 实际=%s", task2.InputFile)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	engine := NewEngine(2, 10) // 最大并发2
	config := createTestConfig()

	engine.CreateTask("/1.mp4", "/out.mp4", config, PriorityNormal)
	engine.CreateTask("/2.mp4", "/out.mp4", config, PriorityNormal)
	engine.CreateTask("/3.mp4", "/out.mp4", config, PriorityNormal)

	// 获取两个任务应该成功
	task1 := engine.GetNextTask()
	if task1 == nil {
		t.Fatal("期望获取到任务1")
	}
	task2 := engine.GetNextTask()
	if task2 == nil {
		t.Fatal("期望获取到任务2")
	}

	// 第三个应该返回nil（并发已满）
	task3 := engine.GetNextTask()
	if task3 != nil {
		t.Error("期望第三个任务为nil（并发限制）")
	}

	// 完成一个后应该能获取新的
	engine.CompleteTask(task1.ID)
	task3 = engine.GetNextTask()
	if task3 == nil {
		t.Error("完成一个任务后应该能获取新任务")
	}
}

func TestUpdateProgress(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)

	// 开始处理
	processing := engine.GetNextTask()

	err := engine.UpdateTaskProgress(processing.ID, 50.0)
	if err != nil {
		t.Fatalf("UpdateTaskProgress 失败: %v", err)
	}

	fetched, _ := engine.GetTask(processing.ID)
	if fetched.Progress != 50.0 {
		t.Errorf("期望进度=50.0, 实际=%f", fetched.Progress)
	}

	// 测试边界值
	engine.UpdateTaskProgress(processing.ID, -10)
	fetched, _ = engine.GetTask(processing.ID)
	if fetched.Progress != 0 {
		t.Errorf("期望进度=0（下限）, 实际=%f", fetched.Progress)
	}

	engine.UpdateTaskProgress(processing.ID, 110)
	fetched, _ = engine.GetTask(processing.ID)
	if fetched.Progress != 100 {
		t.Errorf("期望进度=100（上限）, 实际=%f", fetched.Progress)
	}
}

func TestCompleteTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()

	err := engine.CompleteTask(task.ID)
	if err != nil {
		t.Fatalf("CompleteTask 失败: %v", err)
	}

	fetched, _ := engine.GetTask(task.ID)
	if fetched.Status != TaskStatusCompleted {
		t.Errorf("期望状态=completed, 实际=%s", fetched.Status)
	}
	if fetched.Progress != 100 {
		t.Errorf("期望进度=100, 实际=%f", fetched.Progress)
	}
	if fetched.CompletedAt == nil {
		t.Error("CompletedAt 不应为 nil")
	}
}

func TestFailTask(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()

	err := engine.FailTask(task.ID, "磁盘空间不足")
	if err != nil {
		t.Fatalf("FailTask 失败: %v", err)
	}

	fetched, _ := engine.GetTask(task.ID)
	if fetched.Status != TaskStatusFailed {
		t.Errorf("期望状态=failed, 实际=%s", fetched.Status)
	}
	if fetched.Error != "磁盘空间不足" {
		t.Errorf("期望错误信息=磁盘空间不足, 实际=%s", fetched.Error)
	}
}

func TestAddPreset(t *testing.T) {
	engine := NewEngine(2, 10)

	preset := &TranscodePreset{
		Name:        "自定义预设",
		Description: "测试用自定义预设",
		Config: TranscodeConfig{
			Resolution:   Resolution720p,
			VideoCodec:   CodecH265,
			AudioCodec:   AudioCodecOpus,
			VideoBitrate: 3000,
			AudioBitrate: 128,
			FPS:          24,
		},
	}

	err := engine.AddPreset(preset)
	if err != nil {
		t.Fatalf("AddPreset 失败: %v", err)
	}

	fetched, err := engine.GetPreset("自定义预设")
	if err != nil {
		t.Fatalf("GetPreset 失败: %v", err)
	}
	if fetched.BuiltIn {
		t.Error("自定义预设不应标记为内置")
	}
}

func TestAddDuplicatePreset(t *testing.T) {
	engine := NewEngine(2, 10)

	preset := &TranscodePreset{
		Name:   "重复预设",
		Config: createTestConfig(),
	}

	engine.AddPreset(preset)

	err := engine.AddPreset(preset)
	if err != ErrDuplicatePreset {
		t.Errorf("期望 ErrDuplicatePreset, 实际=%v", err)
	}
}

func TestDeletePreset(t *testing.T) {
	engine := NewEngine(2, 10)

	preset := &TranscodePreset{
		Name:   "要删除的预设",
		Config: createTestConfig(),
	}
	engine.AddPreset(preset)

	err := engine.DeletePreset("要删除的预设")
	if err != nil {
		t.Fatalf("DeletePreset 失败: %v", err)
	}

	_, err = engine.GetPreset("要删除的预设")
	if err != ErrPresetNotFound {
		t.Errorf("期望 ErrPresetNotFound, 实际=%v", err)
	}
}

func TestDeleteBuiltInPreset(t *testing.T) {
	engine := NewEngine(2, 10)

	err := engine.DeletePreset("1080p-标准")
	if err == nil {
		t.Error("期望返回错误，但没有")
	}
}

func TestListPresets(t *testing.T) {
	engine := NewEngine(2, 10)

	presets := engine.ListPresets()
	if len(presets) < 5 {
		t.Errorf("期望至少5个预设, 实际=%d", len(presets))
	}

	// 验证排序
	for i := 1; i < len(presets); i++ {
		if presets[i].Name < presets[i-1].Name {
			t.Error("预设列表应按名称排序")
			break
		}
	}
}

func TestGenerateThumbnail(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()

	thumbnail, err := engine.GenerateThumbnail(task.ID, "/thumb.jpg", 30.0, 640, 480)
	if err != nil {
		t.Fatalf("GenerateThumbnail 失败: %v", err)
	}

	if thumbnail.TaskID != task.ID {
		t.Errorf("期望 TaskID=%s, 实际=%s", task.ID, thumbnail.TaskID)
	}
	if thumbnail.Timestamp != 30.0 {
		t.Errorf("期望 Timestamp=30.0, 实际=%f", thumbnail.Timestamp)
	}
	if thumbnail.Width != 640 || thumbnail.Height != 480 {
		t.Errorf("期望尺寸=640x480, 实际=%dx%d", thumbnail.Width, thumbnail.Height)
	}
}

func TestGenerateThumbnailDefaults(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()

	thumbnail, err := engine.GenerateThumbnail(task.ID, "/thumb.jpg", 10.0, 0, 0)
	if err != nil {
		t.Fatalf("GenerateThumbnail 失败: %v", err)
	}

	if thumbnail.Width != 320 || thumbnail.Height != 240 {
		t.Errorf("期望默认尺寸=320x240, 实际=%dx%d", thumbnail.Width, thumbnail.Height)
	}
}

func TestGetThumbnails(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()

	engine.GenerateThumbnail(task.ID, "/thumb1.jpg", 10.0, 320, 240)
	engine.GenerateThumbnail(task.ID, "/thumb2.jpg", 30.0, 320, 240)
	engine.GenerateThumbnail(task.ID, "/thumb3.jpg", 60.0, 320, 240)

	thumbnails, err := engine.GetThumbnails(task.ID)
	if err != nil {
		t.Fatalf("GetThumbnails 失败: %v", err)
	}

	if len(thumbnails) != 3 {
		t.Errorf("期望3个缩略图, 实际=%d", len(thumbnails))
	}
}

func TestGetStats(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	// 创建并完成一些任务
	engine.CreateTask("/1.mp4", "/out.mp4", config, PriorityNormal)
	processing1 := engine.GetNextTask()
	engine.CompleteTask(processing1.ID)

	engine.CreateTask("/2.mp4", "/out.mp4", config, PriorityNormal)
	processing2 := engine.GetNextTask()
	engine.FailTask(processing2.ID, "错误")

	t3, _ := engine.CreateTask("/3.mp4", "/out.mp4", config, PriorityNormal)
	engine.CancelTask(t3.ID)

	engine.CreateTask("/4.mp4", "/out.mp4", config, PriorityNormal)

	stats := engine.GetStats()

	if stats.TotalTasks != 4 {
		t.Errorf("期望总任务数=4, 实际=%d", stats.TotalTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("期望完成数=1, 实际=%d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("期望失败数=1, 实际=%d", stats.FailedTasks)
	}
	if stats.CancelledTasks != 1 {
		t.Errorf("期望取消数=1, 实际=%d", stats.CancelledTasks)
	}
	if stats.CompletionRate != 25.0 {
		t.Errorf("期望完成率=25.0, 实际=%f", stats.CompletionRate)
	}
}

func TestGetQueueStatus(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/1.mp4", "/out.mp4", config, PriorityNormal)
	engine.CreateTask("/2.mp4", "/out.mp4", config, PriorityNormal)

	status := engine.GetQueueStatus()

	if status["total"] != 2 {
		t.Errorf("期望 total=2, 实际=%d", status["total"])
	}
	if status["pending"] != 2 {
		t.Errorf("期望 pending=2, 实际=%d", status["pending"])
	}
	if status["max_concurrency"] != 2 {
		t.Errorf("期望 max_concurrency=2, 实际=%d", status["max_concurrency"])
	}
}

func TestEstimateFileSize(t *testing.T) {
	engine := NewEngine(2, 10)
	config := TranscodeConfig{
		VideoBitrate: 8000,
		AudioBitrate: 192,
	}

	// 1小时 = 3600秒
	size := engine.EstimateFileSize(config, 3600)
	if size <= 0 {
		t.Error("估算文件大小应大于0")
	}

	// 验证计算：(8000 + 192) * 3600 / 8 / 1024 ≈ 3590 MB
	expectedSize := (8000.0 + 192.0) * 3600.0 / 8.0 / 1024.0
	if size != expectedSize {
		t.Errorf("期望文件大小=%f, 实际=%f", expectedSize, size)
	}
}

func TestGetResolutionLabel(t *testing.T) {
	tests := []struct {
		resolution string
		expected   string
	}{
		{Resolution4K, "4K (2160p)"},
		{Resolution1080p, "Full HD (1080p)"},
		{Resolution720p, "HD (720p)"},
		{Resolution480p, "SD (480p)"},
		{"unknown", "unknown"},
	}

	for _, test := range tests {
		result := GetResolutionLabel(test.resolution)
		if result != test.expected {
			t.Errorf("GetResolutionLabel(%s) 期望=%s, 实际=%s", test.resolution, test.expected, result)
		}
	}
}

func TestGetCodecInfo(t *testing.T) {
	tests := []struct {
		codec    string
		expected string
	}{
		{CodecH264, "H.264/AVC - 通用兼容"},
		{CodecH265, "H.265/HEVC - 高压缩率"},
		{CodecVP9, "VP9 - 开源高效"},
		{CodecAV1, "AV1 - 最新一代编码"},
		{"unknown", "unknown"},
	}

	for _, test := range tests {
		result := GetCodecInfo(test.codec)
		if result != test.expected {
			t.Errorf("GetCodecInfo(%s) 期望=%s, 实际=%s", test.codec, test.expected, result)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30.0秒"},
		{5 * time.Minute, "5.0分钟"},
		{2 * time.Hour, "2.0小时"},
	}

	for _, test := range tests {
		result := FormatDuration(test.duration)
		if result != test.expected {
			t.Errorf("FormatDuration(%v) 期望=%s, 实际=%s", test.duration, test.expected, result)
		}
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := createTestConfig()
	err := ValidateConfig(validConfig)
	if err != nil {
		t.Errorf("有效配置不应返回错误: %v", err)
	}

	invalidConfig := TranscodeConfig{
		Resolution:   "invalid",
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		VideoBitrate: 8000,
		AudioBitrate: 192,
		FPS:          30,
	}
	err = ValidateConfig(invalidConfig)
	if err == nil {
		t.Error("无效配置应返回错误")
	}
}

func TestCodecSupported(t *testing.T) {
	tests := []struct {
		codec    string
		expected bool
	}{
		{CodecH264, true},
		{CodecH265, true},
		{CodecVP9, true},
		{CodecAV1, true},
		{AudioCodecAAC, true},
		{AudioCodecMP3, true},
		{"h264", true}, // 不区分大小写
		{"INVALID", false},
	}

	for _, test := range tests {
		result := CodecSupported(test.codec)
		if result != test.expected {
			t.Errorf("CodecSupported(%s) 期望=%v, 实际=%v", test.codec, test.expected, result)
		}
	}
}

func TestGetSupportedResolutions(t *testing.T) {
	resolutions := GetSupportedResolutions()
	if len(resolutions) != 4 {
		t.Errorf("期望4个分辨率, 实际=%d", len(resolutions))
	}
}

func TestGetSupportedVideoCodecs(t *testing.T) {
	codecs := GetSupportedVideoCodecs()
	if len(codecs) != 4 {
		t.Errorf("期望4个视频编码, 实际=%d", len(codecs))
	}
}

func TestGetSupportedAudioCodecs(t *testing.T) {
	codecs := GetSupportedAudioCodecs()
	if len(codecs) != 4 {
		t.Errorf("期望4个音频编码, 实际=%d", len(codecs))
	}
}

func TestMatchPreset(t *testing.T) {
	engine := NewEngine(2, 10)

	config := TranscodeConfig{
		Resolution:   Resolution1080p,
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		VideoBitrate: 8000,
		AudioBitrate: 192,
		FPS:          30,
	}

	presetName := engine.MatchPreset(config)
	if presetName != "1080p-标准" {
		t.Errorf("期望匹配预设=1080p-标准, 实际=%s", presetName)
	}

	// 不匹配任何预设的配置
	config2 := TranscodeConfig{
		Resolution:   Resolution4K,
		VideoCodec:   CodecH264,
		AudioCodec:   AudioCodecAAC,
		VideoBitrate: 8000,
		AudioBitrate: 192,
		FPS:          30,
	}

	presetName2 := engine.MatchPreset(config2)
	if presetName2 != "" {
		t.Errorf("期望无匹配预设, 实际=%s", presetName2)
	}
}

func TestCleanCompletedTasks(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	// 创建并完成一个任务
	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)
	task := engine.GetNextTask()
	engine.CompleteTask(task.ID)

	// 创建一个待处理任务
	engine.CreateTask("/input2.mp4", "/output2.mp4", config, PriorityNormal)

	// 清理1小时前完成的任务（不应该清理任何）
	removed := engine.CleanCompletedTasks(time.Now().Add(-time.Hour))
	if removed != 0 {
		t.Errorf("期望清理0个任务, 实际=%d", removed)
	}

	// 清理1分钟后完成的任务（应该清理刚才完成的任务）
	removed = engine.CleanCompletedTasks(time.Now().Add(time.Minute))
	if removed != 1 {
		t.Errorf("期望清理1个任务, 实际=%d", removed)
	}
}

func TestTaskString(t *testing.T) {
	config := createTestConfig()
	task := &TranscodeTask{
		ID:         "task_123",
		InputFile:  "/input.mp4",
		OutputFile: "/output.mp4",
		Status:     TaskStatusPending,
		Progress:   50.0,
		Config:     config,
	}

	str := task.String()
	if str == "" {
		t.Error("String() 不应返回空字符串")
	}
}

func TestPresetString(t *testing.T) {
	preset := &TranscodePreset{
		Name:        "测试预设",
		Description: "测试用",
		BuiltIn:     true,
	}

	str := preset.String()
	if str == "" {
		t.Error("String() 不应返回空字符串")
	}
}

func TestConfigString(t *testing.T) {
	config := createTestConfig()

	str := config.String()
	if str == "" {
		t.Error("String() 不应返回空字符串")
	}
}

func TestToJSON(t *testing.T) {
	config := createTestConfig()
	task := &TranscodeTask{
		ID:        "task_123",
		InputFile: "/input.mp4",
		Config:    config,
		Status:    TaskStatusPending,
		CreatedAt: time.Now(),
	}

	jsonStr, err := task.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}
	if jsonStr == "" {
		t.Error("ToJSON 不应返回空字符串")
	}
}

func TestPresetToJSON(t *testing.T) {
	preset := &TranscodePreset{
		Name:   "测试",
		Config: createTestConfig(),
	}

	jsonStr, err := preset.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 失败: %v", err)
	}
	if jsonStr == "" {
		t.Error("ToJSON 不应返回空字符串")
	}
}

func TestParseConfigFromJSON(t *testing.T) {
	configJSON := `{"resolution":"1920x1080","video_codec":"H264","audio_codec":"AAC","video_bitrate":8000,"audio_bitrate":192,"fps":30}`

	parsed, err := ParseConfigFromJSON(configJSON)
	if err != nil {
		t.Fatalf("ParseConfigFromJSON 失败: %v", err)
	}
	if parsed.Resolution != Resolution1080p {
		t.Errorf("期望分辨率=%s, 实际=%s", Resolution1080p, parsed.Resolution)
	}

	// 无效JSON
	_, err = ParseConfigFromJSON("invalid json")
	if err == nil {
		t.Error("期望返回错误，但没有")
	}
}

func TestGetStatsJSON(t *testing.T) {
	engine := NewEngine(2, 10)
	config := createTestConfig()

	engine.CreateTask("/input.mp4", "/output.mp4", config, PriorityNormal)

	jsonStr, err := engine.GetStatsJSON()
	if err != nil {
		t.Fatalf("GetStatsJSON 失败: %v", err)
	}
	if jsonStr == "" {
		t.Error("GetStatsJSON 不应返回空字符串")
	}
}

func TestQueueFull(t *testing.T) {
	engine := NewEngine(2, 2) // 队列大小2
	config := createTestConfig()

	_, err := engine.CreateTask("/1.mp4", "/out.mp4", config, PriorityNormal)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}
	_, err = engine.CreateTask("/2.mp4", "/out.mp4", config, PriorityNormal)
	if err != nil {
		t.Fatalf("第二次创建失败: %v", err)
	}
	_, err = engine.CreateTask("/3.mp4", "/out.mp4", config, PriorityNormal)
	if err != ErrQueueFull {
		t.Errorf("期望 ErrQueueFull, 实际=%v", err)
	}
}

func TestTaskGetDuration(t *testing.T) {
	now := time.Now()
	task := &TranscodeTask{
		StartedAt:   &now,
		CompletedAt: nil,
	}

	dur := GetTaskDuration(task)
	if dur < 0 {
		t.Error("耗时不应为负数")
	}

	// 测试已完成任务
	end := now.Add(5 * time.Second)
	task.CompletedAt = &end
	dur = GetTaskDuration(task)
	if dur != 5*time.Second {
		t.Errorf("期望耗时=5s, 实际=%v", dur)
	}

	// 测试未开始任务
	task2 := &TranscodeTask{}
	dur = GetTaskDuration(task2)
	if dur != 0 {
		t.Errorf("未开始任务耗时应为0, 实际=%v", dur)
	}
}

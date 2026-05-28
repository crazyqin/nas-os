package dataverifier

import (
	"testing"
)

func TestManager_CreateJob(t *testing.T) {
	m := NewManager()

	req := CreateJobRequest{
		Name:      "测试校验",
		Paths:     []string{"/data/docs", "/data/photos"},
		Algorithm: AlgorithmSHA256,
	}

	job, err := m.CreateJob(req)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if job.Name != "测试校验" {
		t.Errorf("期望名称 '测试校验', 得到 '%s'", job.Name)
	}
	if job.Status != JobStatusPending {
		t.Errorf("期望状态 pending, 得到 %s", job.Status)
	}
}

func TestManager_CreateJob_DefaultAlgorithm(t *testing.T) {
	m := NewManager()

	req := CreateJobRequest{
		Name:  "默认算法",
		Paths: []string{"/data"},
	}

	job, err := m.CreateJob(req)
	if err != nil {
		t.Fatalf("创建任务失败: %v", err)
	}
	if job.Algorithm != AlgorithmSHA256 {
		t.Errorf("期望默认算法 SHA256, 得到 %s", job.Algorithm)
	}
}

func TestManager_CreateJob_InvalidAlgorithm(t *testing.T) {
	m := NewManager()

	req := CreateJobRequest{
		Name:      "无效算法",
		Paths:     []string{"/data"},
		Algorithm: "invalid",
	}

	_, err := m.CreateJob(req)
	if err == nil {
		t.Error("期望错误，但没有")
	}
}

func TestManager_GetJob(t *testing.T) {
	m := NewManager()

	req := CreateJobRequest{
		Name:  "获取测试",
		Paths: []string{"/data"},
	}

	job, _ := m.CreateJob(req)

	got, err := m.GetJob(job.ID)
	if err != nil {
		t.Fatalf("获取任务失败: %v", err)
	}
	if got.ID != job.ID {
		t.Errorf("ID不匹配: %s vs %s", got.ID, job.ID)
	}
}

func TestManager_GetJob_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetJob("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("期望 ErrJobNotFound, 得到 %v", err)
	}
}

func TestManager_ListJobs(t *testing.T) {
	m := NewManager()

	m.CreateJob(CreateJobRequest{Name: "任务1", Paths: []string{"/a"}})
	m.CreateJob(CreateJobRequest{Name: "任务2", Paths: []string{"/b"}})

	jobs := m.ListJobs()
	if len(jobs) != 2 {
		t.Errorf("期望2个任务, 得到 %d", len(jobs))
	}
}

func TestManager_DeleteJob(t *testing.T) {
	m := NewManager()

	job, _ := m.CreateJob(CreateJobRequest{Name: "删除测试", Paths: []string{"/data"}})

	if err := m.DeleteJob(job.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	_, err := m.GetJob(job.ID)
	if err != ErrJobNotFound {
		t.Errorf("期望任务不存在, 得到 %v", err)
	}
}

func TestManager_DeleteJob_NotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteJob("nonexistent")
	if err != ErrJobNotFound {
		t.Errorf("期望 ErrJobNotFound, 得到 %v", err)
	}
}

func TestManager_StoreAndVerifyChecksum(t *testing.T) {
	m := NewManager()

	m.StoreChecksum("/data/test.txt", AlgorithmSHA256, "abc123", 1024)

	ok, err := m.VerifyChecksum("/data/test.txt", "abc123", AlgorithmSHA256)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if !ok {
		t.Error("校验和应该匹配")
	}

	ok, _ = m.VerifyChecksum("/data/test.txt", "wrong", AlgorithmSHA256)
	if ok {
		t.Error("校验和不应该匹配")
	}
}

func TestManager_VerifyChecksum_NotFound(t *testing.T) {
	m := NewManager()

	ok, err := m.VerifyChecksum("/nonexistent", "hash", AlgorithmSHA256)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if ok {
		t.Error("不存在的文件不应验证通过")
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.CreateJob(CreateJobRequest{Name: "统计测试1", Paths: []string{"/a"}})
	m.CreateJob(CreateJobRequest{Name: "统计测试2", Paths: []string{"/b"}})

	stats := m.GetStats()
	if stats.TotalJobs != 2 {
		t.Errorf("期望2个任务, 得到 %d", stats.TotalJobs)
	}
}

func TestComputeHash(t *testing.T) {
	data := []byte("hello world")

	hash := ComputeHash(data, AlgorithmSHA256)
	if hash == "" {
		t.Error("哈希值不应为空")
	}

	hash2 := ComputeHash(data, AlgorithmSHA256)
	if hash != hash2 {
		t.Error("相同输入应产生相同哈希")
	}

	crc := ComputeHash(data, AlgorithmCRC32)
	if crc == "" {
		t.Error("CRC32不应为空")
	}
}

func TestManager_RunJob(t *testing.T) {
	m := NewManager()

	job, _ := m.CreateJob(CreateJobRequest{Name: "运行测试", Paths: []string{"/data"}})

	result, err := m.RunJob(job.ID)
	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}
	if result.JobID != job.ID {
		t.Errorf("JobID不匹配: %s vs %s", result.JobID, job.ID)
	}

	// 检查状态
	got, _ := m.GetJob(job.ID)
	if got.Status != JobStatusCompleted {
		t.Errorf("期望 completed, 得到 %s", got.Status)
	}
}

func TestManager_RunJob_AlreadyRunning(t *testing.T) {
	m := NewManager()

	job, _ := m.CreateJob(CreateJobRequest{Name: "并发测试", Paths: []string{"/data"}})
	m.mu.Lock()
	job.Status = JobStatusRunning
	m.mu.Unlock()

	_, err := m.RunJob(job.ID)
	if err != ErrJobRunning {
		t.Errorf("期望 ErrJobRunning, 得到 %v", err)
	}
}

func TestManager_GetResult(t *testing.T) {
	m := NewManager()

	job, _ := m.CreateJob(CreateJobRequest{Name: "结果测试", Paths: []string{"/data"}})
	m.RunJob(job.ID)

	result, err := m.GetResult(job.ID)
	if err != nil {
		t.Fatalf("获取结果失败: %v", err)
	}
	if result.JobID != job.ID {
		t.Errorf("JobID不匹配")
	}
}

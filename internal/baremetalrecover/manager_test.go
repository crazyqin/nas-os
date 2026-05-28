package baremetalrecover

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestCreateImage(t *testing.T) {
	m := NewManager()

	// 创建全量备份
	img, err := m.CreateImage("/dev/sda", "系统备份-1", &BackupOptions{
		Type:    BackupTypeFull,
		Compress: true,
	})
	if err != nil {
		t.Fatalf("create image failed: %v", err)
	}
	if img.Name != "系统备份-1" {
		t.Errorf("expected '系统备份-1', got '%s'", img.Name)
	}
	if img.Type != BackupTypeFull {
		t.Errorf("expected full, got '%s'", img.Type)
	}
	if img.SourceDevice != "/dev/sda" {
		t.Errorf("expected '/dev/sda', got '%s'", img.SourceDevice)
	}
	if img.Checksum == "" {
		t.Error("expected non-empty checksum")
	}

	// 测试参数校验
	_, err = m.CreateImage("", "测试", nil)
	if err == nil {
		t.Error("expected error for empty device")
	}

	_, err = m.CreateImage("/dev/sda", "", nil)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestListImages(t *testing.T) {
	m := NewManager()

	m.CreateImage("/dev/sda", "备份1", nil)
	m.CreateImage("/dev/sdb", "备份2", nil)

	images := m.ListImages()
	if len(images) < 2 {
		t.Errorf("expected at least 2 images, got %d", len(images))
	}
}

func TestDeleteImage(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)

	err := m.DeleteImage(img.ID)
	if err != nil {
		t.Fatalf("delete image failed: %v", err)
	}

	images := m.ListImages()
	if len(images) != 0 {
		t.Errorf("expected 0 images after delete, got %d", len(images))
	}

	// 删除不存在的镜像
	err = m.DeleteImage("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent image")
	}
}

func TestDeleteImageWithChild(t *testing.T) {
	m := NewManager()

	parent, _ := m.CreateImage("/dev/sda", "父备份", nil)
	_, _ = m.CreateImage("/dev/sda", "增量备份", &BackupOptions{
		Type:          BackupTypeIncremental,
		ParentImageID: parent.ID,
	})

	// 删除有子镜像的父镜像应失败
	err := m.DeleteImage(parent.ID)
	if err == nil {
		t.Error("expected error when deleting parent image with children")
	}
}

func TestIncrementalBackup(t *testing.T) {
	m := NewManager()

	parent, _ := m.CreateImage("/dev/sda", "全量备份", nil)

	// 创建增量备份
	child, err := m.CreateImage("/dev/sda", "增量备份1", &BackupOptions{
		Type:          BackupTypeIncremental,
		ParentImageID: parent.ID,
	})
	if err != nil {
		t.Fatalf("create incremental backup failed: %v", err)
	}
	if child.Type != BackupTypeIncremental {
		t.Errorf("expected incremental, got '%s'", child.Type)
	}
	if child.ParentImageID != parent.ID {
		t.Errorf("expected parent '%s', got '%s'", parent.ID, child.ParentImageID)
	}

	// 没有父镜像的增量备份应失败
	_, err = m.CreateImage("/dev/sda", "无效增量", &BackupOptions{
		Type: BackupTypeIncremental,
	})
	if err == nil {
		t.Error("expected error for incremental backup without parent")
	}
}

func TestVerifyImage(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)

	valid, err := m.VerifyImage(img.ID)
	if err != nil {
		t.Fatalf("verify image failed: %v", err)
	}
	if !valid {
		t.Error("expected valid image")
	}

	// 验证不存在的镜像
	_, err = m.VerifyImage("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent image")
	}
}

func TestCreateRecoveryMedia(t *testing.T) {
	m := NewManager()

	// 创建USB介质
	usb, err := m.CreateRecoveryMedia("usb", "/dev/sdb1")
	if err != nil {
		t.Fatalf("create USB media failed: %v", err)
	}
	if usb.Type != MediaTypeUSB {
		t.Errorf("expected usb, got '%s'", usb.Type)
	}
	if usb.Path != "/dev/sdb1" {
		t.Errorf("expected '/dev/sdb1', got '%s'", usb.Path)
	}

	// 创建ISO介质
	iso, err := m.CreateRecoveryMedia("iso", "/tmp/recovery.iso")
	if err != nil {
		t.Fatalf("create ISO media failed: %v", err)
	}
	if iso.Type != MediaTypeISO {
		t.Errorf("expected iso, got '%s'", iso.Type)
	}

	// 无效类型
	_, err = m.CreateRecoveryMedia("dvd", "/tmp/test")
	if err == nil {
		t.Error("expected error for unsupported media type")
	}

	// 空路径
	_, err = m.CreateRecoveryMedia("usb", "")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestCreateAndExecutePlan(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)

	plan := &RecoveryPlan{
		Name:         "恢复计划1",
		Description:  "测试恢复计划",
		ImageIDs:     []string{img.ID},
		TargetDevice: "/dev/sdb",
		Steps: []RecoveryStep{
			{Sequence: 1, Type: "prepare", Description: "准备恢复环境"},
			{Sequence: 2, Type: "restore", Description: "恢复镜像"},
			{Sequence: 3, Type: "verify", Description: "验证恢复结果"},
		},
	}

	err := m.CreatePlan(plan)
	if err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	if plan.ID == "" {
		t.Error("expected non-empty plan ID")
	}

	// 执行计划
	job, err := m.ExecutePlan(plan.ID, "/dev/sdb")
	if err != nil {
		t.Fatalf("execute plan failed: %v", err)
	}
	if job.Status != JobStatusRunning {
		t.Errorf("expected running, got '%s'", job.Status)
	}
	if job.PlanID != plan.ID {
		t.Errorf("expected plan '%s', got '%s'", plan.ID, job.PlanID)
	}

	// 测试参数校验
	err = m.CreatePlan(nil)
	if err == nil {
		t.Error("expected error for nil plan")
	}

	_, err = m.ExecutePlan("nonexistent", "/dev/sdb")
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}

	_, err = m.ExecutePlan(plan.ID, "")
	if err == nil {
		t.Error("expected error for empty target device")
	}
}

func TestCancelJob(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)
	plan := &RecoveryPlan{Name: "计划1", ImageIDs: []string{img.ID}}
	m.CreatePlan(plan)

	job, _ := m.ExecutePlan(plan.ID, "/dev/sdb")

	err := m.CancelJob(job.ID)
	if err != nil {
		t.Fatalf("cancel job failed: %v", err)
	}

	status := m.GetJobStatus(job.ID)
	if status.Status != JobStatusCancelled {
		t.Errorf("expected cancelled, got '%s'", status.Status)
	}

	// 取消已完成的任务应失败（模拟）
	err = m.CancelJob("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestListJobs(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)
	plan := &RecoveryPlan{Name: "计划1", ImageIDs: []string{img.ID}}
	m.CreatePlan(plan)

	m.ExecutePlan(plan.ID, "/dev/sdb")

	jobs := m.ListJobs()
	if len(jobs) < 1 {
		t.Errorf("expected at least 1 job, got %d", len(jobs))
	}
}

func TestAddLocation(t *testing.T) {
	m := NewManager()

	loc := &BackupLocation{
		Name:     "本地备份",
		Type:     LocationTypeLocal,
		Path:     "/backup",
		Capacity: 1024 * 1024 * 1024 * 100, // 100GB
	}

	err := m.AddLocation(loc)
	if err != nil {
		t.Fatalf("add location failed: %v", err)
	}
	if loc.ID == "" {
		t.Error("expected non-empty location ID")
	}
	if !loc.Enabled {
		t.Error("expected enabled")
	}
	if loc.Available != loc.Capacity {
		t.Errorf("expected available = capacity, got %d", loc.Available)
	}

	// 无效类型
	err = m.AddLocation(&BackupLocation{Name: "test", Type: "ftp", Path: "/tmp"})
	if err == nil {
		t.Error("expected error for unsupported location type")
	}

	// 空名称
	err = m.AddLocation(&BackupLocation{Type: LocationTypeLocal, Path: "/tmp"})
	if err == nil {
		t.Error("expected error for empty name")
	}

	// 空路径
	err = m.AddLocation(&BackupLocation{Name: "test", Type: LocationTypeLocal})
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestListLocations(t *testing.T) {
	m := NewManager()

	m.AddLocation(&BackupLocation{Name: "本地", Type: LocationTypeLocal, Path: "/backup"})
	m.AddLocation(&BackupLocation{Name: "NAS", Type: LocationTypeNAS, Path: "//nas/backup"})

	locs := m.ListLocations()
	if len(locs) < 2 {
		t.Errorf("expected at least 2 locations, got %d", len(locs))
	}
}

func TestSetSchedule(t *testing.T) {
	m := NewManager()

	img, _ := m.CreateImage("/dev/sda", "备份1", nil)
	plan := &RecoveryPlan{Name: "计划1", ImageIDs: []string{img.ID}}
	m.CreatePlan(plan)

	sched := &BackupSchedule{
		Name:        "每日备份",
		Frequency:   "daily",
		RetainCount: 7,
	}

	err := m.SetSchedule(plan.ID, sched)
	if err != nil {
		t.Fatalf("set schedule failed: %v", err)
	}
	if sched.ID == "" {
		t.Error("expected non-empty schedule ID")
	}
	if sched.NextRunTime.IsZero() {
		t.Error("expected non-zero next run time")
	}

	// 获取调度
	got := m.GetSchedule(plan.ID)
	if got == nil {
		t.Fatal("expected schedule")
	}
	if got.Name != "每日备份" {
		t.Errorf("expected '每日备份', got '%s'", got.Name)
	}

	// 不存在的计划
	err = m.SetSchedule("nonexistent", sched)
	if err == nil {
		t.Error("expected error for nonexistent plan")
	}
}

func TestEncryptedBackup(t *testing.T) {
	m := NewManager()

	normal, _ := m.CreateImage("/dev/sda", "普通备份", nil)
	encrypted, _ := m.CreateImage("/dev/sda", "加密备份", &BackupOptions{
		Encrypt:       true,
		EncryptionKey: "secret-key-123",
	})

	if !encrypted.Encrypted {
		t.Error("expected encrypted")
	}
	if normal.Encrypted {
		t.Error("expected not encrypted")
	}
	// 加密镜像应更大（有加密头）
	if encrypted.Size <= normal.Size {
		t.Error("expected encrypted image to be larger")
	}
}

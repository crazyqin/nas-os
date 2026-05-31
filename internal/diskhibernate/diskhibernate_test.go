package diskhibernate

import (
	"testing"
	"time"
)

func TestRegisterDisk(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sda", "WD Red", "WD123456", 1024*1024*1024*1024)

	if disk.Device != "/dev/sda" {
		t.Errorf("期望设备 /dev/sda, 得到 %s", disk.Device)
	}

	if disk.State != StateActive {
		t.Errorf("期望状态 active, 得到 %s", disk.State)
	}
}

func TestRecordAccess(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sdb", "Seagate", "ST789012", 2*1024*1024*1024*1024)

	// 记录访问
	m.RecordAccess(disk.ID, 4096, "read")
	m.RecordAccess(disk.ID, 8192, "write")

	d, _ := m.GetDisk(disk.ID)
	if d.AccessCount != 2 {
		t.Errorf("期望访问次数 2, 得到 %d", d.AccessCount)
	}

	if d.TotalIO != 12288 {
		t.Errorf("期望总IO 12288, 得到 %d", d.TotalIO)
	}
}

func TestAccessPattern(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sdc", "Toshiba", "TOS345678", 512*1024*1024*1024)

	// 模拟大量访问
	for i := 0; i < 200; i++ {
		m.RecordAccess(disk.ID, 4096, "read")
	}

	pattern, err := m.GetPattern(disk.ID)
	if err != nil {
		t.Fatalf("获取模式失败: %v", err)
	}

	if pattern.TotalRecords != 200 {
		t.Errorf("期望总记录 200, 得到 %d", pattern.TotalRecords)
	}
}

func TestHibernateCheck(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sdd", "HGST", "HST901234", 4*1024*1024*1024*1024)

	// 设置最后访问时间为2小时前
	m.mu.Lock()
	disk.LastAccess = time.Now().Add(-2 * time.Hour)
	m.mu.Unlock()

	should, reason := m.CheckHibernate(disk.ID)
	if !should {
		t.Log("不应该休眠:", reason)
	}
	t.Logf("休眠检查结果: %v, 原因: %s", should, reason)
}

func TestTemperatureHibernate(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sde", "Samsung", "SAM567890", 1024*1024*1024*1024)

	// 设置高温
	m.mu.Lock()
	disk.Temperature = 60 // 超过默认阈值55°C
	m.mu.Unlock()

	should, reason := m.CheckHibernate(disk.ID)
	if !should {
		t.Error("高温时应该建议休眠")
	}
	t.Logf("休眠原因: %s", reason)
}

func TestWakeDisk(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sdf", "WD Blue", "WDB456789", 256*1024*1024*1024)

	// 先休眠
	m.HibernateDisk(disk.ID, StateSleep)

	d, _ := m.GetDisk(disk.ID)
	if d.State != StateSleep {
		t.Errorf("期望状态 sleep, 得到 %s", d.State)
	}

	// 唤醒
	m.WakeDisk(disk.ID)

	d2, _ := m.GetDisk(disk.ID)
	if d2.State != StateActive {
		t.Errorf("期望状态 active, 得到 %s", d2.State)
	}
}

func TestListDisks(t *testing.T) {
	m := NewManager()

	m.RegisterDisk("/dev/sda", "Disk1", "SER1", 1024)
	m.RegisterDisk("/dev/sdb", "Disk2", "SER2", 2048)

	disks := m.ListDisks()
	if len(disks) != 2 {
		t.Errorf("期望2个磁盘, 得到 %d", len(disks))
	}
}

func TestHibernateReport(t *testing.T) {
	m := NewManager()

	disk := m.RegisterDisk("/dev/sdg", "TestDisk", "TEST123", 1024)
	m.HibernateDisk(disk.ID, StateSleep)

	report := m.GetHibernateReport()
	totalDisks := report["total_disks"].(int)
	if totalDisks != 1 {
		t.Errorf("期望1个磁盘, 得到 %d", totalDisks)
	}

	hibernated := report["hibernated_count"].(int)
	if hibernated != 1 {
		t.Errorf("期望1个休眠, 得到 %d", hibernated)
	}
}

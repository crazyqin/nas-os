package websharepro

import (
	"testing"
)

func TestNewUnifiedFileSystem(t *testing.T) {
	fs := NewUnifiedFileSystem()
	if fs == nil {
		t.Fatal("NewUnifiedFileSystem returned nil")
	}
}

func TestGetSupportedProtocols(t *testing.T) {
	fs := NewUnifiedFileSystem()

	protocols := fs.GetSupportedProtocols()
	if len(protocols) < 4 {
		t.Errorf("expected at least 4 protocols, got %d", len(protocols))
	}

	// 检查必须的协议
	protocolMap := make(map[ProtocolType]bool)
	for _, p := range protocols {
		protocolMap[p] = true
	}

	required := []ProtocolType{ProtocolLocal, ProtocolSMB, ProtocolNFS, ProtocolWebDAV}
	for _, p := range required {
		if !protocolMap[p] {
			t.Errorf("missing required protocol: %s", p)
		}
	}
}

func TestRegisterAdapter(t *testing.T) {
	fs := NewUnifiedFileSystem()

	// 注册自定义适配器
	custom := &LocalAdapter{}
	fs.RegisterAdapter("custom", custom)

	protocols := fs.GetSupportedProtocols()
	found := false
	for _, p := range protocols {
		if p == "custom" {
			found = true
		}
	}
	if !found {
		t.Error("expected custom protocol to be registered")
	}
}

func TestMountUnmount(t *testing.T) {
	fs := NewUnifiedFileSystem()

	config := &ProtocolConfig{
		Type:     ProtocolLocal,
		Endpoint: "/data/share",
		Timeout:  10,
	}

	// Mount
	err := fs.Mount("/mnt/share", config)
	if err != nil {
		t.Fatalf("mount failed: %v", err)
	}

	mounts := fs.ListMounts()
	if len(mounts) != 1 {
		t.Errorf("expected 1 mount, got %d", len(mounts))
	}

	if mounts[0].Path != "/mnt/share" {
		t.Errorf("expected mount path /mnt/share, got %s", mounts[0].Path)
	}

	// Unmount
	err = fs.Unmount("/mnt/share")
	if err != nil {
		t.Fatalf("unmount failed: %v", err)
	}

	mounts = fs.ListMounts()
	if len(mounts) != 0 {
		t.Errorf("expected 0 mounts, got %d", len(mounts))
	}
}

func TestMountUnsupportedProtocol(t *testing.T) {
	fs := NewUnifiedFileSystem()

	config := &ProtocolConfig{
		Type:     "ftp",
		Endpoint: "/data",
		Timeout:  10,
	}

	err := fs.Mount("/mnt/ftp", config)
	if err == nil {
		t.Error("expected error for unsupported protocol")
	}
}

func TestUnmountNonExistent(t *testing.T) {
	fs := NewUnifiedFileSystem()

	err := fs.Unmount("/nonexistent")
	if err == nil {
		t.Error("expected error for non-existent mount")
	}
}

func TestResolvePath(t *testing.T) {
	fs := NewUnifiedFileSystem()

	// 注册测试适配器
	testAdapter := &LocalAdapter{}
	fs.RegisterAdapter(ProtocolLocal, testAdapter)

	config := &ProtocolConfig{
		Type:     ProtocolLocal,
		Endpoint: "/remote/data",
		Timeout:  10,
	}
	fs.Mount("/mnt/local", config)

	// 测试路径解析
	adapter, path, protocol := fs.resolvePath("/mnt/local/subdir/file.txt")
	if adapter == nil {
		t.Fatal("expected adapter")
	}
	if protocol != ProtocolLocal {
		t.Errorf("expected local protocol, got %s", protocol)
	}
	if path != "/remote/data/subdir/file.txt" {
		t.Errorf("expected /remote/data/subdir/file.txt, got %s", path)
	}
}

func TestTransferTaskTracking(t *testing.T) {
	fs := NewUnifiedFileSystem()

	// 创建一个模拟传输任务
	task := &TransferTask{
		ID:      "test-transfer",
		SrcPath: "/src",
		DstPath: "/dst",
		Size:    1024,
		Status:  "running",
	}

	fs.mu.Lock()
	fs.transfers["test-transfer"] = task
	fs.mu.Unlock()

	got, exists := fs.GetTransfer("test-transfer")
	if !exists {
		t.Fatal("expected transfer to exist")
	}
	if got.ID != "test-transfer" {
		t.Errorf("expected ID test-transfer, got %s", got.ID)
	}

	// 不存在的传输
	_, exists = fs.GetTransfer("nonexistent")
	if exists {
		t.Error("expected transfer to not exist")
	}
}

func TestProtocolConfigDefaults(t *testing.T) {
	config := &ProtocolConfig{
		Type:     ProtocolSMB,
		Endpoint: "192.168.1.100",
		Username: "admin",
		Password: "secret",
		Domain:   "WORKGROUP",
		Timeout:  30,
		MaxConns: 10,
	}

	if config.Type != ProtocolSMB {
		t.Errorf("expected SMB, got %s", config.Type)
	}
	if config.MaxConns != 10 {
		t.Errorf("expected 10 max connections, got %d", config.MaxConns)
	}
}

func TestFileInfoStructure(t *testing.T) {
	info := &FileInfo{
		Path:        "/data/test.txt",
		Name:        "test.txt",
		Size:        1024,
		IsDir:       false,
		Mode:        0644,
		ContentType: "text/plain",
		Protocol:    ProtocolLocal,
		Owner:       "root",
		Group:       "root",
	}

	if info.Name != "test.txt" {
		t.Errorf("expected test.txt, got %s", info.Name)
	}
	if info.IsDir {
		t.Error("expected file, not directory")
	}
}

func TestProtocolStatuses(t *testing.T) {
	statuses := []ProtocolStatus{
		StatusActive,
		StatusInactive,
		StatusError,
		StatusConnecting,
	}

	for _, s := range statuses {
		if s == "" {
			t.Error("expected non-empty status")
		}
	}
}

func TestLocalAdapterInterface(t *testing.T) {
	var adapter ProtocolAdapter = &LocalAdapter{}

	if adapter.GetStatus() != StatusInactive {
		t.Error("expected inactive status initially")
	}
}

func TestProgressReader(t *testing.T) {
	data := []byte("test data for progress tracking")
	reader := &progressReader{
		reader: &simpleReader{data: data},
		onRead: func(n int) {
			// 回调被调用即可
		},
	}

	buf := make([]byte, 1024)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes, got %d", len(data), n)
	}
}

// simpleReader 简单的 reader 实现
type simpleReader struct {
	data   []byte
	offset int
}

func (r *simpleReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, nil
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

package usbsmartmount

import (
	"os"
	"path/filepath"
	"testing"
)

// ========== 辅助函数 ==========

func createTestManager(t *testing.T) *USBMountManager {
	t.Helper()
	m := NewManager()
	// 使用临时目录避免冲突
	m.historyFile = filepath.Join(t.TempDir(), "test_history.json")
	m.history = make([]HistoryEntry, 0)
	return m
}

func boolPtr(b bool) *bool { return &b }

// ========== NewManager 测试 ==========

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.mountBase == "" {
		t.Fatal("mountBase should not be empty")
	}
	if m.defaultShare == "" {
		t.Fatal("defaultShare should not be empty")
	}
}

// ========== DetectDevice 测试 ==========

func TestDetectDevice_Camera(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "04b0", // Nikon
		ProductID:  "0123",
		DeviceName: "Nikon D750",
		CapacityGB: 64,
		FileSystem: "fat32",
		IsRemovable: true,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "camera" {
		t.Errorf("expected camera, got %s", result.DeviceType)
	}
	if result.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", result.Confidence)
	}
	if !result.AutoMount {
		t.Error("camera should auto mount")
	}
	if result.RecommendedAction != "backup" {
		t.Errorf("expected backup action, got %s", result.RecommendedAction)
	}
}

func TestDetectDevice_Phone(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "18d1", // Google/Android
		ProductID:  "4ee1",
		DeviceName: "Pixel 8",
		CapacityGB: 128,
		FileSystem: "mtp",
		IsRemovable: true,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "phone" {
		t.Errorf("expected phone, got %s", result.DeviceType)
	}
	if result.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", result.Confidence)
	}
	if !result.AutoMount {
		t.Error("phone should auto mount")
	}
}

func TestDetectDevice_USBDrive(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0781", // SanDisk
		ProductID:  "5581",
		DeviceName: "SanDisk Ultra Fit",
		CapacityGB: 64,
		FileSystem: "exfat",
		IsRemovable: true,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "usb_drive" {
		t.Errorf("expected usb_drive, got %s", result.DeviceType)
	}
	if result.Confidence < 0.7 {
		t.Errorf("expected confidence >= 0.7, got %f", result.Confidence)
	}
}

func TestDetectDevice_ExternalHDD(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0bc9", // Generic
		ProductID:  "231a",
		DeviceName: "WD My Passport",
		CapacityGB: 2000,
		FileSystem: "ntfs",
		IsRemovable: true,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "external_hdd" {
		t.Errorf("expected external_hdd, got %s", result.DeviceType)
	}
	if result.Confidence < 0.75 {
		t.Errorf("expected confidence >= 0.75, got %f", result.Confidence)
	}
}

func TestDetectDevice_CardReader(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "058f", // Alcor
		ProductID:  "6366",
		DeviceName: "Card Reader",
		CapacityGB: 2,
		FileSystem: "fat32",
		IsRemovable: true,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "card_reader" {
		t.Errorf("expected card_reader, got %s", result.DeviceType)
	}
}

func TestDetectDevice_InsufficientInfo(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{} // 全空

	_, err := m.DetectDevice(device)
	if err == nil {
		t.Error("expected error for insufficient device info")
	}
}

func TestDetectDevice_UnknownDevice(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "abcd",
		ProductID:  "1234",
		DeviceName: "Unknown Gadget",
		CapacityGB: 0,
		FileSystem: "",
		IsRemovable: false,
		BusType:    "USB",
	}

	result, err := m.DetectDevice(device)
	if err != nil {
		t.Fatalf("DetectDevice failed: %v", err)
	}
	if result.DeviceType != "unknown" && result.DeviceType != "usb_drive" {
		t.Errorf("expected unknown or usb_drive for unrecognized device, got %s", result.DeviceType)
	}
}

// ========== RecommendMount 测试 ==========

func TestRecommendMount_Camera(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "04b0",
		ProductID:  "0123",
		DeviceName: "Nikon D750",
		CapacityGB: 64,
		FileSystem: "fat32",
		IsRemovable: true,
		BusType:    "USB",
	}

	rec, err := m.RecommendMount(device)
	if err != nil {
		t.Fatalf("RecommendMount failed: %v", err)
	}
	if !rec.ReadOnly {
		t.Error("camera should be mounted read-only")
	}
	if rec.MountPoint == "" {
		t.Error("mount point should not be empty")
	}
	if rec.Reason == "" {
		t.Error("reason should not be empty")
	}
	// 检查包含 ro 选项
	found := false
	for _, opt := range rec.Options {
		if opt == "ro" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'ro' in mount options for camera")
	}
}

func TestRecommendMount_ExternalHDD_NTFS(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0bc9",
		ProductID:  "231a",
		DeviceName: "WD My Passport",
		CapacityGB: 2000,
		FileSystem: "ntfs",
		IsRemovable: true,
		BusType:    "USB",
	}

	rec, err := m.RecommendMount(device)
	if err != nil {
		t.Fatalf("RecommendMount failed: %v", err)
	}
	if rec.ReadOnly {
		t.Error("external HDD should not be read-only")
	}
	// NTFS 应该有 errors=remount-ro
	found := false
	for _, opt := range rec.Options {
		if opt == "errors=remount-ro" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'errors=remount-ro' for NTFS")
	}
}

func TestRecommendMount_EmptyFileSystem(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0781",
		ProductID:  "5581",
		DeviceName: "SanDisk",
		CapacityGB: 32,
		FileSystem: "",
		IsRemovable: true,
		BusType:    "USB",
	}

	_, err := m.RecommendMount(device)
	if err == nil {
		t.Error("expected error for empty file system")
	}
}

func TestRecommendMount_MountPointUnique(t *testing.T) {
	m := createTestManager(t)

	device1 := USBDeviceInfo{
		VendorID:   "0781",
		ProductID:  "5581",
		DeviceName: "SanDisk",
		CapacityGB: 32,
		FileSystem: "exfat",
		IsRemovable: true,
		BusType:    "USB",
	}
	device2 := USBDeviceInfo{
		VendorID:   "04e8",
		ProductID:  "6024",
		DeviceName: "Samsung T7",
		CapacityGB: 1000,
		FileSystem: "exfat",
		IsRemovable: true,
		BusType:    "USB",
	}

	rec1, _ := m.RecommendMount(device1)
	rec2, _ := m.RecommendMount(device2)
	if rec1.MountPoint == rec2.MountPoint {
		t.Error("mount points should be unique for different devices")
	}
}

// ========== SuggestBackup 测试 ==========

func TestSuggestBackup_Camera(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "04b0",
		ProductID:  "0123",
		DeviceName: "Nikon D750",
		CapacityGB: 64,
		FileSystem: "fat32",
		IsRemovable: true,
		BusType:    "USB",
	}

	suggestion, err := m.SuggestBackup(device)
	if err != nil {
		t.Fatalf("SuggestBackup failed: %v", err)
	}
	if !suggestion.ShouldBackup {
		t.Error("camera should backup")
	}
	if suggestion.Strategy != "photo_import" {
		t.Errorf("expected photo_import strategy, got %s", suggestion.Strategy)
	}
	if !suggestion.Deduplicate {
		t.Error("photo import should deduplicate")
	}
	if suggestion.SizeGB <= 0 {
		t.Error("size should be positive")
	}
	if suggestion.EstimatedDuration == "unknown" {
		t.Error("estimated duration should be calculated")
	}
}

func TestSuggestBackup_Phone(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "18d1",
		ProductID:  "4ee1",
		DeviceName: "Pixel 8",
		CapacityGB: 128,
		FileSystem: "mtp",
		IsRemovable: true,
		BusType:    "USB",
	}

	suggestion, err := m.SuggestBackup(device)
	if err != nil {
		t.Fatalf("SuggestBackup failed: %v", err)
	}
	if !suggestion.ShouldBackup {
		t.Error("phone should backup")
	}
	if suggestion.Strategy != "incremental" {
		t.Errorf("expected incremental strategy, got %s", suggestion.Strategy)
	}
	if !suggestion.Encryption {
		t.Error("phone backup should be encrypted")
	}
}

func TestSuggestBackup_ExternalHDD(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0bc9",
		ProductID:  "231a",
		DeviceName: "WD My Passport",
		CapacityGB: 2000,
		FileSystem: "ntfs",
		IsRemovable: true,
		BusType:    "USB",
	}

	suggestion, err := m.SuggestBackup(device)
	if err != nil {
		t.Fatalf("SuggestBackup failed: %v", err)
	}
	if !suggestion.ShouldBackup {
		t.Error("external HDD should backup")
	}
	if suggestion.Strategy != "mirror" {
		t.Errorf("expected mirror strategy, got %s", suggestion.Strategy)
	}
	if suggestion.Deduplicate {
		t.Error("mirror backup should not deduplicate")
	}
}

func TestSuggestBackup_Unknown(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "ffff",
		ProductID:  "ffff",
		DeviceName: "Mystery Device",
		CapacityGB: 0,
		FileSystem: "",
		IsRemovable: false,
		BusType:    "USB",
	}

	suggestion, err := m.SuggestBackup(device)
	if err != nil {
		t.Fatalf("SuggestBackup failed: %v", err)
	}
	if suggestion.ShouldBackup {
		t.Error("unknown device should not backup")
	}
	if suggestion.Strategy != "none" {
		t.Errorf("expected none strategy, got %s", suggestion.Strategy)
	}
}

// ========== SafeEject 测试 ==========

func TestSafeEject_EmptyPath(t *testing.T) {
	m := createTestManager(t)

	_, err := m.SafeEject("")
	if err == nil {
		t.Error("expected error for empty mount path")
	}
}

func TestSafeEject_NonExistentPath(t *testing.T) {
	m := createTestManager(t)

	result, err := m.SafeEject("/nonexistent/path/12345")
	if err != nil {
		t.Fatalf("SafeEject failed: %v", err)
	}
	if result.Success {
		t.Error("should not succeed for non-existent path")
	}
	if result.Warning == "" {
		t.Error("should have warning for non-existent path")
	}
}

func TestSafeEject_ValidPath(t *testing.T) {
	m := createTestManager(t)
	// 创建临时目录模拟挂载点
	mountPath := t.TempDir()

	result, err := m.SafeEject(mountPath)
	if err != nil {
		t.Fatalf("SafeEject failed: %v", err)
	}
	if !result.Success {
		t.Error("should succeed for valid path")
	}
	if !result.SafeToRemove {
		t.Error("should be safe to remove after successful eject")
	}
	if result.PendingIO {
		t.Error("should not have pending I/O for test directory")
	}
}

func TestSafeEject_PendingIO(t *testing.T) {
	m := createTestManager(t)
	mountPath := t.TempDir()
	// 创建标记文件模拟 pending I/O
	pendingFile := filepath.Join(mountPath, ".pending_write")
	if err := os.WriteFile(pendingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create pending file: %v", err)
	}

	result, err := m.SafeEject(mountPath)
	if err != nil {
		t.Fatalf("SafeEject failed: %v", err)
	}
	if result.PendingIO {
		// pending I/O should prevent success
		if result.Success {
			t.Error("should not succeed with pending I/O")
		}
	}
}

// ========== RecordHistory 测试 ==========

func TestRecordHistory_NewDevice(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:     "0781",
		ProductID:    "5581",
		SerialNumber: "SN12345678",
		DeviceName:   "SanDisk Ultra",
		CapacityGB:   64,
		FileSystem:   "exfat",
		IsRemovable:  true,
		BusType:      "USB",
	}

	entry, err := m.RecordHistory(device)
	if err != nil {
		t.Fatalf("RecordHistory failed: %v", err)
	}
	if entry.SerialNumber != "SN12345678" {
		t.Errorf("expected SN12345678, got %s", entry.SerialNumber)
	}
	if entry.MountCount != 1 {
		t.Errorf("expected mount count 1, got %d", entry.MountCount)
	}
	if entry.FirstSeen == 0 {
		t.Error("FirstSeen should not be zero")
	}
	if entry.LastSeen == 0 {
		t.Error("LastSeen should not be zero")
	}
}

func TestRecordHistory_ExistingDevice(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:     "0781",
		ProductID:    "5581",
		SerialNumber: "SN12345678",
		DeviceName:   "SanDisk Ultra",
		CapacityGB:   64,
		FileSystem:   "exfat",
		IsRemovable:  true,
		BusType:      "USB",
	}

	// 第一次记录
	entry1, _ := m.RecordHistory(device)
	// 第二次记录同一个设备
	entry2, _ := m.RecordHistory(device)
	if entry2.MountCount != 2 {
		t.Errorf("expected mount count 2, got %d", entry2.MountCount)
	}
	if entry1.FirstSeen != entry2.FirstSeen {
		t.Error("FirstSeen should not change on subsequent records")
	}
	if entry2.LastSeen < entry1.FirstSeen {
		t.Error("LastSeen should be >= FirstSeen")
	}
	history, _ := m.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestRecordHistory_NoSerial(t *testing.T) {
	m := createTestManager(t)

	device := USBDeviceInfo{
		VendorID:   "0781",
		ProductID:  "5581",
		DeviceName: "SanDisk Ultra",
		CapacityGB: 64,
	}

	_, err := m.RecordHistory(device)
	if err == nil {
		t.Error("expected error for missing serial number")
	}
}

// ========== GetHistory 测试 ==========

func TestGetHistory_Empty(t *testing.T) {
	m := createTestManager(t)

	history, err := m.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d entries", len(history))
	}
}

func TestGetHistory_MultipleDevices(t *testing.T) {
	m := createTestManager(t)

	devices := []USBDeviceInfo{
		{SerialNumber: "SN001", DeviceName: "Device1"},
		{SerialNumber: "SN002", DeviceName: "Device2"},
		{SerialNumber: "SN003", DeviceName: "Device3"},
	}

	for _, d := range devices {
		_, err := m.RecordHistory(d)
		if err != nil {
			t.Fatalf("RecordHistory failed: %v", err)
		}
	}

	history, err := m.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Errorf("expected 3 entries, got %d", len(history))
	}

	// 确保返回的是副本而非内部引用
	history[0].DeviceName = "ModifiedInExternal"
	internal, _ := m.GetHistory()
	if internal[0].DeviceName == "ModifiedInExternal" {
		t.Error("GetHistory should return a copy, not internal reference")
	}
}
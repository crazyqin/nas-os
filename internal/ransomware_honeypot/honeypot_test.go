package ransomware_honeypot

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ===== 蜜罐管理器测试 =====

func TestHoneypotManager_Create(t *testing.T) {
	mgr := NewHoneypotManager()

	hp, err := mgr.Create(CreateHoneypotRequest{
		Name:      "测试蜜罐",
		SharePath: "/shares/honeypot",
	})
	if err != nil {
		t.Fatalf("创建蜜罐失败: %v", err)
	}
	if hp.Name != "测试蜜罐" {
		t.Errorf("名称不匹配: got %s", hp.Name)
	}
	if hp.State != StateActive {
		t.Errorf("状态应为 active, got %s", hp.State)
	}
	if hp.FileCount == 0 {
		t.Error("诱饵文件数量不应为 0")
	}
}

func TestHoneypotManager_Create_EmptyName(t *testing.T) {
	mgr := NewHoneypotManager()
	_, err := mgr.Create(CreateHoneypotRequest{
		SharePath: "/shares/honeypot",
	})
	if err == nil {
		t.Error("空名称应返回错误")
	}
}

func TestHoneypotManager_Create_EmptyPath(t *testing.T) {
	mgr := NewHoneypotManager()
	_, err := mgr.Create(CreateHoneypotRequest{
		Name: "test",
	})
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

func TestHoneypotManager_Create_DuplicateName(t *testing.T) {
	mgr := NewHoneypotManager()
	mgr.Create(CreateHoneypotRequest{
		Name:      "dup",
		SharePath: "/shares/honeypot",
	})
	_, err := mgr.Create(CreateHoneypotRequest{
		Name:      "dup",
		SharePath: "/shares/honeypot2",
	})
	if err == nil {
		t.Error("重复名称应返回错误")
	}
}

func TestHoneypotManager_Get(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "get-test",
		SharePath: "/shares/honeypot",
	})

	got, err := mgr.Get(hp.ID)
	if err != nil {
		t.Fatalf("获取蜜罐失败: %v", err)
	}
	if got.ID != hp.ID {
		t.Errorf("ID 不匹配")
	}
}

func TestHoneypotManager_Get_NotFound(t *testing.T) {
	mgr := NewHoneypotManager()
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("获取不存在的蜜罐应返回错误")
	}
}

func TestHoneypotManager_List(t *testing.T) {
	mgr := NewHoneypotManager()
	mgr.Create(CreateHoneypotRequest{Name: "hp1", SharePath: "/shares/hp1"})
	mgr.Create(CreateHoneypotRequest{Name: "hp2", SharePath: "/shares/hp2"})

	list := mgr.List()
	if len(list) != 2 {
		t.Errorf("期望 2 个蜜罐, got %d", len(list))
	}
}

func TestHoneypotManager_Delete(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "del-test",
		SharePath: "/shares/honeypot",
	})

	err := mgr.Delete(hp.ID)
	if err != nil {
		t.Fatalf("删除蜜罐失败: %v", err)
	}

	_, err = mgr.Get(hp.ID)
	if err == nil {
		t.Error("已删除蜜罐不应存在")
	}
}

func TestHoneypotManager_Delete_NotFound(t *testing.T) {
	mgr := NewHoneypotManager()
	err := mgr.Delete("nonexistent")
	if err == nil {
		t.Error("删除不存在的蜜罐应返回错误")
	}
}

// ===== 扫描测试 =====

func TestHoneypotManager_Scan(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "scan-test",
		SharePath: "/shares/honeypot",
	})

	result, err := mgr.Scan(hp.ID)
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	if result.FilesScanned == 0 {
		t.Error("扫描文件数不应为 0")
	}
	if result.HoneypotID != hp.ID {
		t.Error("蜜罐 ID 不匹配")
	}
}

func TestHoneypotManager_Scan_NotFound(t *testing.T) {
	mgr := NewHoneypotManager()
	_, err := mgr.Scan("nonexistent")
	if err == nil {
		t.Error("扫描不存在的蜜罐应返回错误")
	}
}

// ===== 告警测试 =====

func TestHoneypotManager_Alerts(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "alert-test",
		SharePath: "/shares/honeypot",
	})

	alerts := mgr.GetAlerts(hp.ID)
	if alerts == nil {
		alerts = []*Alert{}
	}
	// 初始无告警
	if len(alerts) != 0 {
		t.Errorf("初始告警应为 0, got %d", len(alerts))
	}
}

func TestHoneypotManager_RespondAlert(t *testing.T) {
	mgr := NewHoneypotManager()

	// 响应不存在的告警
	err := mgr.RespondAlert("nonexistent", AlertResponse{
		Action:  ActionIsolate,
		Comment: "测试",
	})
	if err == nil {
		t.Error("响应不存在的告警应返回错误")
	}
}

// ===== 检测器测试 =====

func TestDetector_CheckEntropyChange(t *testing.T) {
	det := NewDetector(DefaultThresholds())

	// 正常文件
	normal := &DecoyFile{
		FileType: FileTypeText,
		Entropy:  5.0,
	}
	if det.CheckEntropyChange(normal) {
		t.Error("正常熵值不应触发告警")
	}

	// 异常高熵值（被加密）
	encrypted := &DecoyFile{
		FileType: FileTypeText,
		Entropy:  7.9,
	}
	if !det.CheckEntropyChange(encrypted) {
		t.Error("异常高熵值应触发告警")
	}
}

func TestDetector_CheckMassRename(t *testing.T) {
	det := NewDetector(DetectionThresholds{
		MassRenameThreshold: 3,
		MassRenameWindowSec: 60,
	})

	det.RegisterHoneypot("hp-1")

	// 记录少量重命名
	det.RecordEvent("hp-1", AccessEvent{EventType: "rename"})
	det.RecordEvent("hp-1", AccessEvent{EventType: "rename"})

	if det.CheckMassRename("hp-1") {
		t.Error("少量重命名不应触发告警")
	}

	// 达到阈值
	det.RecordEvent("hp-1", AccessEvent{EventType: "rename"})

	if !det.CheckMassRename("hp-1") {
		t.Error("达到阈值应触发告警")
	}
}

func TestDetector_UpdateThresholds(t *testing.T) {
	det := NewDetector(DefaultThresholds())

	newThresh := DetectionThresholds{
		EntropyChangeThreshold: 2.0,
		MassRenameThreshold:    10,
	}
	det.UpdateThresholds(newThresh)

	got := det.GetThresholds()
	if got.EntropyChangeThreshold != 2.0 {
		t.Errorf("阈值未更新: got %f", got.EntropyChangeThreshold)
	}
}

// ===== Handler 测试 =====

func TestHandler_CreateHoneypot(t *testing.T) {
	mgr := NewHoneypotManager()
	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(CreateHoneypotRequest{
		Name:      "api-test",
		SharePath: "/shares/honeypot",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ransomware/honeypot/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态 201, got %d", w.Code)
	}

	var hp Honeypot
	json.NewDecoder(w.Body).Decode(&hp)
	if hp.Name != "api-test" {
		t.Errorf("名称不匹配: got %s", hp.Name)
	}
}

func TestHandler_ListHoneypots(t *testing.T) {
	mgr := NewHoneypotManager()
	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ransomware/honeypot/list", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态 200, got %d", w.Code)
	}
}

func TestHandler_Scan(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "scan-api",
		SharePath: "/shares/honeypot",
	})

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	body, _ := json.Marshal(map[string]string{"honeypot_id": hp.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ransomware/scan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态 200, got %d", w.Code)
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	mgr := NewHoneypotManager()
	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ransomware/honeypot/create", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("期望状态 405, got %d", w.Code)
	}
}

// ===== 诱饵文件生成测试 =====

func TestGenerateDecoyFiles(t *testing.T) {
	mgr := NewHoneypotManager()

	// 默认所有类型
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "file-test",
		SharePath: "/shares/honeypot",
	})

	files := mgr.files[hp.ID]
	if len(files) == 0 {
		t.Error("应生成诱饵文件")
	}

	// 检查文件类型覆盖
	types := make(map[FileType]bool)
	for _, f := range files {
		types[f.FileType] = true
	}
	if !types[FileTypeOffice] {
		t.Error("缺少 Office 类型诱饵")
	}
	if !types[FileTypePDF] {
		t.Error("缺少 PDF 类型诱饵")
	}
}

func TestGenerateDecoyFiles_FilteredType(t *testing.T) {
	mgr := NewHoneypotManager()

	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "filter-test",
		SharePath: "/shares/honeypot",
		FileTypes: []FileType{FileTypePDF},
	})

	files := mgr.files[hp.ID]
	for _, f := range files {
		if f.FileType != FileTypePDF {
			t.Errorf("非 PDF 文件不应存在: %s", f.FilePath)
		}
	}
}

// ===== 扫描历史测试 =====

func TestGetScans(t *testing.T) {
	mgr := NewHoneypotManager()
	hp, _ := mgr.Create(CreateHoneypotRequest{
		Name:      "history-test",
		SharePath: "/shares/honeypot",
	})

	mgr.Scan(hp.ID)

	scans := mgr.GetScans()
	if len(scans) != 1 {
		t.Errorf("期望 1 条扫描记录, got %d", len(scans))
	}
}

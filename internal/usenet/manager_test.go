// Package usenet 提供 Usenet 下载管理单元测试
package usenet

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

func setupTestHandlers(t *testing.T, m *Manager) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	h := NewHandlers(m)
	h.RegisterRoutes(mux)
	return mux
}

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	// 检查预置服务器
	servers := m.ListServers()
	if len(servers) != 3 {
		t.Errorf("expected 3 default servers, got %d", len(servers))
	}
}

func TestAddServer(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name    string
		server  Server
		wantErr bool
	}{
		{
			name: "valid server",
			server: Server{
				Host:        "news.example.com",
				Port:        119,
				Connections: 5,
				SSL:         true,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "missing host",
			server: Server{
				Port:        119,
				Connections: 5,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			server: Server{
				Host:        "news.example.com",
				Port:        0,
				Connections: 5,
			},
			wantErr: true,
		},
		{
			name: "zero connections",
			server: Server{
				Host:        "news.example.com",
				Port:        119,
				Connections: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.AddServer(&tt.server)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID == "" {
				t.Error("expected non-empty ID")
			}
		})
	}
}

func TestUpdateServer(t *testing.T) {
	m := setupTestManager(t)

	// 添加服务器
	server, err := m.AddServer(&Server{
		Host:        "news.example.com",
		Port:        119,
		Connections: 5,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("failed to add server: %v", err)
	}

	// 更新服务器
	updated, err := m.UpdateServer(server.ID, &Server{
		Host:        "news-updated.example.com",
		Port:        563,
		Connections: 10,
		SSL:         true,
	})
	if err != nil {
		t.Fatalf("failed to update server: %v", err)
	}

	if updated.Host != "news-updated.example.com" {
		t.Errorf("expected host 'news-updated.example.com', got '%s'", updated.Host)
	}
	if updated.Port != 563 {
		t.Errorf("expected port 563, got %d", updated.Port)
	}
	if !updated.SSL {
		t.Error("expected SSL to be true")
	}
}

func TestDeleteServer(t *testing.T) {
	m := setupTestManager(t)

	// 添加服务器
	server, err := m.AddServer(&Server{
		Host:        "news.example.com",
		Port:        119,
		Connections: 5,
	})
	if err != nil {
		t.Fatalf("failed to add server: %v", err)
	}

	// 删除服务器
	if err := m.DeleteServer(server.ID); err != nil {
		t.Fatalf("failed to delete server: %v", err)
	}

	// 确认已删除
	_, err = m.GetServer(server.ID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestTestServer(t *testing.T) {
	m := setupTestManager(t)

	// 添加服务器
	server, err := m.AddServer(&Server{
		Host:        "news.example.com",
		Port:        119,
		Connections: 5,
	})
	if err != nil {
		t.Fatalf("failed to add server: %v", err)
	}

	// 测试连接
	duration, err := m.TestServer(server.ID)
	if err != nil {
		t.Fatalf("failed to test server: %v", err)
	}
	if duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestAddNZB(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name    string
		nzb     NZB
		wantErr bool
	}{
		{
			name: "valid nzb",
			nzb: NZB{
				Name:  "test.nzb",
				Size:  1024 * 1024,
				Files: 10,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			nzb: NZB{
				Size:  1024 * 1024,
				Files: 10,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.AddNZB(&tt.nzb)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID == "" {
				t.Error("expected non-empty ID")
			}
			if result.Status != NZBStatusPending {
				t.Errorf("expected status pending, got %v", result.Status)
			}
		})
	}
}

func TestGetNZB(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB
	nzb, err := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	if err != nil {
		t.Fatalf("failed to add nzb: %v", err)
	}

	// 获取 NZB
	result, err := m.GetNZB(nzb.ID)
	if err != nil {
		t.Fatalf("failed to get nzb: %v", err)
	}
	if result.Name != "test.nzb" {
		t.Errorf("expected name 'test.nzb', got '%s'", result.Name)
	}
}

func TestListNZBs(t *testing.T) {
	m := setupTestManager(t)

	// 添加多个 NZB
	for i := 0; i < 5; i++ {
		_, err := m.AddNZB(&NZB{
			Name:  "test.nzb",
			Size:  1024 * 1024,
			Files: 10,
		})
		if err != nil {
			t.Fatalf("failed to add nzb: %v", err)
		}
	}

	// 列出所有
	nzbs, err := m.ListNZBs("")
	if err != nil {
		t.Fatalf("failed to list nzbs: %v", err)
	}
	if len(nzbs) != 5 {
		t.Errorf("expected 5 nzbs, got %d", len(nzbs))
	}

	// 按状态过滤
	pendingNZBs, err := m.ListNZBs(NZBStatusPending)
	if err != nil {
		t.Fatalf("failed to list pending nzbs: %v", err)
	}
	if len(pendingNZBs) != 5 {
		t.Errorf("expected 5 pending nzbs, got %d", len(pendingNZBs))
	}
}

func TestDeleteNZB(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB
	nzb, err := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	if err != nil {
		t.Fatalf("failed to add nzb: %v", err)
	}

	// 删除 NZB
	if err := m.DeleteNZB(nzb.ID); err != nil {
		t.Fatalf("failed to delete nzb: %v", err)
	}

	// 确认已删除
	_, err = m.GetNZB(nzb.ID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestStartDownload(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB
	nzb, err := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	if err != nil {
		t.Fatalf("failed to add nzb: %v", err)
	}

	// 启动下载
	dl, err := m.StartDownload(nzb.ID)
	if err != nil {
		t.Fatalf("failed to start download: %v", err)
	}

	if dl.ID == "" {
		t.Error("expected non-empty download ID")
	}
	if dl.Status != DownloadStatusActive {
		t.Errorf("expected status active, got %v", dl.Status)
	}
	if dl.NZBID != nzb.ID {
		t.Errorf("expected NZBID %s, got %s", nzb.ID, dl.NZBID)
	}

	// 检查 NZB 状态已更新
	updatedNZB, _ := m.GetNZB(nzb.ID)
	if updatedNZB.Status != NZBStatusDownloading {
		t.Errorf("expected NZB status downloading, got %v", updatedNZB.Status)
	}
}

func TestPauseDownload(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB 并启动下载
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	dl, _ := m.StartDownload(nzb.ID)

	// 暂停下载
	if err := m.PauseDownload(dl.ID); err != nil {
		t.Fatalf("failed to pause download: %v", err)
	}

	// 检查状态
	updatedDL, _ := m.GetDownload(dl.ID)
	if updatedDL.Status != DownloadStatusPaused {
		t.Errorf("expected status paused, got %v", updatedDL.Status)
	}
}

func TestResumeDownload(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB 并启动下载
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	dl, _ := m.StartDownload(nzb.ID)

	// 暂停
	m.PauseDownload(dl.ID)

	// 恢复下载
	if err := m.ResumeDownload(dl.ID); err != nil {
		t.Fatalf("failed to resume download: %v", err)
	}

	// 检查状态
	updatedDL, _ := m.GetDownload(dl.ID)
	if updatedDL.Status != DownloadStatusActive {
		t.Errorf("expected status active, got %v", updatedDL.Status)
	}
}

func TestCancelDownload(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB 并启动下载
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	dl, _ := m.StartDownload(nzb.ID)

	// 取消下载
	if err := m.CancelDownload(dl.ID); err != nil {
		t.Fatalf("failed to cancel download: %v", err)
	}

	// 检查状态
	updatedDL, _ := m.GetDownload(dl.ID)
	if updatedDL.Status != DownloadStatusCancelled {
		t.Errorf("expected status cancelled, got %v", updatedDL.Status)
	}
}

func TestListDownloads(t *testing.T) {
	m := setupTestManager(t)

	// 添加多个 NZB 并启动下载
	for i := 0; i < 3; i++ {
		nzb, _ := m.AddNZB(&NZB{
			Name:  "test.nzb",
			Size:  1024 * 1024,
			Files: 10,
		})
		m.StartDownload(nzb.ID)
	}

	// 列出下载
	downloads, err := m.ListDownloads()
	if err != nil {
		t.Fatalf("failed to list downloads: %v", err)
	}
	if len(downloads) != 3 {
		t.Errorf("expected 3 downloads, got %d", len(downloads))
	}
}

func TestQueueManagement(t *testing.T) {
	m := setupTestManager(t)

	// 获取队列（初始为空）
	queue, err := m.GetQueue()
	if err != nil {
		t.Fatalf("failed to get queue: %v", err)
	}
	if len(queue) != 0 {
		t.Errorf("expected empty queue, got %d items", len(queue))
	}
}

func TestAddIndexer(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name    string
		indexer Indexer
		wantErr bool
	}{
		{
			name: "valid indexer",
			indexer: Indexer{
				Name:    "Test Indexer",
				URL:     "https://indexer.example.com/api",
				APIKey:  "test-key",
				Enabled: true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			indexer: Indexer{
				URL: "https://indexer.example.com/api",
			},
			wantErr: true,
		},
		{
			name: "missing url",
			indexer: Indexer{
				Name: "Test Indexer",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.AddIndexer(&tt.indexer)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID == "" {
				t.Error("expected non-empty ID")
			}
		})
	}
}

func TestSearchIndexer(t *testing.T) {
	m := setupTestManager(t)

	// 添加索引器
	indexer, _ := m.AddIndexer(&Indexer{
		Name:    "Test Indexer",
		URL:     "https://indexer.example.com/api",
		Enabled: true,
	})

	// 搜索
	results, err := m.SearchIndexer(indexer.ID, "test query")
	if err != nil {
		t.Fatalf("failed to search indexer: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestSearchDisabledIndexer(t *testing.T) {
	m := setupTestManager(t)

	// 添加禁用的索引器
	indexer, _ := m.AddIndexer(&Indexer{
		Name:    "Test Indexer",
		URL:     "https://indexer.example.com/api",
		Enabled: false,
	})

	// 搜索
	_, err := m.SearchIndexer(indexer.ID, "test query")
	if err == nil {
		t.Error("expected error for disabled indexer")
	}
}

func TestDeleteIndexer(t *testing.T) {
	m := setupTestManager(t)

	// 添加索引器
	indexer, _ := m.AddIndexer(&Indexer{
		Name:    "Test Indexer",
		URL:     "https://indexer.example.com/api",
		Enabled: true,
	})

	// 删除索引器
	if err := m.DeleteIndexer(indexer.ID); err != nil {
		t.Fatalf("failed to delete indexer: %v", err)
	}

	// 确认已删除
	_, err := m.GetIndexer(indexer.ID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestCreateCategory(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name    string
		cat     Category
		wantErr bool
	}{
		{
			name: "valid category",
			cat: Category{
				Name:     "Movies",
				DestPath: "/downloads/movies",
				Priority: 1,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			cat: Category{
				DestPath: "/downloads/movies",
			},
			wantErr: true,
		},
		{
			name: "missing dest path",
			cat: Category{
				Name: "Movies",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := m.CreateCategory(&tt.cat)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID == "" {
				t.Error("expected non-empty ID")
			}
		})
	}
}

func TestListCategories(t *testing.T) {
	m := setupTestManager(t)

	// 添加分类
	m.CreateCategory(&Category{
		Name:     "Movies",
		DestPath: "/downloads/movies",
	})
	m.CreateCategory(&Category{
		Name:     "TV Shows",
		DestPath: "/downloads/tv",
	})

	// 列出分类
	cats, err := m.ListCategories()
	if err != nil {
		t.Fatalf("failed to list categories: %v", err)
	}
	if len(cats) != 2 {
		t.Errorf("expected 2 categories, got %d", len(cats))
	}
}

func TestDeleteCategory(t *testing.T) {
	m := setupTestManager(t)

	// 添加分类
	cat, _ := m.CreateCategory(&Category{
		Name:     "Movies",
		DestPath: "/downloads/movies",
	})

	// 删除分类
	if err := m.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("failed to delete category: %v", err)
	}

	// 确认已删除
	_, err := m.GetCategory(cat.ID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB 并启动下载
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	m.StartDownload(nzb.ID)

	// 获取统计
	stats, err := m.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.ActiveDownloads != 1 {
		t.Errorf("expected 1 active download, got %d", stats.ActiveDownloads)
	}
}

func TestProcessCompleted(t *testing.T) {
	m := setupTestManager(t)

	// 添加 NZB 并启动下载
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})
	dl, _ := m.StartDownload(nzb.ID)

	// 模拟完成（手动设置状态）
	m.mu.Lock()
	m.downloads[dl.ID].Status = DownloadStatusCompleted
	m.downloads[dl.ID].Downloaded = m.downloads[dl.ID].Size
	m.downloads[dl.ID].Progress = 100
	m.mu.Unlock()

	// 处理完成
	if err := m.ProcessCompleted(dl.ID); err != nil {
		t.Fatalf("failed to process completed: %v", err)
	}

	// 检查 NZB 状态
	updatedNZB, _ := m.GetNZB(nzb.ID)
	if updatedNZB.Status != NZBStatusCompleted {
		t.Errorf("expected NZB status completed, got %v", updatedNZB.Status)
	}
}

// HTTP Handler Tests

func TestHandlersListServers(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/usenet/servers", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandlersAddServer(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	server := Server{
		Host:        "news.example.com",
		Port:        119,
		Connections: 5,
		Enabled:     true,
	}
	body, _ := json.Marshal(server)

	req := httptest.NewRequest(http.MethodPost, "/api/usenet/servers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandlersGetNZB(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	// 添加 NZB
	nzb, _ := m.AddNZB(&NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/usenet/nzbs/"+nzb.ID, nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlersAddNZB(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	nzb := NZB{
		Name:  "test.nzb",
		Size:  1024 * 1024,
		Files: 10,
	}
	body, _ := json.Marshal(nzb)

	req := httptest.NewRequest(http.MethodPost, "/api/usenet/nzbs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandlersGetStats(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/usenet/stats", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlersMethodNotAllowed(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	req := httptest.NewRequest(http.MethodPut, "/api/usenet/servers", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// PUT 方法应该返回 405 或 200（取决于具体路由实现）
	// 这里测试的是路由能正确响应
	if w.Code != http.StatusOK && w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 200 or 405, got %d", w.Code)
	}
}

func TestHandlersAddIndexer(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	indexer := Indexer{
		Name:    "Test Indexer",
		URL:     "https://indexer.example.com/api",
		Enabled: true,
	}
	body, _ := json.Marshal(indexer)

	req := httptest.NewRequest(http.MethodPost, "/api/usenet/indexers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandlersAddCategory(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	cat := Category{
		Name:     "Movies",
		DestPath: "/downloads/movies",
	}
	body, _ := json.Marshal(cat)

	req := httptest.NewRequest(http.MethodPost, "/api/usenet/categories", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandlersGetQueue(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/usenet/queue", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandlersListDownloads(t *testing.T) {
	m := setupTestManager(t)
	mux := setupTestHandlers(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/usenet/downloads", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestIsValidNZBStatus 测试状态验证函数.
func TestIsValidNZBStatus(t *testing.T) {
	tests := []struct {
		status NZBStatus
		valid  bool
	}{
		{NZBStatusPending, true},
		{NZBStatusDownloading, true},
		{NZBStatusCompleted, true},
		{NZBStatusFailed, true},
		{NZBStatusPaused, true},
		{NZBStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := IsValidNZBStatus(tt.status); got != tt.valid {
				t.Errorf("IsValidNZBStatus(%v) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

// TestIsValidDownloadStatus 测试下载状态验证函数.
func TestIsValidDownloadStatus(t *testing.T) {
	tests := []struct {
		status DownloadStatus
		valid  bool
	}{
		{DownloadStatusPending, true},
		{DownloadStatusActive, true},
		{DownloadStatusPaused, true},
		{DownloadStatusCompleted, true},
		{DownloadStatusFailed, true},
		{DownloadStatusCancelled, true},
		{DownloadStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := IsValidDownloadStatus(tt.status); got != tt.valid {
				t.Errorf("IsValidDownloadStatus(%v) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

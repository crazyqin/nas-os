// Package filesyncclient 测试文件
package filesyncclient

import (
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.clients == nil {
		t.Error("clients map not initialized")
	}
	if m.folders == nil {
		t.Error("folders map not initialized")
	}
	if m.conflicts == nil {
		t.Error("conflicts map not initialized")
	}
	if m.files == nil {
		t.Error("files map not initialized")
	}
	if m.events == nil {
		t.Error("events slice not initialized")
	}
}

func TestRegisterClient(t *testing.T) {
	m := NewManager()

	req := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
		OS:         "linux",
	}

	client, err := m.RegisterClient(req)
	if err != nil {
		t.Fatalf("RegisterClient failed: %v", err)
	}

	if client.ID == "" {
		t.Error("client ID is empty")
	}
	if client.Name != req.Name {
		t.Errorf("expected name %s, got %s", req.Name, client.Name)
	}
	if client.DeviceType != req.DeviceType {
		t.Errorf("expected device type %s, got %s", req.DeviceType, client.DeviceType)
	}
	if client.Status != ClientOnline {
		t.Errorf("expected status %s, got %s", ClientOnline, client.Status)
	}
}

func TestRegisterClientInvalidType(t *testing.T) {
	m := NewManager()

	req := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: "invalid",
	}

	_, err := m.RegisterClient(req)
	if err == nil {
		t.Error("expected error for invalid device type")
	}
}

func TestListClients(t *testing.T) {
	m := NewManager()

	// 注册多个客户端
	for i := 0; i < 3; i++ {
		req := &RegisterClientRequest{
			Name:       "test-client",
			DeviceType: DeviceDesktop,
			OS:         "linux",
		}
		m.RegisterClient(req)
	}

	clients := m.ListClients()
	if len(clients) != 3 {
		t.Errorf("expected 3 clients, got %d", len(clients))
	}
}

func TestRemoveClient(t *testing.T) {
	m := NewManager()

	req := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}

	client, _ := m.RegisterClient(req)

	// 创建同步文件夹
	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	m.CreateSyncFolder(folderReq)

	// 移除客户端
	if err := m.RemoveClient(client.ID); err != nil {
		t.Fatalf("RemoveClient failed: %v", err)
	}

	// 验证客户端已移除
	clients := m.ListClients()
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}

	// 验证文件夹已移除
	folders := m.ListFolders()
	if len(folders) != 0 {
		t.Errorf("expected 0 folders, got %d", len(folders))
	}
}

func TestRemoveClientNotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveClient("non-existent")
	if err == nil {
		t.Error("expected error for non-existent client")
	}
}

func TestCreateSyncFolder(t *testing.T) {
	m := NewManager()

	// 先注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	req := &CreateFolderRequest{
		ClientID:       client.ID,
		LocalPath:      "/local/path",
		RemotePath:     "/remote/path",
		SyncMode:       SyncTwoWay,
		ConflictPolicy: ConflictKeepLocal,
	}

	folder, err := m.CreateSyncFolder(req)
	if err != nil {
		t.Fatalf("CreateSyncFolder failed: %v", err)
	}

	if folder.ID == "" {
		t.Error("folder ID is empty")
	}
	if folder.ClientID != req.ClientID {
		t.Errorf("expected client ID %s, got %s", req.ClientID, folder.ClientID)
	}
	if folder.Status != FolderActive {
		t.Errorf("expected status %s, got %s", FolderActive, folder.Status)
	}
}

func TestCreateSyncFolderInvalidClient(t *testing.T) {
	m := NewManager()

	req := &CreateFolderRequest{
		ClientID:   "non-existent",
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}

	_, err := m.CreateSyncFolder(req)
	if err == nil {
		t.Error("expected error for non-existent client")
	}
}

func TestListFolders(t *testing.T) {
	m := NewManager()

	// 注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	// 创建多个文件夹
	for i := 0; i < 3; i++ {
		req := &CreateFolderRequest{
			ClientID:   client.ID,
			LocalPath:  "/local/path",
			RemotePath: "/remote/path",
		}
		m.CreateSyncFolder(req)
	}

	folders := m.ListFolders()
	if len(folders) != 3 {
		t.Errorf("expected 3 folders, got %d", len(folders))
	}
}

func TestUpdateFolder(t *testing.T) {
	m := NewManager()

	// 注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	// 创建文件夹
	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	folder, _ := m.CreateSyncFolder(folderReq)

	// 更新文件夹
	updateReq := &UpdateFolderRequest{
		SyncMode:       SyncOneWay,
		Status:         FolderPaused,
		ConflictPolicy: ConflictKeepRemote,
	}

	updated, err := m.UpdateFolder(folder.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateFolder failed: %v", err)
	}

	if updated.SyncMode != SyncOneWay {
		t.Errorf("expected sync mode %s, got %s", SyncOneWay, updated.SyncMode)
	}
	if updated.Status != FolderPaused {
		t.Errorf("expected status %s, got %s", FolderPaused, updated.Status)
	}
	if updated.ConflictPolicy != ConflictKeepRemote {
		t.Errorf("expected conflict policy %s, got %s", ConflictKeepRemote, updated.ConflictPolicy)
	}
}

func TestUpdateFolderNotFound(t *testing.T) {
	m := NewManager()

	req := &UpdateFolderRequest{
		SyncMode: SyncOneWay,
	}

	_, err := m.UpdateFolder("non-existent", req)
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}

func TestTriggerSync(t *testing.T) {
	m := NewManager()

	// 注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	// 创建文件夹
	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	folder, _ := m.CreateSyncFolder(folderReq)

	// 触发同步
	if err := m.TriggerSync(folder.ID); err != nil {
		t.Fatalf("TriggerSync failed: %v", err)
	}

	// 验证文件夹状态
	folders := m.ListFolders()
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder, got %d", len(folders))
	}

	if folders[0].Status != FolderActive {
		t.Errorf("expected status %s, got %s", FolderActive, folders[0].Status)
	}
	if folders[0].FileCount != 5 {
		t.Errorf("expected file count 5, got %d", folders[0].FileCount)
	}
}

func TestTriggerSyncNotFound(t *testing.T) {
	m := NewManager()

	err := m.TriggerSync("non-existent")
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}

func TestListConflicts(t *testing.T) {
	m := NewManager()

	conflicts := m.ListConflicts()
	if len(conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(conflicts))
	}
}

func TestResolveConflict(t *testing.T) {
	m := NewManager()

	// 手动添加冲突
	m.mu.Lock()
	conflict := &SyncConflict{
		ID:            "test-conflict",
		FolderID:      "test-folder",
		FilePath:      "/test/file.txt",
		LocalVersion:  "v1",
		RemoteVersion: "v2",
	}
	m.conflicts[conflict.ID] = conflict
	m.mu.Unlock()

	req := &ResolveConflictRequest{
		Resolution: "keep_local",
	}

	if err := m.ResolveConflict(conflict.ID, req); err != nil {
		t.Fatalf("ResolveConflict failed: %v", err)
	}

	// 验证冲突已解决
	conflicts := m.ListConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(conflicts))
	}

	if conflicts[0].Resolution != "keep_local" {
		t.Errorf("expected resolution 'keep_local', got %s", conflicts[0].Resolution)
	}
}

func TestResolveConflictNotFound(t *testing.T) {
	m := NewManager()

	req := &ResolveConflictRequest{
		Resolution: "keep_local",
	}

	err := m.ResolveConflict("non-existent", req)
	if err == nil {
		t.Error("expected error for non-existent conflict")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	// 注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	// 创建文件夹
	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	m.CreateSyncFolder(folderReq)

	// 触发同步
	folders := m.ListFolders()
	m.TriggerSync(folders[0].ID)

	stats := m.GetStats()
	if stats.TotalClients != 1 {
		t.Errorf("expected 1 client, got %d", stats.TotalClients)
	}
	if stats.TotalFolders != 1 {
		t.Errorf("expected 1 folder, got %d", stats.TotalFolders)
	}
	if stats.TotalFiles != 5 {
		t.Errorf("expected 5 files, got %d", stats.TotalFiles)
	}
}

func TestGetEvents(t *testing.T) {
	m := NewManager()

	// 注册客户端
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	// 创建文件夹
	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	m.CreateSyncFolder(folderReq)

	// 获取所有事件
	events := m.GetEvents("")
	if len(events) < 2 {
		t.Errorf("expected at least 2 events, got %d", len(events))
	}

	// 按客户端ID筛选
	clientEvents := m.GetEvents(client.ID)
	if len(clientEvents) < 2 {
		t.Errorf("expected at least 2 events for client, got %d", len(clientEvents))
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 并发注册客户端
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &RegisterClientRequest{
				Name:       "test-client",
				DeviceType: DeviceDesktop,
			}
			_, err := m.RegisterClient(req)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		t.Errorf("concurrent RegisterClient error: %v", err)
	}

	clients := m.ListClients()
	if len(clients) != 10 {
		t.Errorf("expected 10 clients, got %d", len(clients))
	}

	// 并发创建文件夹
	if len(clients) > 0 {
		errChan = make(chan error, 50)
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := &CreateFolderRequest{
					ClientID:   clients[0].ID,
					LocalPath:  "/local/path",
					RemotePath: "/remote/path",
				}
				_, err := m.CreateSyncFolder(req)
				if err != nil {
					errChan <- err
				}
			}()
		}

		wg.Wait()
		close(errChan)

		for err := range errChan {
			t.Errorf("concurrent CreateSyncFolder error: %v", err)
		}

		folders := m.ListFolders()
		if len(folders) != 5 {
			t.Errorf("expected 5 folders, got %d", len(folders))
		}
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	m := NewManager()

	// 先创建一些数据
	clientReq := &RegisterClientRequest{
		Name:       "test-client",
		DeviceType: DeviceDesktop,
	}
	client, _ := m.RegisterClient(clientReq)

	folderReq := &CreateFolderRequest{
		ClientID:   client.ID,
		LocalPath:  "/local/path",
		RemotePath: "/remote/path",
	}
	folder, _ := m.CreateSyncFolder(folderReq)

	var wg sync.WaitGroup

	// 并发读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.ListClients()
			_ = m.ListFolders()
			_ = m.GetStats()
			_ = m.GetEvents("")
		}()
	}

	// 并发写入
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.TriggerSync(folder.ID)
		}()
	}

	wg.Wait()
}

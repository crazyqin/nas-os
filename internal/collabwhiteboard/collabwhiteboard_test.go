// Package collabwhiteboard 测试
package collabwhiteboard

import (
	"testing"
	"time"
)

func TestCreateBoard(t *testing.T) {
	e := NewEngine()
	board, err := e.CreateBoard(CreateBoardRequest{
		Title:       "测试白板",
		Description: "这是一个测试白板",
		Owner:       "admin",
		Width:       1920,
		Height:      1080,
	})
	if err != nil {
		t.Fatalf("创建白板失败: %v", err)
	}
	if board == nil {
		t.Fatal("白板不应为nil")
	}
	if board.Title != "测试白板" {
		t.Errorf("标题不匹配: %s", board.Title)
	}
	if board.Width != 1920 {
		t.Errorf("宽度不匹配: %d", board.Width)
	}
}

func TestGetBoard(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})

	got, err := e.GetBoard(board.ID)
	if err != nil {
		t.Fatalf("获取白板失败: %v", err)
	}
	if got.Title != "test" {
		t.Errorf("标题不匹配")
	}
}

func TestGetBoardNotFound(t *testing.T) {
	e := NewEngine()
	_, err := e.GetBoard("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestUpdateBoard(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "old", Owner: "admin"})

	newTitle := "new"
	updated, err := e.UpdateBoard(board.ID, UpdateBoardRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Title != "new" {
		t.Errorf("标题未更新: %s", updated.Title)
	}
}

func TestDeleteBoard(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "to delete", Owner: "admin"})

	err := e.DeleteBoard(board.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = e.GetBoard(board.ID)
	if err == nil {
		t.Error("已删除白板不应存在")
	}
}

func TestListBoards(t *testing.T) {
	e := NewEngine()
	e.CreateBoard(CreateBoardRequest{Title: "board1", Owner: "admin"})
	e.CreateBoard(CreateBoardRequest{Title: "board2", Owner: "admin"})
	e.CreateBoard(CreateBoardRequest{Title: "board3", Owner: "other"})

	adminBoards := e.ListBoards("admin")
	if len(adminBoards) != 2 {
		t.Errorf("期望2个白板，实际 %d", len(adminBoards))
	}

	allBoards := e.ListBoards("")
	if len(allBoards) != 3 {
		t.Errorf("期望3个白板，实际 %d", len(allBoards))
	}
}

func TestAddElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})

	elem, err := e.AddElement(board.ID, AddElementRequest{
		Type:   "shape",
		X:      100,
		Y:      100,
		Width:  200,
		Height: 100,
		Style:  Style{StrokeColor: "#000000", FillColor: "#ffffff"},
	}, "admin")
	if err != nil {
		t.Fatalf("添加元素失败: %v", err)
	}
	if elem.Type != "shape" {
		t.Errorf("类型不匹配: %s", elem.Type)
	}
	if elem.X != 100 {
		t.Errorf("X坐标不匹配: %f", elem.X)
	}
}

func TestUpdateElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape", X: 100, Y: 100}, "admin")

	newX := 200.0
	updated, err := e.UpdateElement(board.ID, elem.ID, UpdateElementRequest{X: &newX}, "admin")
	if err != nil {
		t.Fatalf("更新元素失败: %v", err)
	}
	if updated.X != 200 {
		t.Errorf("X坐标未更新: %f", updated.X)
	}
}

func TestDeleteElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")

	err := e.DeleteElement(board.ID, elem.ID, "admin")
	if err != nil {
		t.Fatalf("删除元素失败: %v", err)
	}

	_, err = e.GetElement(board.ID, elem.ID)
	if err == nil {
		t.Error("已删除元素不应存在")
	}
}

func TestMoveElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape", X: 100, Y: 100}, "admin")

	moved, err := e.MoveElement(board.ID, elem.ID, MoveElementRequest{X: 200, Y: 300}, "admin")
	if err != nil {
		t.Fatalf("移动元素失败: %v", err)
	}
	if moved.X != 200 || moved.Y != 300 {
		t.Errorf("坐标未更新: %f, %f", moved.X, moved.Y)
	}
}

func TestResizeElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape", Width: 100, Height: 100}, "admin")

	resized, err := e.ResizeElement(board.ID, elem.ID, ResizeElementRequest{Width: 200, Height: 150}, "admin")
	if err != nil {
		t.Fatalf("调整大小失败: %v", err)
	}
	if resized.Width != 200 || resized.Height != 150 {
		t.Errorf("尺寸未更新: %f, %f", resized.Width, resized.Height)
	}
}

func TestGetElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")

	got, err := e.GetElement(board.ID, elem.ID)
	if err != nil {
		t.Fatalf("获取元素失败: %v", err)
	}
	if got.ID != elem.ID {
		t.Errorf("ID不匹配: %s", got.ID)
	}
}

func TestGetElements(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")
	e.AddElement(board.ID, AddElementRequest{Type: "text"}, "admin")

	elements, err := e.GetElements(board.ID)
	if err != nil {
		t.Fatalf("获取元素失败: %v", err)
	}
	if len(elements) != 2 {
		t.Errorf("期望2个元素，实际 %d", len(elements))
	}
}

func TestSaveVersion(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")

	version, err := e.SaveVersion(board.ID, "admin", "初始版本")
	if err != nil {
		t.Fatalf("保存版本失败: %v", err)
	}
	if version.Comment != "初始版本" {
		t.Errorf("备注不匹配: %s", version.Comment)
	}
}

func TestGetVersions(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.SaveVersion(board.ID, "admin", "v1")
	e.SaveVersion(board.ID, "admin", "v2")

	versions, err := e.GetVersions(board.ID)
	if err != nil {
		t.Fatalf("获取版本失败: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("期望2个版本，实际 %d", len(versions))
	}
}

func TestRestoreVersion(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddElement(board.ID, AddElementRequest{Type: "shape", X: 100}, "admin")
	version, _ := e.SaveVersion(board.ID, "admin", "v1")
	e.AddElement(board.ID, AddElementRequest{Type: "text", X: 200}, "admin")

	err := e.RestoreVersion(board.ID, version.ID, "admin")
	if err != nil {
		t.Fatalf("恢复版本失败: %v", err)
	}

	elements, _ := e.GetElements(board.ID)
	if len(elements) != 1 {
		t.Errorf("期望1个元素，实际 %d", len(elements))
	}
}

func TestGetOperations(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")
	e.AddElement(board.ID, AddElementRequest{Type: "text"}, "admin")

	operations, err := e.GetOperations(board.ID, 0)
	if err != nil {
		t.Fatalf("获取操作失败: %v", err)
	}
	if len(operations) != 2 {
		t.Errorf("期望2个操作，实际 %d", len(operations))
	}
}

func TestAddCollaborator(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})

	collab, err := e.AddCollaborator(board.ID, AddCollaboratorRequest{
		UserID:   "user1",
		Username: "Alice",
		Role:     "editor",
		Color:    "#ff0000",
	})
	if err != nil {
		t.Fatalf("添加协作者失败: %v", err)
	}
	if collab.Username != "Alice" {
		t.Errorf("用户名不匹配: %s", collab.Username)
	}
}

func TestRemoveCollaborator(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddCollaborator(board.ID, AddCollaboratorRequest{UserID: "user1", Username: "Alice", Role: "editor"})

	err := e.RemoveCollaborator(board.ID, "user1")
	if err != nil {
		t.Fatalf("移除协作者失败: %v", err)
	}
}

func TestGetCollaborators(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddCollaborator(board.ID, AddCollaboratorRequest{UserID: "user1", Username: "Alice", Role: "editor"})
	e.AddCollaborator(board.ID, AddCollaboratorRequest{UserID: "user2", Username: "Bob", Role: "viewer"})

	collabs, err := e.GetCollaborators(board.ID)
	if err != nil {
		t.Fatalf("获取协作者失败: %v", err)
	}
	// 包括所有者和2个协作者
	if len(collabs) != 3 {
		t.Errorf("期望3个协作者，实际 %d", len(collabs))
	}
}

func TestClearBoard(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	e.AddElement(board.ID, AddElementRequest{Type: "shape"}, "admin")
	e.AddElement(board.ID, AddElementRequest{Type: "text"}, "admin")

	err := e.ClearBoard(board.ID, "admin")
	if err != nil {
		t.Fatalf("清空白板失败: %v", err)
	}

	elements, _ := e.GetElements(board.ID)
	if len(elements) != 0 {
		t.Errorf("期望0个元素，实际 %d", len(elements))
	}
}

func TestDuplicateElement(t *testing.T) {
	e := NewEngine()
	board, _ := e.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	elem, _ := e.AddElement(board.ID, AddElementRequest{Type: "shape", X: 100, Y: 100}, "admin")

	duplicated, err := e.DuplicateElement(board.ID, elem.ID, "admin")
	if err != nil {
		t.Fatalf("复制元素失败: %v", err)
	}
	if duplicated.ID == elem.ID {
		t.Error("复制元素ID应不同")
	}
	if duplicated.X != 120 {
		t.Errorf("X坐标应偏移20: %f", duplicated.X)
	}
}

func TestRealtimeJoinBoard(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	err := rt.JoinBoard(board.ID, "user1")
	if err != nil {
		t.Fatalf("加入白板失败: %v", err)
	}

	users, _ := rt.GetConnectedUsers(board.ID)
	if len(users) != 1 {
		t.Errorf("期望1个用户，实际 %d", len(users))
	}
}

func TestRealtimeLeaveBoard(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	rt.JoinBoard(board.ID, "user1")
	rt.LeaveBoard(board.ID, "user1")

	users, _ := rt.GetConnectedUsers(board.ID)
	if len(users) != 0 {
		t.Errorf("期望0个用户，实际 %d", len(users))
	}
}

func TestRealtimeUpdateCursor(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	rt.JoinBoard(board.ID, "user1")
	rt.UpdateCursor(board.ID, CursorUpdate{
		UserID:   "user1",
		Username: "Alice",
		X:        100,
		Y:        200,
		Color:    "#ff0000",
	})

	cursors, _ := rt.GetCursors(board.ID)
	if len(cursors) != 1 {
		t.Errorf("期望1个光标，实际 %d", len(cursors))
	}
	if cursors[0].X != 100 {
		t.Errorf("X坐标不匹配: %f", cursors[0].X)
	}
}

func TestRealtimeBroadcastOperation(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	ch, _ := rt.Subscribe(board.ID)

	op := Operation{
		ID:        "op1",
		Type:      "add",
		ElementID: "elem1",
		UserID:    "user1",
		Timestamp: time.Now(),
	}

	rt.BroadcastOperation(board.ID, op)

	select {
	case received := <-ch:
		if received.ID != "op1" {
			t.Errorf("操作ID不匹配: %s", received.ID)
		}
	case <-time.After(time.Second):
		t.Error("未收到操作")
	}
}

func TestRealtimeApplyOT(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	op := OTOperation{
		ID:        "op1",
		Type:      "insert",
		ElementID: "elem1",
		Position:  0,
		Version:   0,
		UserID:    "user1",
		Timestamp: time.Now(),
	}

	result, err := rt.ApplyOT(board.ID, op)
	if err != nil {
		t.Fatalf("应用OT操作失败: %v", err)
	}
	if !result.Applied {
		t.Error("操作应已应用")
	}
}

func TestRealtimeSyncState(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	rt.JoinBoard(board.ID, "user1")
	rt.JoinBoard(board.ID, "user2")

	state, err := rt.SyncState(board.ID)
	if err != nil {
		t.Fatalf("获取同步状态失败: %v", err)
	}
	if state.ConnectedUsers != 2 {
		t.Errorf("期望2个用户，实际 %d", state.ConnectedUsers)
	}
}

func TestRealtimeSyncFull(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	rt.JoinBoard(board.ID, "user1")

	data, err := rt.SyncFull(board.ID)
	if err != nil {
		t.Fatalf("全量同步失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("同步数据不应为空")
	}
}

func TestRealtimeCleanupStaleCursors(t *testing.T) {
	engine := NewEngine()
	board, _ := engine.CreateBoard(CreateBoardRequest{Title: "test", Owner: "admin"})
	rt := NewRealtimeEngine(engine)

	rt.JoinBoard(board.ID, "user1")
	rt.UpdateCursor(board.ID, CursorUpdate{
		UserID: "user1",
		X:      100,
		Y:      100,
	})

	// 模拟过期光标
	rt.mu.Lock()
	if rt.cursors[board.ID] != nil {
		if cursor, ok := rt.cursors[board.ID]["user1"]; ok {
			cursor.UpdatedAt = time.Now().Add(-time.Hour)
		}
	}
	rt.mu.Unlock()

	removed := rt.CleanupStaleCursors(board.ID, 30*time.Minute)
	if removed != 1 {
		t.Errorf("期望清理1个光标，实际 %d", removed)
	}
}

func TestRendererToSVG(t *testing.T) {
	r := NewRenderer()
	board := &Board{
		ID:     "board1",
		Width:  800,
		Height: 600,
		Elements: []Element{
			{
				ID:      "elem1",
				Type:    "shape",
				X:       100,
				Y:       100,
				Width:   200,
				Height:  100,
				Visible: true,
				Style: Style{
					StrokeColor: "#000000",
					FillColor:   "#ffffff",
					StrokeWidth: 2,
				},
			},
		},
	}

	svg, err := r.RenderToSVG(board)
	if err != nil {
		t.Fatalf("渲染SVG失败: %v", err)
	}
	if svg == "" {
		t.Error("SVG不应为空")
	}
	if !contains(svg, "<svg") {
		t.Error("SVG应包含svg标签")
	}
}

func TestRendererToCanvas(t *testing.T) {
	r := NewRenderer()
	board := &Board{
		ID:     "board1",
		Width:  800,
		Height: 600,
		Elements: []Element{
			{
				ID:      "elem1",
				Type:    "shape",
				X:       100,
				Y:       100,
				Width:   200,
				Height:  100,
				Visible: true,
			},
		},
	}

	commands, err := r.RenderToCanvas(board)
	if err != nil {
		t.Fatalf("渲染Canvas失败: %v", err)
	}
	if len(commands) == 0 {
		t.Error("命令不应为空")
	}
}

func TestRendererToJSON(t *testing.T) {
	r := NewRenderer()
	board := &Board{
		ID:    "board1",
		Title: "Test",
	}

	data, err := r.ExportToJSON(board)
	if err != nil {
		t.Fatalf("导出JSON失败: %v", err)
	}
	if len(data) == 0 {
		t.Error("JSON不应为空")
	}
}

func TestRendererStats(t *testing.T) {
	r := NewRenderer()
	board := &Board{
		Elements: []Element{
			{Type: "shape", Visible: true},
			{Type: "text", Visible: false},
			{Type: "stroke", Visible: true, Locked: true},
		},
	}

	stats := r.GetRenderStats(board)
	if stats["total"] != 3 {
		t.Errorf("总数应为3，实际 %d", stats["total"])
	}
	if stats["visible"] != 2 {
		t.Errorf("可见数应为2，实际 %d", stats["visible"])
	}
	if stats["locked"] != 1 {
		t.Errorf("锁定数应为1，实际 %d", stats["locked"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

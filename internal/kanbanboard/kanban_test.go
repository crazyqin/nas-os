package kanbanboard

import (
	"fmt"
	"testing"
	"time"
)

// ============================================================
// 看板管理测试
// ============================================================

func TestCreateBoard(t *testing.T) {
	mgr := NewManager()

	req := &CreateBoardRequest{
		Name:        "测试看板",
		Description: "这是一个测试看板",
		OwnerID:     "user1",
	}

	board, err := mgr.CreateBoard(req)
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}

	if board.Name != "测试看板" {
		t.Errorf("expected name '测试看板', got '%s'", board.Name)
	}
	if board.OwnerID != "user1" {
		t.Errorf("expected owner 'user1', got '%s'", board.OwnerID)
	}
	if len(board.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(board.Columns))
	}
	if board.Status != BoardStatusActive {
		t.Errorf("expected status 'active', got '%s'", board.Status)
	}
}

func TestCreateBoardEmptyName(t *testing.T) {
	mgr := NewManager()

	req := &CreateBoardRequest{
		Name:    "",
		OwnerID: "user1",
	}

	_, err := mgr.CreateBoard(req)
	if err == nil {
		t.Error("expected error for empty board name")
	}
}

func TestGetBoard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	found, err := mgr.GetBoard(board.ID)
	if err != nil {
		t.Fatalf("GetBoard failed: %v", err)
	}
	if found.ID != board.ID {
		t.Error("board ID mismatch")
	}
}

func TestGetBoardNotFound(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.GetBoard("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent board")
	}
}

func TestListBoards(t *testing.T) {
	mgr := NewManager()

	mgr.CreateBoard(&CreateBoardRequest{Name: "Board1", OwnerID: "u1"})
	mgr.CreateBoard(&CreateBoardRequest{Name: "Board2", OwnerID: "u2"})

	boards := mgr.ListBoards()
	if len(boards) != 2 {
		t.Errorf("expected 2 boards, got %d", len(boards))
	}
}

func TestUpdateBoard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Old", OwnerID: "u1"})

	newName := "New"
	newDesc := "Updated description"
	updated, err := mgr.UpdateBoard(board.ID, &UpdateBoardRequest{
		Name:        &newName,
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("UpdateBoard failed: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("expected name 'New', got '%s'", updated.Name)
	}
	if updated.Description != "Updated description" {
		t.Errorf("expected description update")
	}
}

func TestDeleteBoard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Delete Me", OwnerID: "u1"})

	err := mgr.DeleteBoard(board.ID)
	if err != nil {
		t.Fatalf("DeleteBoard failed: %v", err)
	}

	_, err = mgr.GetBoard(board.ID)
	if err == nil {
		t.Error("expected error after deletion")
	}
}

func TestArchiveBoard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Archive Me", OwnerID: "u1"})

	err := mgr.ArchiveBoard(board.ID)
	if err != nil {
		t.Fatalf("ArchiveBoard failed: %v", err)
	}

	updated, _ := mgr.GetBoard(board.ID)
	if updated.Status != BoardStatusArchived {
		t.Errorf("expected archived status, got '%s'", updated.Status)
	}
}

// ============================================================
// 列管理测试
// ============================================================

func TestAddColumn(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	col, err := mgr.AddColumn(board.ID, &CreateColumnRequest{
		Name:     "Review",
		Position: 2,
		WIPLimit: 3,
	})
	if err != nil {
		t.Fatalf("AddColumn failed: %v", err)
	}

	if col.Name != "Review" {
		t.Errorf("expected name 'Review', got '%s'", col.Name)
	}
	if col.WIPLimit != 3 {
		t.Errorf("expected WIP limit 3, got %d", col.WIPLimit)
	}
}

func TestUpdateColumn(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	newName := "Updated Column"
	newWIP := 10
	updated, err := mgr.UpdateColumn(board.ID, board.Columns[0].ID, &UpdateColumnRequest{
		Name:     &newName,
		WIPLimit: &newWIP,
	})
	if err != nil {
		t.Fatalf("UpdateColumn failed: %v", err)
	}
	if updated.Name != "Updated Column" {
		t.Errorf("expected name 'Updated Column', got '%s'", updated.Name)
	}
	if updated.WIPLimit != 10 {
		t.Errorf("expected WIP limit 10, got %d", updated.WIPLimit)
	}
}

func TestDeleteColumn(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	// 添加一个新列并删除
	col, _ := mgr.AddColumn(board.ID, &CreateColumnRequest{Name: "Temp"})
	err := mgr.DeleteColumn(board.ID, col.ID)
	if err != nil {
		t.Fatalf("DeleteColumn failed: %v", err)
	}
}

func TestDeleteColumnWithCards(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	// 添加卡片到第一列
	mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID:  board.Columns[0].ID,
		Title:     "Card1",
		CreatedBy: "u1",
	})

	// 尝试删除有卡片的列
	err := mgr.DeleteColumn(board.ID, board.Columns[0].ID)
	if err == nil {
		t.Error("expected error when deleting column with cards")
	}
}

// ============================================================
// 卡片管理测试
// ============================================================

func TestAddCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	card, err := mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID:    board.Columns[0].ID,
		Title:       "实现登录功能",
		Description: "用户登录模块开发",
		Priority:    PriorityHigh,
		AssigneeID:  "user2",
		LabelIDs:    []string{"label1"},
		CreatedBy:   "u1",
	})
	if err != nil {
		t.Fatalf("AddCard failed: %v", err)
	}

	if card.Title != "实现登录功能" {
		t.Errorf("expected title '实现登录功能', got '%s'", card.Title)
	}
	if card.Priority != PriorityHigh {
		t.Errorf("expected priority 'high', got '%s'", card.Priority)
	}
	if card.Status != CardStatusTodo {
		t.Errorf("expected status 'todo', got '%s'", card.Status)
	}
}

func TestAddCardDefaultPriority(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID:  board.Columns[0].ID,
		Title:     "No Priority",
		CreatedBy: "u1",
	})

	if card.Priority != PriorityMedium {
		t.Errorf("expected default priority 'medium', got '%s'", card.Priority)
	}
}

func TestAddCardWIPLimit(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	// 设置 WIP 限制为 2
	board.Columns[1].WIPLimit = 2

	// 添加 2 张卡片
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[1].ID, Title: "Card1", CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[1].ID, Title: "Card2", CreatedBy: "u1"})

	// 第 3 张应该失败
	_, err := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[1].ID, Title: "Card3", CreatedBy: "u1"})
	if err == nil {
		t.Error("expected error when exceeding WIP limit")
	}
}

func TestGetCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Find Me", CreatedBy: "u1"})

	found, err := mgr.GetCard(board.ID, card.ID)
	if err != nil {
		t.Fatalf("GetCard failed: %v", err)
	}
	if found.Title != "Find Me" {
		t.Errorf("expected title 'Find Me', got '%s'", found.Title)
	}
}

func TestUpdateCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Old Title", CreatedBy: "u1"})

	newTitle := "New Title"
	newPriority := PriorityUrgent
	updated, err := mgr.UpdateCard(board.ID, card.ID, &UpdateCardRequest{
		Title:    &newTitle,
		Priority: &newPriority,
	})
	if err != nil {
		t.Fatalf("UpdateCard failed: %v", err)
	}
	if updated.Title != "New Title" {
		t.Errorf("expected title 'New Title', got '%s'", updated.Title)
	}
	if updated.Priority != PriorityUrgent {
		t.Errorf("expected priority 'urgent', got '%s'", updated.Priority)
	}
}

func TestUpdateCardStatus(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Status Test", CreatedBy: "u1"})

	doneStatus := CardStatusDone
	updated, _ := mgr.UpdateCard(board.ID, card.ID, &UpdateCardRequest{
		Status: &doneStatus,
	})

	if updated.Status != CardStatusDone {
		t.Errorf("expected status 'done', got '%s'", updated.Status)
	}
	if updated.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestDeleteCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Delete Me", CreatedBy: "u1"})

	err := mgr.DeleteCard(board.ID, card.ID)
	if err != nil {
		t.Fatalf("DeleteCard failed: %v", err)
	}

	_, err = mgr.GetCard(board.ID, card.ID)
	if err == nil {
		t.Error("expected error after card deletion")
	}
}

func TestMoveCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Move Me", CreatedBy: "u1"})

	moved, err := mgr.MoveCard(board.ID, card.ID, &MoveCardRequest{
		TargetColumnID: board.Columns[1].ID,
		Position:       0,
	})
	if err != nil {
		t.Fatalf("MoveCard failed: %v", err)
	}

	if moved.ColumnID != board.Columns[1].ID {
		t.Errorf("expected column %s, got %s", board.Columns[1].ID, moved.ColumnID)
	}
}

func TestMoveCardWIPLimit(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	// 设置 WIP 限制为 1
	board.Columns[1].WIPLimit = 1

	// 添加卡片到不同列
	card1, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})
	card2, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[1].ID, Title: "Card2", CreatedBy: "u1"})
	_ = card2

	// 移动 card1 到已满的列
	_, err := mgr.MoveCard(board.ID, card1.ID, &MoveCardRequest{TargetColumnID: board.Columns[1].ID})
	if err == nil {
		t.Error("expected error when moving to column at WIP limit")
	}
}

func TestAssignCard(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Assign Me", CreatedBy: "u1"})

	assigned, err := mgr.AssignCard(board.ID, card.ID, "user3")
	if err != nil {
		t.Fatalf("AssignCard failed: %v", err)
	}
	if assigned.AssigneeID != "user3" {
		t.Errorf("expected assignee 'user3', got '%s'", assigned.AssigneeID)
	}
}

// ============================================================
// 标签管理测试
// ============================================================

func TestAddLabel(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	label, err := mgr.AddLabel(board.ID, &CreateLabelRequest{
		Name:  "Bug",
		Color: "#FF0000",
	})
	if err != nil {
		t.Fatalf("AddLabel failed: %v", err)
	}
	if label.Name != "Bug" {
		t.Errorf("expected name 'Bug', got '%s'", label.Name)
	}
}

func TestUpdateLabel(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Old", Color: "#000"})

	newName := "New"
	newColor := "#FFF"
	updated, err := mgr.UpdateLabel(board.ID, label.ID, &UpdateLabelRequest{
		Name:  &newName,
		Color: &newColor,
	})
	if err != nil {
		t.Fatalf("UpdateLabel failed: %v", err)
	}
	if updated.Name != "New" {
		t.Errorf("expected name 'New', got '%s'", updated.Name)
	}
}

func TestDeleteLabel(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Delete", Color: "#F00"})

	// 给卡片添加标签
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID: board.Columns[0].ID,
		Title:    "Test Card",
		LabelIDs: []string{label.ID},
		CreatedBy: "u1",
	})

	err := mgr.DeleteLabel(board.ID, label.ID)
	if err != nil {
		t.Fatalf("DeleteLabel failed: %v", err)
	}

	// 确认卡片的标签也被移除
	for _, id := range card.LabelIDs {
		if id == label.ID {
			t.Error("expected label to be removed from card")
		}
	}
}

func TestApplyLabel(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Feature", Color: "#0F0"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Test", CreatedBy: "u1"})

	err := mgr.ApplyLabel(board.ID, card.ID, label.ID)
	if err != nil {
		t.Fatalf("ApplyLabel failed: %v", err)
	}

	// 再次应用同一标签（应忽略）
	err = mgr.ApplyLabel(board.ID, card.ID, label.ID)
	if err != nil {
		t.Fatalf("ApplyLabel duplicate failed: %v", err)
	}

	// 验证只有一个标签
	updated, _ := mgr.GetCard(board.ID, card.ID)
	count := 0
	for _, id := range updated.LabelIDs {
		if id == label.ID {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 label, got %d", count)
	}
}

func TestRemoveLabel(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Remove", Color: "#F00"})
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID:  board.Columns[0].ID,
		Title:     "Test",
		LabelIDs:  []string{label.ID},
		CreatedBy: "u1",
	})

	err := mgr.RemoveLabel(board.ID, card.ID, label.ID)
	if err != nil {
		t.Fatalf("RemoveLabel failed: %v", err)
	}

	updated, _ := mgr.GetCard(board.ID, card.ID)
	if len(updated.LabelIDs) != 0 {
		t.Error("expected no labels after removal")
	}
}

// ============================================================
// 搜索与过滤测试
// ============================================================

func TestSearchCardsByAssignee(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", AssigneeID: "user1", CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card2", AssigneeID: "user2", CreatedBy: "u1"})

	results := mgr.SearchCards(&CardFilter{BoardID: board.ID, AssigneeID: "user1"})
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchCardsByStatus(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Todo", CreatedBy: "u1"})
	card2, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Done", CreatedBy: "u1"})

	doneStatus := CardStatusDone
	mgr.UpdateCard(board.ID, card2.ID, &UpdateCardRequest{Status: &doneStatus})

	results := mgr.SearchCards(&CardFilter{BoardID: board.ID, Status: CardStatusDone})
	if len(results) != 1 {
		t.Errorf("expected 1 done card, got %d", len(results))
	}
}

func TestSearchCardsByKeyword(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "实现登录功能", CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "数据库优化", CreatedBy: "u1"})

	results := mgr.SearchCards(&CardFilter{BoardID: board.ID, Keyword: "登录"})
	if len(results) != 1 {
		t.Errorf("expected 1 result for '登录', got %d", len(results))
	}
}

func TestSearchCardsByPriority(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Urgent", Priority: PriorityUrgent, CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Low", Priority: PriorityLow, CreatedBy: "u1"})

	results := mgr.SearchCards(&CardFilter{BoardID: board.ID, Priority: PriorityUrgent})
	if len(results) != 1 {
		t.Errorf("expected 1 urgent card, got %d", len(results))
	}
}

func TestSearchCardsByLabels(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Bug", Color: "#F00"})

	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Buggy", LabelIDs: []string{label.ID}, CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Clean", CreatedBy: "u1"})

	results := mgr.SearchCards(&CardFilter{BoardID: board.ID, LabelIDs: []string{label.ID}})
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// ============================================================
// 统计报表测试
// ============================================================

func TestGetBoardStats(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	// 添加不同状态的卡片
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Todo1", CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Todo2", CreatedBy: "u1"})

	card3, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[1].ID, Title: "InProgress", CreatedBy: "u1"})
	inProgress := CardStatusInProgress
	mgr.UpdateCard(board.ID, card3.ID, &UpdateCardRequest{Status: &inProgress})

	card4, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[2].ID, Title: "Done", CreatedBy: "u1"})
	done := CardStatusDone
	mgr.UpdateCard(board.ID, card4.ID, &UpdateCardRequest{Status: &done})

	stats, err := mgr.GetBoardStats(board.ID)
	if err != nil {
		t.Fatalf("GetBoardStats failed: %v", err)
	}

	if stats.TotalCards != 4 {
		t.Errorf("expected 4 total cards, got %d", stats.TotalCards)
	}
	if stats.TodoCards != 2 {
		t.Errorf("expected 2 todo cards, got %d", stats.TodoCards)
	}
	if stats.InProgressCards != 1 {
		t.Errorf("expected 1 in_progress card, got %d", stats.InProgressCards)
	}
	if stats.CompletedCards != 1 {
		t.Errorf("expected 1 completed card, got %d", stats.CompletedCards)
	}
}

func TestGetBurndownData(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})

	points, err := mgr.GetBurndownData(board.ID, 7)
	if err != nil {
		t.Fatalf("GetBurndownData failed: %v", err)
	}
	if len(points) != 7 {
		t.Errorf("expected 7 points, got %d", len(points))
	}
}

func TestGetVelocityData(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})

	points, err := mgr.GetVelocityData(board.ID, 4)
	if err != nil {
		t.Fatalf("GetVelocityData failed: %v", err)
	}
	if len(points) != 4 {
		t.Errorf("expected 4 points, got %d", len(points))
	}
}

func TestGetCumulativeFlowData(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})

	points, err := mgr.GetCumulativeFlowData(board.ID, 7)
	if err != nil {
		t.Fatalf("GetCumulativeFlowData failed: %v", err)
	}
	if len(points) != 7 {
		t.Errorf("expected 7 points, got %d", len(points))
	}
}

// ============================================================
// 成员管理测试
// ============================================================

func TestAssignMember(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	member, err := mgr.AssignMember(board.ID, &AssignMemberRequest{
		UserID:   "user2",
		Username: "Alice",
		Role:     "member",
	})
	if err != nil {
		t.Fatalf("AssignMember failed: %v", err)
	}
	if member.Username != "Alice" {
		t.Errorf("expected username 'Alice', got '%s'", member.Username)
	}
}

func TestAssignMemberUpdate(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AssignMember(board.ID, &AssignMemberRequest{UserID: "u2", Username: "Bob", Role: "member"})

	// 更新角色
	updated, err := mgr.AssignMember(board.ID, &AssignMemberRequest{UserID: "u2", Username: "Bob", Role: "admin"})
	if err != nil {
		t.Fatalf("AssignMember update failed: %v", err)
	}
	if updated.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", updated.Role)
	}
}

func TestRemoveMember(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AssignMember(board.ID, &AssignMemberRequest{UserID: "u2", Username: "Bob", Role: "member"})

	err := mgr.RemoveMember(board.ID, "u2")
	if err != nil {
		t.Fatalf("RemoveMember failed: %v", err)
	}
}

func TestRemoveMemberNotFound(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	err := mgr.RemoveMember(board.ID, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent member")
	}
}

// ============================================================
// 活动记录测试
// ============================================================

func TestGetActivity(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})

	activities := mgr.GetActivity(board.ID, 10)
	if len(activities) < 2 {
		t.Errorf("expected at least 2 activities, got %d", len(activities))
	}
}

func TestGetActivityLimit(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card1", CreatedBy: "u1"})
	mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Card2", CreatedBy: "u1"})

	activities := mgr.GetActivity(board.ID, 1)
	if len(activities) != 1 {
		t.Errorf("expected 1 activity, got %d", len(activities))
	}
}

// ============================================================
// 边界情况测试
// ============================================================

func TestConcurrentAccess(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			mgr.AddCard(board.ID, &CreateCardRequest{
				ColumnID:  board.Columns[0].ID,
				Title:     fmt.Sprintf("Card %d", i),
				CreatedBy: "u1",
			})
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats, _ := mgr.GetBoardStats(board.ID)
	if stats.TotalCards != 10 {
		t.Errorf("expected 10 cards after concurrent add, got %d", stats.TotalCards)
	}
}

func TestCardLabelMultiple(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})
	label1, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Bug", Color: "#F00"})
	label2, _ := mgr.AddLabel(board.ID, &CreateLabelRequest{Name: "Feature", Color: "#0F0"})

	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{
		ColumnID:  board.Columns[0].ID,
		Title:     "Multi Label",
		LabelIDs:  []string{label1.ID, label2.ID},
		CreatedBy: "u1",
	})

	updated, _ := mgr.GetCard(board.ID, card.ID)
	if len(updated.LabelIDs) != 2 {
		t.Errorf("expected 2 labels, got %d", len(updated.LabelIDs))
	}
}

func TestBoardStatsEmpty(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Empty", OwnerID: "u1"})

	stats, err := mgr.GetBoardStats(board.ID)
	if err != nil {
		t.Fatalf("GetBoardStats failed: %v", err)
	}
	if stats.TotalCards != 0 {
		t.Errorf("expected 0 total cards, got %d", stats.TotalCards)
	}
}

func TestCardTimestamps(t *testing.T) {
	mgr := NewManager()

	board, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Test", OwnerID: "u1"})

	before := time.Now()
	card, _ := mgr.AddCard(board.ID, &CreateCardRequest{ColumnID: board.Columns[0].ID, Title: "Timestamp", CreatedBy: "u1"})
	after := time.Now()

	if card.CreatedAt.Before(before) || card.CreatedAt.After(after) {
		t.Error("CreatedAt not in expected range")
	}
	if card.UpdatedAt.Before(before) || card.UpdatedAt.After(after) {
		t.Error("UpdatedAt not in expected range")
	}
}

func TestHelperRemoveString(t *testing.T) {
	slice := []string{"a", "b", "c"}
	result := removeString(slice, "b")

	if len(result) != 2 {
		t.Errorf("expected 2 items, got %d", len(result))
	}
	if result[0] != "a" || result[1] != "c" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestHelperMatchFilter(t *testing.T) {
	card := &Card{
		Title:       "Test Card",
		Description: "A test card",
		AssigneeID:  "user1",
		Status:      CardStatusTodo,
		Priority:    PriorityHigh,
		LabelIDs:    []string{"label1"},
	}

	// 匹配
	if !matchFilter(card, &CardFilter{AssigneeID: "user1"}) {
		t.Error("expected match for assignee")
	}

	// 不匹配
	if matchFilter(card, &CardFilter{AssigneeID: "user2"}) {
		t.Error("expected no match for different assignee")
	}

	// 关键词匹配
	if !matchFilter(card, &CardFilter{Keyword: "test"}) {
		t.Error("expected keyword match")
	}

	// 标签匹配
	if !matchFilter(card, &CardFilter{LabelIDs: []string{"label1"}}) {
		t.Error("expected label match")
	}
}

func TestSearchCardsAllBoards(t *testing.T) {
	mgr := NewManager()

	board1, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Board1", OwnerID: "u1"})
	board2, _ := mgr.CreateBoard(&CreateBoardRequest{Name: "Board2", OwnerID: "u2"})

	mgr.AddCard(board1.ID, &CreateCardRequest{ColumnID: board1.Columns[0].ID, Title: "Card A", CreatedBy: "u1"})
	mgr.AddCard(board2.ID, &CreateCardRequest{ColumnID: board2.Columns[0].ID, Title: "Card B", CreatedBy: "u2"})

	// 搜索所有看板
	results := mgr.SearchCards(&CardFilter{})
	if len(results) != 2 {
		t.Errorf("expected 2 results across boards, got %d", len(results))
	}
}

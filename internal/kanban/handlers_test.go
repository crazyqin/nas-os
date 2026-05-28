package kanban

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *gin.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kanban.json")

	mgr := NewManager(configPath)
	handler := NewHandler(mgr)

	router := gin.New()
	api := router.Group("/api")
	handler.RegisterRoutes(api)

	return handler, mgr, router
}

func TestCreateBoard(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"name":"测试看板","description":"这是一个测试看板"}`
	req := httptest.NewRequest(http.MethodPost, "/api/kanban/boards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var board Board
	if err := json.Unmarshal(w.Body.Bytes(), &board); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if board.Name != "测试看板" {
		t.Errorf("expected name '测试看板', got '%s'", board.Name)
	}

	if len(board.Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(board.Columns))
	}

	if len(board.Members) != 1 {
		t.Errorf("expected 1 member, got %d", len(board.Members))
	}
}

func TestCreateBoardWithTemplate(t *testing.T) {
	_, _, router := setupTestHandler(t)

	body := `{"name":"Scrum看板","template_id":"scrum"}`
	req := httptest.NewRequest(http.MethodPost, "/api/kanban/boards", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var board Board
	json.Unmarshal(w.Body.Bytes(), &board)

	if len(board.Columns) != 4 {
		t.Errorf("expected 4 columns for scrum template, got %d", len(board.Columns))
	}
}

func TestListBoards(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	// Create two boards
	mgr.CreateBoard(CreateBoardRequest{Name: "看板1"}, "user1")
	mgr.CreateBoard(CreateBoardRequest{Name: "看板2"}, "user2")

	req := httptest.NewRequest(http.MethodGet, "/api/kanban/boards", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Boards []*Board `json:"boards"`
		Total  int      `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total != 2 {
		t.Errorf("expected 2 boards, got %d", resp.Total)
	}
}

func TestGetBoard(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "测试看板"}, "user1")

	req := httptest.NewRequest(http.MethodGet, "/api/kanban/boards/"+board.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Board
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.ID != board.ID {
		t.Errorf("expected board ID %s, got %s", board.ID, resp.ID)
	}
}

func TestGetBoardNotFound(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/kanban/boards/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestUpdateBoard(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "原名称"}, "user1")

	body := `{"name":"新名称","description":"新描述"}`
	req := httptest.NewRequest(http.MethodPut, "/api/kanban/boards/"+board.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp Board
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Name != "新名称" {
		t.Errorf("expected name '新名称', got '%s'", resp.Name)
	}
}

func TestDeleteBoard(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "待删除"}, "user1")

	req := httptest.NewRequest(http.MethodDelete, "/api/kanban/boards/"+board.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify deleted
	req2 := httptest.NewRequest(http.MethodGet, "/api/kanban/boards/"+board.ID, nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w2.Code)
	}
}

func TestCreateCard(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "测试看板"}, "user1")
	columnID := board.Columns[0].ID

	body := `{"column_id":"` + columnID + `","title":"测试卡片","description":"卡片描述","priority":"high"}`
	req := httptest.NewRequest(http.MethodPost, "/api/kanban/cards?board_id="+board.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var card Card
	json.Unmarshal(w.Body.Bytes(), &card)

	if card.Title != "测试卡片" {
		t.Errorf("expected title '测试卡片', got '%s'", card.Title)
	}
	if card.Priority != "high" {
		t.Errorf("expected priority 'high', got '%s'", card.Priority)
	}
}

func TestMoveCard(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "测试看板"}, "user1")
	columnID := board.Columns[0].ID
	targetColumnID := board.Columns[1].ID

	// Create a card
	card, _ := mgr.CreateCard(board.ID, CreateCardRequest{
		ColumnID: columnID,
		Title:    "待移动卡片",
	}, "user1")

	body := `{"target_column_id":"` + targetColumnID + `","position":0}`
	req := httptest.NewRequest(http.MethodPut, "/api/kanban/cards/"+card.ID+"/move?board_id="+board.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var movedCard Card
	json.Unmarshal(w.Body.Bytes(), &movedCard)

	if movedCard.ColumnID != targetColumnID {
		t.Errorf("expected column_id %s, got %s", targetColumnID, movedCard.ColumnID)
	}
}

func TestAddComment(t *testing.T) {
	_, mgr, router := setupTestHandler(t)

	board, _ := mgr.CreateBoard(CreateBoardRequest{Name: "测试看板"}, "user1")
	columnID := board.Columns[0].ID

	card, _ := mgr.CreateCard(board.ID, CreateCardRequest{
		ColumnID: columnID,
		Title:    "测试卡片",
	}, "user1")

	body := `{"content":"这是一条评论"}`
	req := httptest.NewRequest(http.MethodPost, "/api/kanban/cards/"+card.ID+"/comments?board_id="+board.ID, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var comment Comment
	json.Unmarshal(w.Body.Bytes(), &comment)

	if comment.Content != "这是一条评论" {
		t.Errorf("expected content '这是一条评论', got '%s'", comment.Content)
	}
}

func TestListTemplates(t *testing.T) {
	_, _, router := setupTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/kanban/templates", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Templates []*BoardTemplate `json:"templates"`
		Total     int              `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Total < 3 {
		t.Errorf("expected at least 3 templates, got %d", resp.Total)
	}
}

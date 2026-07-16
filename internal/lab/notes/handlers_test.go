// Package notes 提供 REST API 处理器测试
package notes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter() (*gin.Engine, *Manager) {
	gin.SetMode(gin.TestMode)
	mgr := NewManager()
	h := NewHandlers(mgr)

	r := gin.New()
	api := r.Group("/api")
	h.RegisterRoutes(api)

	return r, mgr
}

// TestCreateNote 测试创建笔记.
func TestCreateNote(t *testing.T) {
	r, _ := setupTestRouter()

	reqBody := CreateNoteRequest{
		Title:   "Test Note",
		Content: "# Hello World\n\nThis is a test note.",
		Author:  "testuser",
		Tags:    []string{"test", "demo"},
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/notes", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Test Note", data["title"])
	assert.Equal(t, "testuser", data["author"])
	assert.Equal(t, 1, int(data["version"].(float64)))
}

// TestUpdateNote 测试更新笔记.
func TestUpdateNote(t *testing.T) {
	r, mgr := setupTestRouter()

	// 先创建笔记
	note := mgr.CreateNote(CreateNoteRequest{
		Title:   "Original Title",
		Content: "Original content",
		Author:  "testuser",
	})

	// 更新笔记
	newTitle := "Updated Title"
	newContent := "Updated content with more words"
	reqBody := UpdateNoteRequest{
		Title:   &newTitle,
		Content: &newContent,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/notes/"+note.ID, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Updated Title", data["title"])
	assert.Equal(t, 2, int(data["version"].(float64)))
}

// TestSearchNotes 测试搜索笔记.
func TestSearchNotes(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建多个笔记
	mgr.CreateNote(CreateNoteRequest{
		Title:   "Go Programming",
		Content: "Go is a statically typed language",
		Author:  "user1",
	})

	mgr.CreateNote(CreateNoteRequest{
		Title:   "Python Tutorial",
		Content: "Python is a dynamically typed language",
		Author:  "user1",
	})

	mgr.CreateNote(CreateNoteRequest{
		Title:   "Web Development",
		Content: "Building web applications with Go and React",
		Author:  "user2",
	})

	// 搜索包含 "Go" 的笔记
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/notes/search?q=Go", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 2, int(data["total"].(float64)))
}

// TestCreateShareLink 测试创建分享链接.
func TestCreateShareLink(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建笔记
	note := mgr.CreateNote(CreateNoteRequest{
		Title:   "Shareable Note",
		Content: "This note will be shared",
		Author:  "testuser",
	})

	// 创建分享链接
	reqBody := CreateShareLinkRequest{
		Password:  "secret123",
		AllowEdit: false,
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/notes/"+note.ID+"/share", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.NotEmpty(t, data["token"])
	assert.Equal(t, "secret123", data["password"])
}

// TestNoteVersionHistory 测试笔记版本历史.
func TestNoteVersionHistory(t *testing.T) {
	r, mgr := setupTestRouter()

	// 创建笔记
	note := mgr.CreateNote(CreateNoteRequest{
		Title:   "Versioned Note",
		Content: "Version 1 content",
		Author:  "testuser",
	})

	// 更新两次
	title2 := "Version 2"
	content2 := "Version 2 content"
	mgr.UpdateNote(note.ID, UpdateNoteRequest{Title: &title2, Content: &content2})

	title3 := "Version 3"
	content3 := "Version 3 content"
	mgr.UpdateNote(note.ID, UpdateNoteRequest{Title: &title3, Content: &content3})

	// 获取版本历史
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/notes/"+note.ID+"/versions", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, 3, int(data["total"].(float64)))
}

// TestNotebooks 测试笔记本功能.
func TestNotebooks(t *testing.T) {
	r, _ := setupTestRouter()

	// 创建笔记本
	reqBody := CreateNotebookRequest{
		Name:        "Work Notes",
		Description: "Notes for work projects",
		Owner:       "testuser",
		Color:       "#FF5733",
	}
	body, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/notes/notebooks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, 0, resp.Code)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "Work Notes", data["name"])
	assert.Equal(t, "#FF5733", data["color"])
}

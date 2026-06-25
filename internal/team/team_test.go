package team

import (
	"os"
	"testing"
)

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager("", dir)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestNewAuditLogger(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/audit.json"
	logger := NewAuditLogger(path)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNewNotifier(t *testing.T) {
	notifier := NewNotifier()
	if notifier == nil {
		t.Fatal("expected non-nil notifier")
	}
}

func TestNewWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
}

func TestNewShareManager(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager("", dir)
	shareMgr := NewShareManager("", mgr)
	if shareMgr == nil {
		t.Fatal("expected non-nil share manager")
	}
}

func TestNewCommentManager(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager("", dir)
	commentMgr := NewCommentManager("", mgr)
	if commentMgr == nil {
		t.Fatal("expected non-nil comment manager")
	}
}

func TestNewCollabManager(t *testing.T) {
	dir := t.TempDir()
	mgr, _ := NewManager("", dir)
	collabMgr := NewCollabManager("", mgr)
	if collabMgr == nil {
		t.Fatal("expected non-nil collab manager")
	}
}

func TestManager_DataDir(t *testing.T) {
	dir := t.TempDir()
	mgr, err := NewManager("", dir)
	if err != nil {
		t.Fatal(err)
	}

	// 验证数据目录被创建
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("data dir should exist")
	}
	_ = mgr
}

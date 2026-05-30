package familydash

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestNewManagerNilLogger(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("NewManager with nil logger returned nil")
	}
}

func TestListMembers(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)
	members := mgr.ListMembers()
	if members == nil {
		t.Fatal("ListMembers returned nil")
	}
}

func TestCreateMember(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mgr := NewManager(logger)
	req := &CreateMemberRequest{
		Name: "测试成员",
	}
	member, err := mgr.CreateMember(req)
	if err != nil {
		t.Fatalf("CreateMember failed: %v", err)
	}
	if member == nil {
		t.Fatal("CreateMember returned nil")
	}
}

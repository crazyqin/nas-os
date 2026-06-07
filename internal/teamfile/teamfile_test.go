package teamfile

import (
	"testing"
)

func TestNewTeamFileManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewTeamFileManager(cfg)
	if mgr == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestCreateFolder(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, err := mgr.CreateFolder("技术文档", "技术团队共享文档", "team-eng", "/shared/tech-docs")
	if err != nil {
		t.Fatalf("create folder failed: %v", err)
	}
	if folder.Name != "技术文档" {
		t.Errorf("expected 技术文档, got %s", folder.Name)
	}
	if !folder.IsActive {
		t.Error("folder should be active")
	}
}

func TestDeleteFolder(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")

	if err := mgr.DeleteFolder(folder.ID); err != nil {
		t.Fatalf("delete folder failed: %v", err)
	}

	_, err := mgr.GetFolder(folder.ID)
	if err != ErrFolderNotFound {
		t.Errorf("expected ErrFolderNotFound, got %v", err)
	}
}

func TestGetFolder(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	_, err := mgr.GetFolder("nonexistent")
	if err != ErrFolderNotFound {
		t.Errorf("expected ErrFolderNotFound, got %v", err)
	}

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")
	got, err := mgr.GetFolder(folder.ID)
	if err != nil {
		t.Fatalf("get folder failed: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("expected test, got %s", got.Name)
	}
}

func TestListFolders(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	mgr.CreateFolder("folder1", "desc1", "team1", "/path1")
	mgr.CreateFolder("folder2", "desc2", "team2", "/path2")

	folders := mgr.ListFolders()
	if len(folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(folders))
	}
}

func TestAddMember(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")

	if err := mgr.AddMember(folder.ID, "user-001", RoleMember, PermWrite); err != nil {
		t.Fatalf("add member failed: %v", err)
	}

	// 重复添加
	if err := mgr.AddMember(folder.ID, "user-001", RoleMember, PermWrite); err != ErrMemberExists {
		t.Errorf("expected ErrMemberExists, got %v", err)
	}

	members, _ := mgr.GetMembers(folder.ID)
	if len(members) != 2 { // owner + new member
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestRemoveMember(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")
	mgr.AddMember(folder.ID, "user-001", RoleMember, PermWrite)

	if err := mgr.RemoveMember(folder.ID, "user-001"); err != nil {
		t.Fatalf("remove member failed: %v", err)
	}

	if err := mgr.RemoveMember(folder.ID, "user-001"); err != ErrMemberNotFound {
		t.Errorf("expected ErrMemberNotFound, got %v", err)
	}
}

func TestShareLink(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")

	link, err := mgr.CreateShareLink(folder.ID, "user-001", PermRead, 7)
	if err != nil {
		t.Fatalf("create share link failed: %v", err)
	}
	if link.Token == "" {
		t.Error("token should not be empty")
	}

	// 验证链接
	validated, err := mgr.ValidateShareLink(link.Token)
	if err != nil {
		t.Fatalf("validate link failed: %v", err)
	}
	if validated.ID != link.ID {
		t.Errorf("expected link ID %s, got %s", link.ID, validated.ID)
	}

	// 无效token
	_, err = mgr.ValidateShareLink("invalid-token")
	if err != ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestAuditLog(t *testing.T) {
	mgr := NewTeamFileManager(DefaultManagerConfig())

	folder, _ := mgr.CreateFolder("test", "test", "team", "/test")
	mgr.AddMember(folder.ID, "user-001", RoleMember, PermRead)

	logs := mgr.GetAuditLog(folder.ID)
	if len(logs) < 2 {
		t.Errorf("expected at least 2 audit logs, got %d", len(logs))
	}
}

func TestMaxFolders(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.MaxFolders = 2
	mgr := NewTeamFileManager(cfg)

	mgr.CreateFolder("f1", "d1", "t1", "/p1")
	mgr.CreateFolder("f2", "d2", "t2", "/p2")

	_, err := mgr.CreateFolder("f3", "d3", "t3", "/p3")
	if err != ErrMaxFoldersReached {
		t.Errorf("expected ErrMaxFoldersReached, got %v", err)
	}
}

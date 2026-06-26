package teamfile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTeam(t *testing.T) {
	m := NewManager()

	team, err := m.CreateTeam("user-001", &CreateTeamRequest{
		Name:        "研发团队",
		Description: "产品研发",
	})
	require.NoError(t, err)
	assert.Contains(t, team.ID, "team_")
	assert.Equal(t, "研发团队", team.Name)
	assert.Equal(t, "user-001", team.OwnerID)
	assert.Len(t, team.Members, 1)
	assert.Equal(t, RoleAdmin, team.Members[0].Role)
}

func TestCreateTeamInvalidInput(t *testing.T) {
	m := NewManager()

	_, err := m.CreateTeam("", &CreateTeamRequest{Name: "test"})
	assert.ErrorIs(t, err, ErrInvalidInput)

	_, err = m.CreateTeam("user-001", &CreateTeamRequest{Name: ""})
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestAddMember(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})

	err := m.AddMember("admin", team.ID, &AddMemberRequest{
		UserID: "editor1",
		Role:   RoleEditor,
	})
	require.NoError(t, err)

	err = m.AddMember("admin", team.ID, &AddMemberRequest{
		UserID: "viewer1",
		Role:   RoleViewer,
	})
	require.NoError(t, err)

	got, _ := m.GetTeam(team.ID)
	assert.Len(t, got.Members, 3)
}

func TestAddMemberPermissionDenied(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "viewer1", Role: RoleViewer})

	err := m.AddMember("viewer1", team.ID, &AddMemberRequest{UserID: "newuser", Role: RoleEditor})
	assert.ErrorIs(t, err, ErrPermissionDenied)
}

func TestAddMemberAlreadyExists(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})

	err := m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "admin", Role: RoleViewer})
	assert.ErrorIs(t, err, ErrAlreadyMember)
}

func TestRemoveMember(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "user1", Role: RoleEditor})

	err := m.RemoveMember("admin", team.ID, "user1")
	require.NoError(t, err)

	got, _ := m.GetTeam(team.ID)
	assert.Len(t, got.Members, 1)
}

func TestRemoveOwnerFails(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})

	err := m.RemoveMember("admin", team.ID, "admin")
	assert.ErrorIs(t, err, ErrRoleNotAllowed)
}

func TestUpdateMemberRole(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "user1", Role: RoleViewer})

	err := m.UpdateMemberRole("admin", team.ID, "user1", RoleEditor)
	require.NoError(t, err)

	got, _ := m.GetTeam(team.ID)
	for _, m := range got.Members {
		if m.UserID == "user1" {
			assert.Equal(t, RoleEditor, m.Role)
		}
	}
}

func TestShareFile(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "editor1", Role: RoleEditor})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "viewer1", Role: RoleViewer})

	sf, err := m.ShareFile("editor1", team.ID, &ShareFileRequest{
		Path:        "/shared/project-plan.md",
		Description: "项目计划",
	})
	require.NoError(t, err)
	assert.Equal(t, "/shared/project-plan.md", sf.Path)
	assert.Equal(t, "editor1", sf.SharedBy)

	// 只读用户不能共享文件
	_, err = m.ShareFile("viewer1", team.ID, &ShareFileRequest{Path: "/shared/secret.md"})
	assert.ErrorIs(t, err, ErrPermissionDenied)
}

func TestLockUnlockFile(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "editor1", Role: RoleEditor})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "editor2", Role: RoleEditor})

	sf, _ := m.ShareFile("editor1", team.ID, &ShareFileRequest{Path: "/shared/doc.md"})

	// editor1 锁定文件
	err := m.LockFile("editor1", team.ID, sf.ID, nil)
	require.NoError(t, err)

	got, _ := m.GetTeam(team.ID)
	assert.NotNil(t, got.Files[0].Lock)
	assert.Equal(t, "editor1", got.Files[0].Lock.LockedBy)

	// editor2 不能锁定已被 editor1 锁定的文件
	err = m.LockFile("editor2", team.ID, sf.ID, nil)
	assert.ErrorIs(t, err, ErrFileLocked)

	// editor1 解锁
	err = m.UnlockFile("editor1", team.ID, sf.ID)
	require.NoError(t, err)

	got, _ = m.GetTeam(team.ID)
	assert.Nil(t, got.Files[0].Lock)
}

func TestLockFileWithExpiry(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	sf, _ := m.ShareFile("admin", team.ID, &ShareFileRequest{Path: "/shared/doc.md"})

	dur := 30 * time.Minute
	err := m.LockFile("admin", team.ID, sf.ID, &LockFileRequest{Duration: &dur})
	require.NoError(t, err)

	got, _ := m.GetTeam(team.ID)
	assert.NotNil(t, got.Files[0].Lock.ExpiresAt)
}

func TestAdminCanUnlockOthers(t *testing.T) {
	m := NewManager()

	team, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team"})
	_ = m.AddMember("admin", team.ID, &AddMemberRequest{UserID: "editor1", Role: RoleEditor})

	sf, _ := m.ShareFile("editor1", team.ID, &ShareFileRequest{Path: "/shared/doc.md"})
	_ = m.LockFile("editor1", team.ID, sf.ID, nil)

	// 管理员可以解锁他人锁定的文件
	err := m.UnlockFile("admin", team.ID, sf.ID)
	require.NoError(t, err)
}

func TestListUserTeams(t *testing.T) {
	m := NewManager()

	team1, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team1"})
	team2, _ := m.CreateTeam("admin", &CreateTeamRequest{Name: "team2"})
	_ = m.AddMember("admin", team1.ID, &AddMemberRequest{UserID: "user1", Role: RoleViewer})
	_ = m.AddMember("admin", team2.ID, &AddMemberRequest{UserID: "user1", Role: RoleEditor})

	teams := m.ListUserTeams("user1")
	assert.Len(t, teams, 2)

	teams = m.ListUserTeams("admin")
	assert.Len(t, teams, 2)
}

func TestRoleMethods(t *testing.T) {
	assert.True(t, RoleAdmin.CanEdit())
	assert.True(t, RoleAdmin.CanAdmin())
	assert.True(t, RoleEditor.CanEdit())
	assert.False(t, RoleEditor.CanAdmin())
	assert.False(t, RoleViewer.CanEdit())
	assert.False(t, RoleViewer.CanAdmin())
}

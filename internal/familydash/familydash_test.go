package familydash

import (
	"testing"
)

func TestNewDashboard(t *testing.T) {
	cfg := &Config{
		Enabled: true,
	}
	dash := NewDashboard(cfg)
	if dash == nil {
		t.Fatal("NewDashboard returned nil")
	}
}

func TestNewDashboardNilConfig(t *testing.T) {
	dash := NewDashboard(nil)
	if dash == nil {
		t.Fatal("NewDashboard with nil config returned nil")
	}
}

func TestGetMembers(t *testing.T) {
	dash := NewDashboard(&Config{Enabled: true})
	members := dash.GetMembers()
	if members == nil {
		t.Fatal("GetMembers returned nil")
	}
}

func TestAddMember(t *testing.T) {
	dash := NewDashboard(&Config{Enabled: true})
	member := &Member{Name: "测试成员", Role: "parent"}
	err := dash.AddMember(member)
	if err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}
}

func TestGetDashboardStatus(t *testing.T) {
	dash := NewDashboard(&Config{Enabled: true})
	status := dash.GetStatus()
	if status == nil {
		t.Fatal("GetStatus returned nil")
	}
}

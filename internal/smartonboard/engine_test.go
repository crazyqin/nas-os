package smartonboard

import (
	"testing"
)

func TestNewSmartOnboard(t *testing.T) {
	ob := NewSmartOnboard()
	if ob == nil {
		t.Fatal("NewSmartOnboard returned nil")
	}
}

func TestCreateProfile(t *testing.T) {
	ob := NewSmartOnboard()
	profile := ob.CreateProfile("新NAS初始化")
	if profile == nil {
		t.Fatal("expected non-nil profile")
	}
	if profile.Name != "新NAS初始化" {
		t.Errorf("unexpected name: %s", profile.Name)
	}
	if len(profile.Steps) != 7 {
		t.Errorf("expected 7 steps, got %d", len(profile.Steps))
	}
	if profile.Progress != 0 {
		t.Errorf("expected 0%% progress, got %f", profile.Progress)
	}
}

func TestCompleteStep(t *testing.T) {
	ob := NewSmartOnboard()
	profile := ob.CreateProfile("测试")

	ok := ob.CompleteStep(profile.ID, "network")
	if !ok {
		t.Fatal("expected true")
	}

	profiles := ob.GetProfiles()
	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}
	// 1/7 ≈ 14.28%
	if profiles[0].Progress < 14 || profiles[0].Progress > 15 {
		t.Errorf("unexpected progress: %f", profiles[0].Progress)
	}
}

func TestSkipStep(t *testing.T) {
	ob := NewSmartOnboard()
	profile := ob.CreateProfile("测试")

	// required 步骤不能跳过
	ok := ob.SkipStep(profile.ID, "network")
	if ok {
		t.Fatal("should not skip required step")
	}

	// 非required可以跳过
	ok = ob.SkipStep(profile.ID, "apps")
	if !ok {
		t.Fatal("expected true for non-required step")
	}
}

func TestCheckHealth(t *testing.T) {
	ob := NewSmartOnboard()
	health := ob.CheckHealth()
	if health == nil {
		t.Fatal("expected non-nil health")
	}
	if health.Score < 0 || health.Score > 100 {
		t.Errorf("invalid score: %d", health.Score)
	}
}

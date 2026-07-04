package fileinsights

import (
	"testing"
	"time"

	"nas-os/internal/smartfolders"
)

func TestBuildProfileLargeFilesAndMediaActions(t *testing.T) {
	res := &smartfolders.Result{
		Scanned: 6,
		Items: []smartfolders.Item{
			{Name: "movie-a.mkv", Size: 5 << 20, Class: smartfolders.ClassVideo},
			{Name: "movie-b.mkv", Size: 6 << 20, Class: smartfolders.ClassVideo},
			{Name: "raw.bin", Size: 20 << 20, Class: smartfolders.ClassOther},
		},
		Summary: smartfolders.Summary{
			ByClass: map[smartfolders.FileClass]int{
				smartfolders.ClassPhoto: 3,
				smartfolders.ClassVideo: 2,
			},
			SizeByClass: map[smartfolders.FileClass]int64{
				smartfolders.ClassPhoto: 9 << 20,
				smartfolders.ClassVideo: 11 << 20,
			},
		},
	}
	advisor := NewAdvisor().WithThresholds(10<<20, 15<<20)
	advisor.now = func() time.Time { return time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC) }
	profile := advisor.BuildProfile(res)
	if profile.Scanned != 6 || len(profile.Actions) != 3 {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if profile.Actions[0].ID != "review-large-files" || profile.Actions[0].Severity != "warning" {
		t.Fatalf("large-file warning should be first: %+v", profile.Actions)
	}
}

func TestBuildProfileNilResult(t *testing.T) {
	profile := NewAdvisor().BuildProfile(nil)
	if profile.Scanned != 0 || len(profile.Actions) != 0 {
		t.Fatalf("nil result should produce empty profile: %+v", profile)
	}
}

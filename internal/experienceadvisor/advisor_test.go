package experienceadvisor

import (
	"testing"
	"time"
)

func TestRecommendPrioritizesStorageRisk(t *testing.T) {
	advisor := New(nil).WithNow(func() time.Time { return time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC) })

	recs := advisor.Recommend([]Signal{
		{Workload: WorkloadPhotos, ItemCount: 6000, Enabled: true},
		{Workload: WorkloadStorage, ErrorCount: 5, Enabled: true},
	})

	if len(recs) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	if recs[0].ID != "storage-snapshot-scrub" {
		t.Fatalf("expected storage risk first, got %s", recs[0].ID)
	}
	if recs[1].ID != "photos-ai-curation" {
		t.Fatalf("expected photo curation second, got %s", recs[1].ID)
	}
}

func TestRecommendSkipsDisabledSignals(t *testing.T) {
	advisor := New(nil)
	recs := advisor.Recommend([]Signal{{Workload: WorkloadPhotos, ItemCount: 9000, Enabled: false}})
	if len(recs) != 0 {
		t.Fatalf("expected disabled signal to be skipped, got %d", len(recs))
	}
}

func TestRecommendRemoteAccessRaisesPriorityOnErrors(t *testing.T) {
	advisor := New(&AdvisorConfig{LargePhotoLibraryCount: 10, LargeMediaLibraryGB: 10, BackupSizeGB: 10, MinActiveDevices: 2, HighErrorCount: 3, StaleDays: 14})
	recs := advisor.Recommend([]Signal{{Workload: WorkloadRemote, ActiveDevices: 1, ErrorCount: 4, Enabled: true}})
	if len(recs) != 1 {
		t.Fatalf("expected remote recommendation, got %d", len(recs))
	}
	if recs[0].Priority != 90 {
		t.Fatalf("expected priority 90, got %d", recs[0].Priority)
	}
}

func TestRecommendStaleApps(t *testing.T) {
	now := time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC)
	advisor := New(nil).WithNow(func() time.Time { return now })
	recs := advisor.Recommend([]Signal{{Workload: WorkloadApps, LastActivity: now.Add(-15 * 24 * time.Hour), Enabled: true}})
	if len(recs) != 1 || recs[0].ID != "apps-curation-cleanup" {
		t.Fatalf("expected stale app recommendation, got %#v", recs)
	}
}

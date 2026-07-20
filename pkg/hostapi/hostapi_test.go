package hostapi

import (
	"testing"
)

func TestAPIVersionNonEmpty(t *testing.T) {
	if APIVersion == "" {
		t.Fatal("APIVersion must be set")
	}
}

func TestTrustRankOrdering(t *testing.T) {
	if TrustLocal.Rank() >= TrustCommunity.Rank() {
		t.Fatal("local must rank below community")
	}
	if TrustCommunity.Rank() >= TrustSystem.Rank() {
		t.Fatal("community must rank below system")
	}
	if TrustSystem.Rank() >= TrustPlatform.Rank() {
		t.Fatal("system must rank below platform")
	}
}

func TestStaticHostPathsAndAllows(t *testing.T) {
	h := &StaticHost{
		Data:     "/var/lib/nas-os",
		Config:   "/etc/nas-os",
		MinTrust: TrustSystem,
	}
	if h.APIVersion() != APIVersion {
		t.Fatalf("APIVersion %s", h.APIVersion())
	}
	if h.DataPath("a", "b") != "/var/lib/nas-os/a/b" {
		t.Fatalf("DataPath: %s", h.DataPath("a", "b"))
	}
	if h.ConfigPath("x") != "/etc/nas-os/x" {
		t.Fatalf("ConfigPath: %s", h.ConfigPath("x"))
	}
	if !h.Allows(TrustSystem) {
		t.Fatal("system allowed")
	}
	if h.Allows(TrustCommunity) {
		t.Fatal("community below MinTrust system must be denied")
	}
}

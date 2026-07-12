package fecconfig

import (
	"testing"
	"time"
)

func TestAnalyze_FECUniversallyOff(t *testing.T) {
	recs := Analyze(Signal{
		FECUniversallyOff: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-enable-global" {
			found = true
			if r.Priority != "high" {
				t.Errorf("expected high, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected fec-enable-global recommendation")
	}
}

func TestAnalyze_LossWithFECOff(t *testing.T) {
	recs := Analyze(Signal{
		FECUniversallyOff:  true,
		TotalPacketLoss:    0.5,
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-loss-detected" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected fec-loss-detected recommendation")
	}
}

func TestAnalyze_HighSpeedNoFEC(t *testing.T) {
	recs := Analyze(Signal{
		Interfaces: []Interface{
			{Name: "eth0", SpeedGbps: 25, FECModeCurrent: FECNone, HasFECSupport: true},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-enable-eth0" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-enable-eth0 recommendation")
	}
}

func TestAnalyze_StorageNoFEC(t *testing.T) {
	recs := Analyze(Signal{
		Interfaces: []Interface{
			{Name: "eth1", SpeedGbps: 10, IsStorage: true, FECModeCurrent: FECNone, HasFECSupport: true},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-storage-eth1" {
			found = true
			if r.Priority != "critical" {
				t.Errorf("expected critical, got %s", r.Priority)
			}
		}
	}
	if !found {
		t.Error("expected fec-storage-eth1 recommendation")
	}
}

func TestAnalyze_LongCableNoFEC(t *testing.T) {
	recs := Analyze(Signal{
		Interfaces: []Interface{
			{Name: "eth2", SpeedGbps: 10, CableLengthM: 30, FECModeCurrent: FECNone, HasFECSupport: true},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-long-cable-eth2" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-long-cable-eth2 recommendation")
	}
}

func TestAnalyze_WiFiStorage(t *testing.T) {
	recs := Analyze(Signal{
		Interfaces: []Interface{
			{Name: "wlan0", LinkType: LinkWiFi, IsStorage: true},
		},
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-wifi-storage-wlan0" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-wifi-storage-wlan0 recommendation")
	}
}

func TestAnalyze_ProtocolErrors(t *testing.T) {
	recs := Analyze(Signal{
		ProtocolErrors: 200,
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-protocol-errors" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-protocol-errors recommendation")
	}
}

func TestAnalyze_StaleReview(t *testing.T) {
	recs := Analyze(Signal{
		LastFECReview: time.Now().Add(-100 * 24 * time.Hour),
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-stale-review" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-stale-review recommendation")
	}
}

func TestAnalyze_SameNIC(t *testing.T) {
	recs := Analyze(Signal{
		StorageOnSameNIC:  true,
		ReplicationActive: true,
	})
	found := false
	for _, r := range recs {
		if r.ID == "fec-separate-nics" {
			found = true
		}
	}
	if !found {
		t.Error("expected fec-separate-nics recommendation")
	}
}

func TestRecommendedFECMode(t *testing.T) {
	tests := []struct {
		speedGbps float64
		expected  FECMode
	}{
		{100, FECLDPC},
		{40, FECRS},
		{25, FECRS},
		{10, FECBCH},
		{1, FECHamming},
	}
	for _, tt := range tests {
		iface := Interface{SpeedGbps: tt.speedGbps}
		got := recommendedFECMode(iface)
		if got != tt.expected {
			t.Errorf("speed %.0fG: expected %s, got %s", tt.speedGbps, tt.expected, got)
		}
	}
}
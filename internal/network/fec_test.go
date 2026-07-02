package network

import "testing"

func TestRecommendFEC(t *testing.T) {
	cases := []struct {
		name    string
		speed   int
		latency bool
		want    FECMode
	}{
		{"400g", 400000, false, FECModeRS},
		{"25g throughput", 25000, false, FECModeRS},
		{"25g latency", 25000, true, FECModeBaseR},
		{"10g", 10000, false, FECModeBaseR},
		{"1g", 1000, false, FECModeAuto},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RecommendFEC(tc.speed, tc.latency); got != tc.want {
				t.Fatalf("RecommendFEC() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestConfigureFECValidatesMode(t *testing.T) {
	m := NewManager("")
	if _, err := m.ConfigureFEC("eth0", FECMode("bad"), false); err == nil {
		t.Fatal("expected invalid FEC mode error")
	}
	cfg, err := m.ConfigureFEC("eth0", FECModeAuto, true)
	if err != nil {
		t.Fatalf("ConfigureFEC() error = %v", err)
	}
	if cfg.Interface != "eth0" || cfg.Mode != FECModeAuto || !cfg.Persistent {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestBuildNetworkIntent(t *testing.T) {
	intent := BuildNetworkIntent("eth0", "fec", "admin", "enable rs fec")
	if intent.ID == "" || intent.Interface != "eth0" || intent.Operation != "fec" {
		t.Fatalf("unexpected intent: %+v", intent)
	}
}

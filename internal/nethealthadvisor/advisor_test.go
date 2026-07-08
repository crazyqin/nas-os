package nethealthadvisor

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateExpiredCert(t *testing.T) {
	advisor := New().WithNow(func() time.Time { return time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC) })
	report := advisor.Generate(Signal{
		InterfaceName:       "eth0",
		HasValidCert:        true,
		CertDaysLeft:        15,
		FirewallEnabled:     true,
		HasIPv6:             true,
		RemoteAccessEnabled: true,
		HasDDNS:             true,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "renew-expiring-certificate" {
			found = true
			if rec.Priority != "high" {
				t.Fatalf("cert priority = %s, want high (days >= 7)", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing renew-expiring-certificate recommendation")
	}
	if report.HealthStatus != "warning" {
		t.Fatalf("health = %s, want warning", report.HealthStatus)
	}
}

func TestGenerateCriticalCert(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:   "eth0",
		HasValidCert:    true,
		CertDaysLeft:    2,
		FirewallEnabled: true,
		HasIPv6:         true,
	})

	for _, rec := range report.Recommendations {
		if rec.ID == "renew-expiring-certificate" {
			if rec.Priority != "critical" {
				t.Fatalf("cert priority = %s, want critical (days < 7)", rec.Priority)
			}
		}
	}
	if report.HealthStatus != "critical" {
		t.Fatalf("health = %s, want critical", report.HealthStatus)
	}
}

func TestGenerateNoFirewall(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:   "eth0",
		FirewallEnabled: false,
		HasIPv6:         true,
		HasValidCert:    true,
		CertDaysLeft:    90,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "enable-firewall" {
			found = true
			if rec.Priority != "critical" {
				t.Fatalf("firewall priority = %s, want critical", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing enable-firewall recommendation")
	}
	if report.HealthStatus != "critical" {
		t.Fatalf("health = %s, want critical", report.HealthStatus)
	}
	if report.NetworkScore > 70 {
		t.Fatalf("score = %d, want <= 70 when firewall disabled", report.NetworkScore)
	}
}

func TestGenerateHighLatencyAndPacketLoss(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:     "eth0",
		FirewallEnabled:   true,
		HasIPv6:           true,
		HasValidCert:      true,
		CertDaysLeft:      90,
		PacketLossPercent: 3.5,
		LatencyMs:         80,
		JitterMs:          25,
	})

	wantIDs := map[string]bool{
		"investigate-packet-loss":   false,
		"optimize-network-latency":  false,
		"optimize-qos-for-jitter":   false,
	}
	for _, rec := range report.Recommendations {
		if _, ok := wantIDs[rec.ID]; ok {
			wantIDs[rec.ID] = true
		}
	}
	for id, seen := range wantIDs {
		if !seen {
			t.Fatalf("missing recommendation %s", id)
		}
	}
	if report.HealthStatus != "warning" {
		t.Fatalf("health = %s, want warning", report.HealthStatus)
	}
	if report.NetworkScore >= 70 {
		t.Fatalf("score = %d, want < 70 for poor network quality", report.NetworkScore)
	}
}

func TestGenerateHealthyNetwork(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:       "bond0",
		LinkSpeedMbps:       2000,
		MTU:                 1500,
		IsBonded:            true,
		BondSlaveCount:      2,
		HasIPv6:             true,
		HasDDNS:             true,
		HasValidCert:        true,
		CertDaysLeft:        90,
		FirewallEnabled:     true,
		UPnPEnabled:         false,
		RemoteAccessEnabled: true,
		FailedLoginAttempts: 0,
		ConcurrentUsers:     3,
		PacketLossPercent:   0,
		LatencyMs:           5,
		JitterMs:            2,
	})

	if report.HealthStatus != "healthy" {
		t.Fatalf("health = %s, want healthy", report.HealthStatus)
	}
	if report.NetworkScore < 95 {
		t.Fatalf("score = %d, want >= 95", report.NetworkScore)
	}
	if len(report.Recommendations) != 0 {
		t.Fatalf("recommendations = %#v, want none for healthy network", report.Recommendations)
	}
}

func TestGenerateNoDDNSWithRemoteAccess(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:       "eth0",
		FirewallEnabled:     true,
		HasIPv6:             true,
		HasValidCert:        true,
		CertDaysLeft:        90,
		RemoteAccessEnabled: true,
		HasDDNS:             false,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "configure-ddns-for-remote-access" {
			found = true
			if rec.Priority != "high" {
				t.Fatalf("ddns priority = %s, want high", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing configure-ddns-for-remote-access recommendation")
	}
}

func TestGenerateUPnPWithRemoteAccess(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:       "eth0",
		FirewallEnabled:     true,
		HasIPv6:             true,
		HasValidCert:        true,
		CertDaysLeft:        90,
		UPnPEnabled:         true,
		RemoteAccessEnabled: true,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "disable-upnp-use-manual-port-forwarding" {
			found = true
			if rec.Priority != "medium" {
				t.Fatalf("upnp priority = %s, want medium", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing disable-upnp-use-manual-port-forwarding recommendation")
	}
}

func TestGenerateLinkAggregationNeeded(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:   "eth0",
		FirewallEnabled: true,
		HasIPv6:         true,
		HasValidCert:    true,
		CertDaysLeft:    90,
		IsBonded:        false,
		ConcurrentUsers: 10,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "consider-link-aggregation" {
			found = true
			if rec.Priority != "medium" {
				t.Fatalf("bond priority = %s, want medium", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing consider-link-aggregation recommendation")
	}
}

func TestGenerateFailedLogins(t *testing.T) {
	report := New().Generate(Signal{
		InterfaceName:       "eth0",
		FirewallEnabled:     true,
		HasIPv6:             true,
		HasValidCert:        true,
		CertDaysLeft:        90,
		FailedLoginAttempts: 50,
	})

	var found bool
	for _, rec := range report.Recommendations {
		if rec.ID == "harden-login-security" {
			found = true
			if rec.Priority != "high" {
				t.Fatalf("login security priority = %s, want high", rec.Priority)
			}
		}
	}
	if !found {
		t.Fatalf("missing harden-login-security recommendation")
	}
	if report.HealthStatus != "warning" {
		t.Fatalf("health = %s, want warning", report.HealthStatus)
	}
}

func TestSummarizeActions(t *testing.T) {
	summary := SummarizeActions([]Recommendation{
		{Title: "启用防火墙", Actions: []string{"启用系统自带防火墙"}},
		{Title: "续期证书", Actions: []string{"申请续期"}},
	})
	if !strings.Contains(summary, "启用防火墙: 启用系统自带防火墙") {
		t.Fatalf("summary missing firewall action: %q", summary)
	}
	if !strings.Contains(summary, "续期证书: 申请续期") {
		t.Fatalf("summary missing cert action: %q", summary)
	}
	if !strings.Contains(summary, "; ") {
		t.Fatalf("summary should contain '; ' separator: %q", summary)
	}
}

func TestSummarizeActionsEmpty(t *testing.T) {
	summary := SummarizeActions(nil)
	if summary != "" {
		t.Fatalf("summary = %q, want empty string", summary)
	}
}

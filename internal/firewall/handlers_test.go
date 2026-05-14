package firewall

import (
	"testing"
	"time"
)

func TestAddAndGetRule(t *testing.T) {
	mgr := NewManager()
	rule := &Rule{
		ID:        "rule-1",
		Name:      "allow-http",
		Action:    ActionAllow,
		Protocol:  ProtoTCP,
		DstPort:   "80",
		Direction: DirInbound,
		Enabled:   true,
	}
	if err := mgr.AddRule(rule); err != nil {
		t.Fatalf("AddRule: %v", err)
	}
	got, err := mgr.GetRule("rule-1")
	if err != nil {
		t.Fatalf("GetRule: %v", err)
	}
	if got.Name != "allow-http" {
		t.Errorf("expected name 'allow-http', got %q", got.Name)
	}
	if got.HitCount != 0 {
		t.Errorf("expected hit_count 0, got %d", got.HitCount)
	}
}

func TestDeleteRule(t *testing.T) {
	mgr := NewManager()
	rule := &Rule{ID: "r1", Name: "test", Action: ActionDeny, Protocol: ProtoTCP, Direction: DirInbound}
	mgr.AddRule(rule)
	if err := mgr.DeleteRule("r1"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if _, err := mgr.GetRule("r1"); err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestDeleteRuleNotFound(t *testing.T) {
	mgr := NewManager()
	if err := mgr.DeleteRule("nonexistent"); err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}
}

func TestUpdateRule(t *testing.T) {
	mgr := NewManager()
	rule := &Rule{ID: "r1", Name: "old", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirBoth}
	mgr.AddRule(rule)
	rule.Name = "new"
	rule.Action = ActionDeny
	if err := mgr.UpdateRule("r1", rule); err != nil {
		t.Fatalf("UpdateRule: %v", err)
	}
	got, _ := mgr.GetRule("r1")
	if got.Name != "new" {
		t.Errorf("expected 'new', got %q", got.Name)
	}
	if got.Action != ActionDeny {
		t.Errorf("expected deny, got %s", got.Action)
	}
}

func TestEnableDisableRule(t *testing.T) {
	mgr := NewManager()
	rule := &Rule{ID: "r1", Name: "test", Enabled: true, Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound}
	mgr.AddRule(rule)
	mgr.DisableRule("r1")
	got, _ := mgr.GetRule("r1")
	if got.Enabled {
		t.Error("expected rule to be disabled")
	}
	mgr.EnableRule("r1")
	got, _ = mgr.GetRule("r1")
	if !got.Enabled {
		t.Error("expected rule to be enabled")
	}
}

func TestListRules(t *testing.T) {
	mgr := NewManager()
	mgr.AddRule(&Rule{ID: "r1", Name: "a", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound})
	mgr.AddRule(&Rule{ID: "r2", Name: "b", Action: ActionDeny, Protocol: ProtoUDP, Direction: DirOutbound})
	rules := mgr.ListRules()
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
}

func TestMaxRulesLimit(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxRules = 2
	mgr.AddRule(&Rule{ID: "r1", Name: "a", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound})
	mgr.AddRule(&Rule{ID: "r2", Name: "b", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound})
	err := mgr.AddRule(&Rule{ID: "r3", Name: "c", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound})
	if err != ErrMaxRulesReached {
		t.Errorf("expected ErrMaxRulesReached, got %v", err)
	}
}

func TestZones(t *testing.T) {
	mgr := NewManager()
	zone := &Zone{
		Name:          "lan",
		Description:   "Local Network",
		Interfaces:    []string{"eth0"},
		DefaultAction: ActionAllow,
	}
	mgr.AddZone(zone)
	got, ok := mgr.GetZone("lan")
	if !ok {
		t.Fatal("expected zone 'lan' to exist")
	}
	if got.DefaultAction != ActionAllow {
		t.Errorf("expected allow, got %s", got.DefaultAction)
	}
}

func TestStats(t *testing.T) {
	mgr := NewManager()
	mgr.AddRule(&Rule{ID: "r1", Name: "a", Enabled: true, Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound})
	mgr.AddRule(&Rule{ID: "r2", Name: "b", Enabled: false, Action: ActionDeny, Protocol: ProtoUDP, Direction: DirOutbound})
	stats := mgr.GetStats()
	if stats.TotalRules != 2 {
		t.Errorf("expected 2 total rules, got %d", stats.TotalRules)
	}
	if stats.EnabledRules != 1 {
		t.Errorf("expected 1 enabled rule, got %d", stats.EnabledRules)
	}
}

func TestTrafficLog(t *testing.T) {
	mgr := NewManager()
	// TrafficLog is populated internally; test retrieval
	logs := mgr.GetTrafficLog(10)
	if len(logs) != 0 {
		t.Errorf("expected empty log, got %d entries", len(logs))
	}
}

func TestConfig(t *testing.T) {
	mgr := NewManager()
	cfg := mgr.GetConfig()
	if !cfg.Enabled {
		t.Error("expected firewall to be enabled by default")
	}
	if cfg.DefaultIn != ActionDeny {
		t.Errorf("expected default_in deny, got %s", cfg.DefaultIn)
	}
	cfg.DefaultIn = ActionAllow
	mgr.UpdateConfig(cfg)
	got := mgr.GetConfig()
	if got.DefaultIn != ActionAllow {
		t.Errorf("expected default_in allow after update, got %s", got.DefaultIn)
	}
}

func TestRuleTimestamps(t *testing.T) {
	mgr := NewManager()
	before := time.Now()
	rule := &Rule{ID: "r1", Name: "test", Action: ActionAllow, Protocol: ProtoTCP, Direction: DirInbound}
	mgr.AddRule(rule)
	got, _ := mgr.GetRule("r1")
	if got.CreatedAt.Before(before) {
		t.Error("CreatedAt should be set")
	}
	if got.UpdatedAt.Before(before) {
		t.Error("UpdatedAt should be set")
	}
}

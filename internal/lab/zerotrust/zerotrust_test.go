// Package zerotrust 零信任安全模块单元测试
package zerotrust

import (
	"testing"
	"time"
)

func TestPolicyEngine_AddAndGetPolicy(t *testing.T) {
	pe := NewPolicyEngine()
	p := &SecurityPolicy{ID: "p1", Name: "admin", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Users: []string{"admin"}}}
	if err := pe.AddPolicy(p); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	got, err := pe.GetPolicy("p1")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if got.Name != "admin" {
		t.Errorf("name: %s", got.Name)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt zero")
	}
}

func TestPolicyEngine_AddPolicy_Validation(t *testing.T) {
	pe := NewPolicyEngine()
	if err := pe.AddPolicy(&SecurityPolicy{Name: "x"}); err == nil {
		t.Error("reject empty ID")
	}
	if err := pe.AddPolicy(&SecurityPolicy{ID: "p1"}); err == nil {
		t.Error("reject empty name")
	}
}

func TestPolicyEngine_RemovePolicy(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "p1", Name: "t"})
	if err := pe.RemovePolicy("p1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := pe.GetPolicy("p1"); err == nil {
		t.Error("should be removed")
	}
	if err := pe.RemovePolicy("x"); err == nil {
		t.Error("remove nonexistent")
	}
}

func TestPolicyEngine_ListPolicies_Sorted(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "p3", Name: "l", Priority: 30, Enabled: true})
	pe.AddPolicy(&SecurityPolicy{ID: "p1", Name: "h", Priority: 10, Enabled: true})
	pe.AddPolicy(&SecurityPolicy{ID: "p2", Name: "m", Priority: 20, Enabled: true})
	p := pe.ListPolicies()
	if len(p) != 3 {
		t.Fatalf("want 3, got %d", len(p))
	}
	if p[0].Priority != 10 || p[1].Priority != 20 || p[2].Priority != 30 {
		t.Error("not sorted")
	}
}

func TestPolicyEngine_Evaluate_AllowByUser(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "a", Name: "a", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Users: []string{"admin"}}})
	d := pe.Evaluate(ZTAccessRequest{UserID: "admin", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if !d.Allowed {
		t.Error("admin should be allowed")
	}
}

func TestPolicyEngine_Evaluate_DefaultDeny(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "a", Name: "a", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Users: []string{"admin"}}})
	d := pe.Evaluate(ZTAccessRequest{UserID: "guest", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if d.Allowed {
		t.Error("guest should be denied")
	}
	if d.PolicyID != "default" {
		t.Errorf("policy: %s", d.PolicyID)
	}
}

func TestPolicyEngine_Evaluate_DisabledPolicy(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "p1", Name: "d", Enabled: false, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Users: []string{"admin"}}})
	d := pe.Evaluate(ZTAccessRequest{UserID: "admin", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if d.Allowed {
		t.Error("disabled policy should not apply")
	}
}

func TestPolicyEngine_Evaluate_ByNetwork(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "n", Name: "n", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Networks: []string{"192.168.1.0/24"}}})
	r := ZTAccessRequest{UserID: "u", Resource: "/d", Action: "read", Timestamp: time.Now()}
	r.IP = "192.168.1.100"
	if d := pe.Evaluate(r); !d.Allowed {
		t.Error("internal IP allowed")
	}
	r.IP = "10.0.0.1"
	if d := pe.Evaluate(r); d.Allowed {
		t.Error("external IP denied")
	}
}

func TestPolicyEngine_Evaluate_ByTimeRange(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "t", Name: "t", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{TimeStart: "09:00", TimeEnd: "18:00"}})
	r := ZTAccessRequest{UserID: "u", Resource: "/d", Action: "read"}
	r.Timestamp = time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC)
	if d := pe.Evaluate(r); !d.Allowed {
		t.Error("work hours allowed")
	}
	r.Timestamp = time.Date(2025, 1, 15, 22, 0, 0, 0, time.UTC)
	if d := pe.Evaluate(r); d.Allowed {
		t.Error("off hours denied")
	}
}

func TestPolicyEngine_Evaluate_ByResourcePattern(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "r", Name: "r", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Resources: []string{"/api/*"}}})
	r := ZTAccessRequest{UserID: "u", Action: "read", Timestamp: time.Now()}
	r.Resource = "/api/users"
	if d := pe.Evaluate(r); !d.Allowed {
		t.Error("/api/users allowed")
	}
	r.Resource = "/admin/x"
	if d := pe.Evaluate(r); d.Allowed {
		t.Error("/admin/x denied")
	}
}

func TestPolicyEngine_Evaluate_ChallengeEffect(t *testing.T) {
	pe := NewPolicyEngine()
	pe.AddPolicy(&SecurityPolicy{ID: "c", Name: "c", Enabled: true, Priority: 1, Effect: PolicyChallenge,
		Conditions: PolicyCondition{Users: []string{"u1"}}})
	d := pe.Evaluate(ZTAccessRequest{UserID: "u1", Resource: "/s", Action: "read", Timestamp: time.Now()})
	if d.Allowed {
		t.Error("challenge not allowed")
	}
	if d.Effect != PolicyChallenge {
		t.Errorf("effect: %s", d.Effect)
	}
}

func TestDeviceTrustManager_RegisterAndGet(t *testing.T) {
	dtm := NewDeviceTrustManager()
	d := &DeviceInfo{ID: "dev1", Name: "test", Fingerprint: "abc", IP: "1.2.3.4"}
	if err := dtm.RegisterDevice(d); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, _ := dtm.GetDevice("dev1")
	if got.Name != "test" {
		t.Errorf("name: %s", got.Name)
	}
	if got.LastSeen.IsZero() {
		t.Error("LastSeen zero")
	}
	if got.TrustLevel != ZTTrustLevelLow {
		t.Errorf("trust: %s", got.TrustLevel)
	}
}

func TestDeviceTrustManager_RegisterValidation(t *testing.T) {
	dtm := NewDeviceTrustManager()
	if err := dtm.RegisterDevice(&DeviceInfo{Fingerprint: "fp"}); err == nil {
		t.Error("reject empty ID")
	}
	if err := dtm.RegisterDevice(&DeviceInfo{ID: "d"}); err == nil {
		t.Error("reject empty fp")
	}
}

func TestDeviceTrustManager_DuplicateFingerprint(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d1", Fingerprint: "fp"})
	if err := dtm.RegisterDevice(&DeviceInfo{ID: "d2", Name: "d2", Fingerprint: "fp"}); err == nil {
		t.Error("dup")
	}
	if err := dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d1u", Fingerprint: "fp"}); err != nil {
		t.Error("re-reg ok")
	}
}

func TestDeviceTrustManager_Unregister(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp"})
	if err := dtm.UnregisterDevice("d1"); err != nil {
		t.Fatalf("unreg: %v", err)
	}
	if _, err := dtm.GetDevice("d1"); err == nil {
		t.Error("should be gone")
	}
	if err := dtm.UnregisterDevice("x"); err == nil {
		t.Error("unreg nonexistent")
	}
}

func TestDeviceTrustManager_CheckCompliance(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp",
		Compliance: DeviceCompliance{OSUpdated: true, FirewallEnabled: true, AntivirusActive: true, DiskEncrypted: true, PasswordStrong: true}})
	comp, _ := dtm.CheckCompliance("d1")
	if comp.ComplianceScore != 100 {
		t.Errorf("score: %.0f", comp.ComplianceScore)
	}
	d, _ := dtm.GetDevice("d1")
	if d.TrustLevel != ZTTrustLevelHigh {
		t.Errorf("trust: %s", d.TrustLevel)
	}
}

func TestDeviceTrustManager_CheckCompliance_LowScore(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp"})
	comp, _ := dtm.CheckCompliance("d1")
	if comp.ComplianceScore != 0 {
		t.Errorf("score: %.0f", comp.ComplianceScore)
	}
	d, _ := dtm.GetDevice("d1")
	if d.TrustLevel != ZTTrustLevelUntrusted {
		t.Errorf("trust: %s", d.TrustLevel)
	}
}

func TestGenerateFingerprint(t *testing.T) {
	fp1 := GenerateFingerprint(map[string]string{"os": "linux", "browser": "chrome"})
	fp2 := GenerateFingerprint(map[string]string{"browser": "chrome", "os": "linux"})
	if fp1 != fp2 {
		t.Error("order should not matter")
	}
	if len(fp1) != 64 {
		t.Errorf("len: %d", len(fp1))
	}
	fp3 := GenerateFingerprint(map[string]string{"os": "windows"})
	if fp1 == fp3 {
		t.Error("different should differ")
	}
}

func TestListDevices(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "c", Name: "c", Fingerprint: "fpc"})
	dtm.RegisterDevice(&DeviceInfo{ID: "a", Name: "a", Fingerprint: "fpa"})
	dtm.RegisterDevice(&DeviceInfo{ID: "b", Name: "b", Fingerprint: "fpb"})
	d := dtm.ListDevices()
	if len(d) != 3 {
		t.Fatalf("want 3")
	}
	if d[0].ID != "a" || d[2].ID != "c" {
		t.Error("not sorted")
	}
}

func TestContinuousAuth_CreateAndGetSession(t *testing.T) {
	ca := NewContinuousAuth()
	s, err := ca.CreateSession("u1", "d1", "1.2.3.4", "bj")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.UserID != "u1" {
		t.Errorf("user: %s", s.UserID)
	}
	if !s.Active {
		t.Error("should be active")
	}
	if s.TrustLevel != ZTTrustLevelMedium {
		t.Errorf("trust: %s", s.TrustLevel)
	}
}

func TestContinuousAuth_EmptyUser(t *testing.T) {
	ca := NewContinuousAuth()
	if _, err := ca.CreateSession("", "d", "1.2.3.4", "l"); err == nil {
		t.Error("reject empty")
	}
}

func TestContinuousAuth_EndSession(t *testing.T) {
	ca := NewContinuousAuth()
	s, _ := ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	if err := ca.EndSession(s.ID); err != nil {
		t.Fatalf("end: %v", err)
	}
	got, _ := ca.GetSession(s.ID)
	if got.Active {
		t.Error("should be ended")
	}
	if err := ca.EndSession("x"); err == nil {
		t.Error("end nonexistent")
	}
}

func TestContinuousAuth_RecordActivity_RiskScore(t *testing.T) {
	ca := NewContinuousAuth()
	s, _ := ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	for i := 0; i < 6; i++ {
		ca.RecordActivity(s.ID, Activity{Type: "access", Success: false, IP: "1.2.3.4", Location: "l"})
	}
	got, _ := ca.GetSession(s.ID)
	if got.RiskScore <= 0 {
		t.Error("risk should rise")
	}
}

func TestContinuousAuth_BlockedIP(t *testing.T) {
	ca := NewContinuousAuth()
	for i := 0; i < 5; i++ {
		ca.RecordFailedAttempt("10.0.0.1")
	}
	if _, err := ca.CreateSession("u", "d", "10.0.0.1", "l"); err == nil {
		t.Error("blocked IP")
	}
}

func TestContinuousAuth_GetUserSessions(t *testing.T) {
	ca := NewContinuousAuth()
	ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	ca.CreateSession("u1", "d2", "5.6.7.8", "l")
	if len(ca.GetUserSessions("u1")) != 2 {
		t.Error("want 2")
	}
	if len(ca.GetUserSessions("nobody")) != 0 {
		t.Error("want 0")
	}
}

func TestContinuousAuth_CleanupExpired(t *testing.T) {
	ca := NewContinuousAuth()
	s, _ := ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	ca.EndSession(s.ID)
	ca.mu.Lock()
	ca.sessions[s.ID].LastActivity = time.Now().Add(-2 * time.Hour)
	ca.mu.Unlock()
	if c := ca.CleanupExpiredSessions(); c != 1 {
		t.Errorf("want 1, got %d", c)
	}
}

func TestContinuousAuth_RecordOnEndedSession(t *testing.T) {
	ca := NewContinuousAuth()
	s, _ := ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	ca.EndSession(s.ID)
	if err := ca.RecordActivity(s.ID, Activity{Type: "a", Success: true}); err == nil {
		t.Error("ended")
	}
}

func TestMicroSegmentManager_AddSegment(t *testing.T) {
	msm := NewMicroSegmentManager()
	if err := msm.AddSegment(&ZTNetworkSegment{ID: "i", Name: "int", Subnets: []string{"192.168.1.0/24"}}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := msm.AddSegment(&ZTNetworkSegment{ID: "b", Subnets: []string{"bad"}}); err == nil {
		t.Error("bad CIDR")
	}
	if err := msm.AddSegment(&ZTNetworkSegment{Subnets: []string{"10.0.0.0/8"}}); err == nil {
		t.Error("empty ID")
	}
}

func TestMicroSegmentManager_RemoveSegment(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "s1", Subnets: []string{"10.0.0.0/8"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "s2", Subnets: []string{"192.168.0.0/16"}})
	msm.AddAccessRule(&ZTAccessRule{ID: "r1", SourceSeg: "s1", DestSeg: "s2", Protocol: "tcp", Enabled: true})
	if err := msm.RemoveSegment("s1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(msm.ListAccessRules()) != 0 {
		t.Error("rules cleaned")
	}
	if err := msm.RemoveSegment("x"); err == nil {
		t.Error("nonexistent")
	}
}

func TestMicroSegmentManager_AccessRules(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "d", Subnets: []string{"10.0.1.0/24"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "i", Subnets: []string{"192.168.1.0/24"}})
	if err := msm.AddAccessRule(&ZTAccessRule{ID: "r1", SourceSeg: "d", DestSeg: "i", Ports: []int{443}, Protocol: "tcp", Effect: PolicyAllow, Enabled: true}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := msm.AddAccessRule(&ZTAccessRule{ID: "r2", SourceSeg: "x", DestSeg: "i"}); err == nil {
		t.Error("bad seg")
	}
	if err := msm.RemoveAccessRule("r1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := msm.RemoveAccessRule("x"); err == nil {
		t.Error("nonexistent")
	}
}

func TestMicroSegmentManager_CheckAccess_SameSegment(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "lan", Subnets: []string{"192.168.1.0/24"}})
	if e, _ := msm.CheckAccess("192.168.1.10", "192.168.1.20", 80, "tcp"); e != PolicyAllow {
		t.Error("same seg")
	}
}

func TestMicroSegmentManager_CheckAccess_CrossSegment(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "d", Subnets: []string{"10.0.1.0/24"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "i", Subnets: []string{"192.168.1.0/24"}})
	msm.AddAccessRule(&ZTAccessRule{ID: "r1", SourceSeg: "d", DestSeg: "i", Ports: []int{443}, Protocol: "tcp", Effect: PolicyAllow, Enabled: true})
	if e, _ := msm.CheckAccess("10.0.1.5", "192.168.1.10", 443, "tcp"); e != PolicyAllow {
		t.Error("443 ok")
	}
	if e, _ := msm.CheckAccess("10.0.1.5", "192.168.1.10", 22, "tcp"); e != PolicyDeny {
		t.Error("22 deny")
	}
	if e, _ := msm.CheckAccess("10.0.1.5", "192.168.1.10", 443, "udp"); e != PolicyDeny {
		t.Error("udp deny")
	}
}

func TestMicroSegmentManager_CheckAccess_UnknownIP(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "lan", Subnets: []string{"192.168.1.0/24"}})
	if e, _ := msm.CheckAccess("10.0.0.1", "192.168.1.10", 80, "tcp"); e != PolicyDeny {
		t.Error("unknown")
	}
}

func TestMicroSegmentManager_DisabledRule(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "s1", Subnets: []string{"10.0.1.0/24"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "s2", Subnets: []string{"10.0.2.0/24"}})
	msm.AddAccessRule(&ZTAccessRule{ID: "r1", SourceSeg: "s1", DestSeg: "s2", Protocol: "any", Effect: PolicyAllow, Enabled: false})
	if e, _ := msm.CheckAccess("10.0.1.1", "10.0.2.1", 80, "tcp"); e != PolicyDeny {
		t.Error("disabled")
	}
}

func TestThreatDetector_DetectBruteForce(t *testing.T) {
	td := NewThreatDetector()
	for i := 0; i < 4; i++ {
		if ev := td.DetectBruteForce("10.0.0.1", false); ev != nil {
			t.Error("4 fails")
		}
	}
	ev := td.DetectBruteForce("10.0.0.1", false)
	if ev == nil {
		t.Fatal("5 fails trigger")
	}
	if ev.Severity != SeverityHigh {
		t.Errorf("sev: %s", ev.Severity)
	}
	if ev.Action != ActionBlock {
		t.Errorf("act: %s", ev.Action)
	}
}

func TestThreatDetector_DetectBruteForce_SuccessDoesNotTrigger(t *testing.T) {
	td := NewThreatDetector()
	for i := 0; i < 10; i++ {
		td.DetectBruteForce("10.0.0.1", true)
	}
	if ev := td.DetectBruteForce("10.0.0.1", true); ev != nil {
		t.Error("success")
	}
}

func TestThreatDetector_DetectAbnormalLogin(t *testing.T) {
	td := NewThreatDetector()
	if ev := td.DetectAbnormalLogin("u1", "1.2.3.4", "bj"); ev != nil {
		t.Error("1 loc")
	}
	if ev := td.DetectAbnormalLogin("u1", "5.6.7.8", "sh"); ev != nil {
		t.Error("2 loc")
	}
	ev := td.DetectAbnormalLogin("u1", "9.10.11.12", "gz")
	if ev == nil {
		t.Fatal("3 loc trigger")
	}
	if ev.Severity != SeverityHigh {
		t.Errorf("sev: %s", ev.Severity)
	}
}

func TestThreatDetector_DetectSQLInjection(t *testing.T) {
	td := NewThreatDetector()
	if ev := td.DetectSQLInjection("hello", "w"); ev != nil {
		t.Error("normal")
	}
	tests := []struct{ i, n string }{{"' OR '1'='1", "or"}, {"1; DROP TABLE x", "drop"}, {"1 UNION SELECT * FROM x", "union"}}
	for _, tc := range tests {
		ev := td.DetectSQLInjection(tc.i, "w")
		if ev == nil {
			t.Errorf("miss: %s", tc.n)
		}
		if ev != nil && ev.Severity != SeverityCritical {
			t.Errorf("sev: %s", ev.Severity)
		}
	}
}

func TestThreatDetector_DetectXSS(t *testing.T) {
	td := NewThreatDetector()
	if ev := td.DetectXSS("normal", "w"); ev != nil {
		t.Error("normal")
	}
	tests := []struct{ i, n string }{{"<script>x</script>", "script"}, {"javascript:x", "js"}, {"<img onerror=x>", "onerror"}, {"eval(x)", "eval"}}
	for _, tc := range tests {
		ev := td.DetectXSS(tc.i, "w")
		if ev == nil {
			t.Errorf("miss: %s", tc.n)
		}
		if ev != nil && ev.Severity != SeverityHigh {
			t.Errorf("sev: %s", ev.Severity)
		}
	}
}

func TestThreatDetector_GetEvents(t *testing.T) {
	td := NewThreatDetector()
	td.DetectSQLInjection("' OR 1=1--", "w")
	td.DetectXSS("<script>x</script>", "w")
	if len(td.GetEvents(10)) != 2 {
		t.Error("want 2")
	}
	if len(td.GetEventsByType("sql_injection", 10)) != 1 {
		t.Error("want 1")
	}
}

func TestSecurityEventManager_RecordAndGet(t *testing.T) {
	sem := NewSecurityEventManager()
	sem.RecordEvent(&SecurityEvent{Type: "login", Severity: SeverityInfo, Source: "u1"})
	if len(sem.GetEvents(10, nil)) != 1 {
		t.Error("want 1")
	}
}

func TestSecurityEventManager_HighSeverityCreatesAlert(t *testing.T) {
	sem := NewSecurityEventManager()
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityHigh, Source: "s"})
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityInfo, Source: "s"})
	if len(sem.GetAlerts(10, nil)) != 1 {
		t.Error("want 1 alert")
	}
}

func TestSecurityEventManager_ResolveEvent(t *testing.T) {
	sem := NewSecurityEventManager()
	sem.RecordEvent(&SecurityEvent{ID: "ev1", Type: "t", Severity: SeverityHigh, Source: "s"})
	if err := sem.ResolveEvent("ev1", "admin"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	events := sem.GetEvents(10, nil)
	if !events[0].Resolved {
		t.Error("resolved")
	}
	if events[0].ResolvedBy != "admin" {
		t.Errorf("by: %s", events[0].ResolvedBy)
	}
	if events[0].ResolvedAt == nil {
		t.Error("at nil")
	}
	if err := sem.ResolveEvent("x", "a"); err == nil {
		t.Error("nonexistent")
	}
}

func TestSecurityEventManager_GetEventStats(t *testing.T) {
	sem := NewSecurityEventManager()
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityInfo, Source: "s"})
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityHigh, Source: "s"})
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityCritical, Source: "s"})
	st := sem.GetEventStats()
	if st["total"] != 3 {
		t.Errorf("total: %d", st["total"])
	}
	if st["high"] != 1 {
		t.Errorf("high: %d", st["high"])
	}
	if st["critical"] != 1 {
		t.Errorf("crit: %d", st["critical"])
	}
}

func TestComplianceReporter_GenerateReport(t *testing.T) {
	ztm := NewZeroTrustManager()
	ztm.PolicyEngine.AddPolicy(&SecurityPolicy{ID: "p1", Name: "t", Enabled: true, Priority: 1, Effect: PolicyAllow})
	ztm.DeviceManager.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp1",
		Compliance: DeviceCompliance{OSUpdated: true, FirewallEnabled: true, AntivirusActive: true, DiskEncrypted: true, PasswordStrong: true}})
	ztm.SegmentManager.AddSegment(&ZTNetworkSegment{ID: "s1", Subnets: []string{"10.0.0.0/8"}})
	report := ztm.Reporter.GenerateReport("月报", time.Now().AddDate(0, -1, 0), time.Now())
	if report.Title != "月报" {
		t.Errorf("title: %s", report.Title)
	}
	if len(report.Sections) != 5 {
		t.Errorf("sections: %d", len(report.Sections))
	}
	if report.Summary.ComplianceScore <= 0 {
		t.Error("score > 0")
	}
}

func TestComplianceReporter_ExportJSON(t *testing.T) {
	ztm := NewZeroTrustManager()
	report := ztm.Reporter.GenerateReport("test", time.Now(), time.Now())
	data, err := ztm.Reporter.ExportReportJSON(report)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(data) == 0 {
		t.Error("empty")
	}
}

func TestComplianceReporter_Recommendations(t *testing.T) {
	ztm := NewZeroTrustManager()
	report := ztm.Reporter.GenerateReport("test", time.Now(), time.Now())
	if len(report.Recommendations) == 0 {
		t.Error("should have recs")
	}
}

func TestZeroTrustManager_ProcessAccessRequest(t *testing.T) {
	ztm := NewZeroTrustManager()
	ztm.PolicyEngine.AddPolicy(&SecurityPolicy{ID: "a", Name: "a", Enabled: true, Priority: 1, Effect: PolicyAllow,
		Conditions: PolicyCondition{Users: []string{"admin"}}})
	d := ztm.ProcessAccessRequest(ZTAccessRequest{UserID: "admin", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if !d.Allowed {
		t.Error("admin allowed")
	}
	d = ztm.ProcessAccessRequest(ZTAccessRequest{UserID: "guest", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if d.Allowed {
		t.Error("guest denied")
	}
}

func TestZeroTrustManager_ProcessAccessRequest_DeviceTrust(t *testing.T) {
	ztm := NewZeroTrustManager()
	ztm.DeviceManager.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp", TrustLevel: ZTTrustLevelUntrusted})
	d := ztm.ProcessAccessRequest(ZTAccessRequest{UserID: "u", DeviceID: "d1", Resource: "/api", Action: "read", Timestamp: time.Now()})
	if d.Allowed {
		t.Error("untrusted device denied")
	}
}

func TestTrustLevel_String(t *testing.T) {
	tests := []struct {
		l ZTTrustLevel
		s string
	}{
		{ZTTrustLevelUntrusted, "untrusted"}, {ZTTrustLevelLow, "low"}, {ZTTrustLevelMedium, "medium"},
		{ZTTrustLevelHigh, "high"}, {ZTTrustLevelFull, "full"}, {ZTTrustLevel(99), "unknown"},
	}
	for _, tc := range tests {
		if tc.l.String() != tc.s {
			t.Errorf("%d: %s != %s", tc.l, tc.l.String(), tc.s)
		}
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		sev Severity
		s   string
	}{
		{SeverityInfo, "info"}, {SeverityLow, "low"}, {SeverityMedium, "medium"},
		{SeverityHigh, "high"}, {SeverityCritical, "critical"}, {Severity(99), "unknown"},
	}
	for _, tc := range tests {
		if tc.sev.String() != tc.s {
			t.Errorf("%d: %s != %s", tc.sev, tc.sev.String(), tc.s)
		}
	}
}

func TestResponseAction_String(t *testing.T) {
	tests := []struct {
		a ResponseAction
		s string
	}{
		{ActionLog, "log"}, {ZTActionAlert, "alert"}, {ActionThrottle, "throttle"},
		{ActionBlock, "block"}, {ActionQuarantine, "quarantine"}, {ResponseAction(99), "unknown"},
	}
	for _, tc := range tests {
		if tc.a.String() != tc.s {
			t.Errorf("%d: %s != %s", tc.a, tc.a.String(), tc.s)
		}
	}
}

func TestPolicyEffect_String(t *testing.T) {
	tests := []struct {
		e PolicyEffect
		s string
	}{
		{PolicyAllow, "allow"}, {PolicyDeny, "deny"}, {PolicyChallenge, "challenge"}, {PolicyEffect(99), "unknown"},
	}
	for _, tc := range tests {
		if tc.e.String() != tc.s {
			t.Errorf("%d: %s != %s", tc.e, tc.e.String(), tc.s)
		}
	}
}

func TestMicroSegmentManager_ListSorted(t *testing.T) {
	msm := NewMicroSegmentManager()
	msm.AddSegment(&ZTNetworkSegment{ID: "c", Subnets: []string{"10.0.3.0/24"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "a", Subnets: []string{"10.0.1.0/24"}})
	msm.AddSegment(&ZTNetworkSegment{ID: "b", Subnets: []string{"10.0.2.0/24"}})
	s := msm.ListSegments()
	if s[0].ID != "a" || s[1].ID != "b" || s[2].ID != "c" {
		t.Error("not sorted")
	}
}

func TestEvaluateDeviceTrust_TimeDecay(t *testing.T) {
	dtm := NewDeviceTrustManager()
	dtm.RegisterDevice(&DeviceInfo{ID: "d1", Name: "d", Fingerprint: "fp", TrustLevel: ZTTrustLevelHigh,
		Compliance: DeviceCompliance{OSUpdated: true, FirewallEnabled: true, AntivirusActive: true, DiskEncrypted: true, PasswordStrong: true}})
	dtm.CheckCompliance("d1") // 计算合规分数
	trust, _ := dtm.EvaluateDeviceTrust("d1")
	if trust != ZTTrustLevelHigh {
		t.Errorf("trust: %s", trust)
	}
}

func TestSecurityEventManager_SeverityFilter(t *testing.T) {
	sem := NewSecurityEventManager()
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityInfo, Source: "s"})
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityHigh, Source: "s"})
	sem.RecordEvent(&SecurityEvent{Type: "t", Severity: SeverityCritical, Source: "s"})
	sev := SeverityHigh
	events := sem.GetEvents(10, &sev)
	if len(events) != 2 {
		t.Errorf("want 2, got %d", len(events))
	}
}

func TestContinuousAuth_HighRisk_KillsSession(t *testing.T) {
	ca := NewContinuousAuth()
	s, _ := ca.CreateSession("u1", "d1", "1.2.3.4", "l")
	for i := 0; i < 10; i++ {
		ca.RecordActivity(s.ID, Activity{Type: "access", Success: false, Location: "l1", IP: "1.2.3.4"})
	}
	for i := 0; i < 5; i++ {
		ca.RecordActivity(s.ID, Activity{Type: "admin", Success: true, Location: "l2", IP: "5.6.7.8"})
	}
	for i := 0; i < 5; i++ {
		ca.RecordActivity(s.ID, Activity{Type: "download", Success: true, Location: "l3", IP: "9.10.11.12"})
	}
	for i := 0; i < 5; i++ {
		ca.RecordActivity(s.ID, Activity{Type: "admin", Success: false, Location: "l4", IP: "13.14.15.16"})
	}
	got, _ := ca.GetSession(s.ID)
	if got.RiskScore >= 80 && got.Active {
		t.Error("high risk kill")
	}
}

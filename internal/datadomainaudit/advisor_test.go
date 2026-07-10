package datadomainaudit

import (
	"testing"
)

func TestAdvisor_PIIUnencrypted(t *testing.T) {
	s := Signal{PIIPresent: true, PIIEncrypted: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-PII-ENCRYPT" {
			if r.Priority != "Critical" {
				t.Fatalf("expected Critical priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-PII-ENCRYPT when PII is present but unencrypted")
	}
}

func TestAdvisor_PIIEncrypted_NoRec(t *testing.T) {
	s := Signal{PIIPresent: true, PIIEncrypted: true}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "REC-PII-ENCRYPT" {
			t.Fatal("should not recommend PII encryption when already encrypted")
		}
	}
}

func TestAdvisor_CrossBorderReplication(t *testing.T) {
	s := Signal{CrossBorderReplication: true, DataLocalizationRequired: true}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-CROSS-BORDER" {
			if r.Priority != "Critical" {
				t.Fatalf("expected Critical priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-CROSS-BORDER when cross-border replication with localization required")
	}
}

func TestAdvisor_CrossBorderNoLocalization(t *testing.T) {
	s := Signal{CrossBorderReplication: true, DataLocalizationRequired: false}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "REC-CROSS-BORDER" {
			t.Fatal("should not recommend halting cross-border when localization not required")
		}
	}
}

func TestAdvisor_NoResidencyPolicy(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-RESIDENCY-POLICY" {
			if r.Priority != "High" {
				t.Fatalf("expected High priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-RESIDENCY-POLICY when no residency policy exists")
	}
}

func TestAdvisor_IncompleteAccessLog(t *testing.T) {
	s := Signal{AccessLogComplete: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-ACCESS-LOG" {
			if r.Priority != "High" {
				t.Fatalf("expected High priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-ACCESS-LOG when access logs are incomplete")
	}
}

func TestAdvisor_NoRetentionPolicy(t *testing.T) {
	s := Signal{RetentionPolicyExists: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-RETENTION-POLICY" {
			if r.Priority != "High" {
				t.Fatalf("expected High priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-RETENTION-POLICY when no retention policy exists")
	}
}

func TestAdvisor_NoProcessingInventory(t *testing.T) {
	s := Signal{DataProcessingInventoryExists: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-PROCESSING-INVENTORY" {
			if r.Priority != "High" {
				t.Fatalf("expected High priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-PROCESSING-INVENTORY when no processing inventory exists")
	}
}

func TestAdvisor_DPAMissing(t *testing.T) {
	s := Signal{DPAAgreementExists: false, ThirdPartyDataSharing: true}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-DPA-AGREEMENT" {
			if r.Priority != "High" {
				t.Fatalf("expected High priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-DPA-AGREEMENT when third-party sharing without DPA")
	}
}

func TestAdvisor_DPAPresent_NoRec(t *testing.T) {
	s := Signal{DPAAgreementExists: true, ThirdPartyDataSharing: true}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "REC-DPA-AGREEMENT" {
			t.Fatal("should not recommend DPA when one already exists")
		}
	}
}

func TestAdvisor_DPA_NoThirdParty_NoRec(t *testing.T) {
	s := Signal{DPAAgreementExists: false, ThirdPartyDataSharing: false}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "REC-DPA-AGREEMENT" {
			t.Fatal("should not recommend DPA when there is no third-party sharing")
		}
	}
}

func TestAdvisor_NoEncryptionAtRest(t *testing.T) {
	s := Signal{EncryptionAtRest: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-ENCRYPT-REST" {
			if r.Priority != "Medium" {
				t.Fatalf("expected Medium priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-ENCRYPT-REST when encryption at rest is disabled")
	}
}

func TestAdvisor_NoEncryptionInTransit(t *testing.T) {
	s := Signal{EncryptionInTransit: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-ENCRYPT-TRANSIT" {
			if r.Priority != "Medium" {
				t.Fatalf("expected Medium priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-ENCRYPT-TRANSIT when encryption in transit is disabled")
	}
}

func TestAdvisor_NotDSARCompliant(t *testing.T) {
	s := Signal{DSARCompliant: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-DSAR" {
			if r.Priority != "Medium" {
				t.Fatalf("expected Medium priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-DSAR when not DSAR compliant")
	}
}

func TestAdvisor_NotBreachNotificationReady(t *testing.T) {
	s := Signal{BreachNotificationReady: false}
	recs := Analyze(s)
	found := false
	for _, r := range recs {
		if r.ID == "REC-BREACH-NOTIFY" {
			if r.Priority != "Low" {
				t.Fatalf("expected Low priority, got %s", r.Priority)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation REC-BREACH-NOTIFY when breach notification is not ready")
	}
}

func TestAdvisor_EmptySignal(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)

	// An empty signal should trigger every !flag-based recommendation
	// but NOT the conditional ones (PII-ENCRYPT, CROSS-BORDER, DPA).
	expectedCount := 9 // residency, access-log, retention, inventory, encrypt-rest, encrypt-transit, DSAR, breach-notify
	// Actually: let's count: residency, access-log, retention, inventory, encrypt-rest, encrypt-transit, DSAR, breach-notify = 8
	// DPA requires ThirdPartyDataSharing which is false on empty signal → not included.
	// PII-ENCRYPT requires PIIPresent which is false → not included.
	// CROSS-BORDER requires CrossBorderReplication && DataLocalizationRequired → not included.
	expectedCount = 8

	if len(recs) != expectedCount {
		t.Fatalf("expected %d recommendations for empty signal, got %d", expectedCount, len(recs))
	}

	for _, r := range recs {
		switch r.ID {
		case "REC-RESIDENCY-POLICY", "REC-ACCESS-LOG", "REC-RETENTION-POLICY",
			"REC-PROCESSING-INVENTORY", "REC-ENCRYPT-REST", "REC-ENCRYPT-TRANSIT",
			"REC-DSAR", "REC-BREACH-NOTIFY":
			// OK
		default:
			t.Fatalf("unexpected recommendation ID for empty signal: %s", r.ID)
		}
	}
}

func TestAdvisor_EmptySignal_NoConditionalRecs(t *testing.T) {
	s := Signal{}
	recs := Analyze(s)
	for _, r := range recs {
		if r.ID == "REC-PII-ENCRYPT" {
			t.Fatal("empty signal should not trigger PII encryption recommendation")
		}
		if r.ID == "REC-CROSS-BORDER" {
			t.Fatal("empty signal should not trigger cross-border recommendation")
		}
		if r.ID == "REC-DPA-AGREEMENT" {
			t.Fatal("empty signal should not trigger DPA recommendation")
		}
	}
}

func TestAdvisor_AllViolations(t *testing.T) {
	s := Signal{
		PIIPresent:                   true,
		PIIEncrypted:                 false,
		CrossBorderReplication:       true,
		DataResidencyPolicyExists:    false,
		AccessLogComplete:            false,
		RetentionPolicyExists:        false,
		DataProcessingInventoryExists: false,
		DPAAgreementExists:            false,
		EncryptionAtRest:              false,
		EncryptionInTransit:           false,
		DataLocalizationRequired:      true,
		Jurisdiction:                  "EU-GDPR",
		ThirdPartyDataSharing:         true,
		DSARCompliant:                 false,
		BreachNotificationReady:       false,
	}
	recs := Analyze(s)
	if len(recs) != 11 {
		t.Fatalf("expected 11 recommendations for full-violation signal, got %d", len(recs))
	}
}

func TestAdvisor_AllCompliant(t *testing.T) {
	s := Signal{
		PIIPresent:                   true,
		PIIEncrypted:                 true,
		CrossBorderReplication:       true,
		DataResidencyPolicyExists:    true,
		AccessLogComplete:            true,
		RetentionPolicyExists:        true,
		DataProcessingInventoryExists: true,
		DPAAgreementExists:            true,
		EncryptionAtRest:              true,
		EncryptionInTransit:           true,
		DataLocalizationRequired:      false,
		Jurisdiction:                  "EU-GDPR",
		ThirdPartyDataSharing:         true,
		DSARCompliant:                 true,
		BreachNotificationReady:       true,
	}
	recs := Analyze(s)
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations when fully compliant, got %d", len(recs))
	}
}

func TestAdvisor_PriorityOrdering(t *testing.T) {
	s := Signal{
		PIIPresent:                   true,
		PIIEncrypted:                  false,
		CrossBorderReplication:       true,
		DataLocalizationRequired:     true,
		DataResidencyPolicyExists:    false,
		AccessLogComplete:            false,
		RetentionPolicyExists:        false,
		DataProcessingInventoryExists: false,
		DPAAgreementExists:            false,
		ThirdPartyDataSharing:         true,
		EncryptionAtRest:              false,
		EncryptionInTransit:           false,
		DSARCompliant:                 false,
		BreachNotificationReady:       false,
	}
	recs := Analyze(s)

	// Verify sorted: Critical first, then High, then Medium, then Low
	for i := 1; i < len(recs); i++ {
		if priorityRank(recs[i-1].Priority) > priorityRank(recs[i].Priority) {
			t.Fatalf("recommendations not sorted by priority at index %d: %s before %s",
				i-1, recs[i-1].Priority, recs[i].Priority)
		}
	}
}

func TestAdvisor_PriorityOrdering_AllPresent(t *testing.T) {
	// Trigger all 11 recommendations and verify strict ordering
	s := Signal{
		PIIPresent:                   true,
		PIIEncrypted:                  false,
		CrossBorderReplication:       true,
		DataResidencyPolicyExists:    false,
		AccessLogComplete:            false,
		RetentionPolicyExists:        false,
		DataProcessingInventoryExists: false,
		DPAAgreementExists:            false,
		EncryptionAtRest:              false,
		EncryptionInTransit:           false,
		DataLocalizationRequired:      true,
		ThirdPartyDataSharing:         true,
		DSARCompliant:                 false,
		BreachNotificationReady:       false,
	}
	recs := Analyze(s)
	if len(recs) != 11 {
		t.Fatalf("expected 11 recommendations, got %d", len(recs))
	}

	// Critical: REC-CROSS-BORDER, REC-PII-ENCRYPT (sorted by ID)
	// High: REC-ACCESS-LOG, REC-DPA-AGREEMENT, REC-PROCESSING-INVENTORY, REC-RESIDENCY-POLICY, REC-RETENTION-POLICY
	// Medium: REC-ENCRYPT-REST, REC-ENCRYPT-TRANSIT, REC-DSAR
	// Low: REC-BREACH-NOTIFY

	// Verify the first two are Critical
	if recs[0].Priority != "Critical" || recs[1].Priority != "Critical" {
		t.Fatalf("expected first two recommendations to be Critical, got %s and %s",
			recs[0].Priority, recs[1].Priority)
	}
	// Within Critical, sorted by ID
	if recs[0].ID > recs[1].ID {
		t.Fatalf("Critical recommendations not sorted by ID: %s before %s",
			recs[0].ID, recs[1].ID)
	}

	// Verify last is Low (breach notification)
	if recs[len(recs)-1].Priority != "Low" {
		t.Fatalf("expected last recommendation to be Low priority, got %s",
			recs[len(recs)-1].Priority)
	}
}

func TestAdvisor_JurisdictionFieldPreserved(t *testing.T) {
	s := Signal{
		Jurisdiction:              "EU-GDPR",
		DataResidencyPolicyExists: true,
		AccessLogComplete:         true,
		RetentionPolicyExists:     true,
		DataProcessingInventoryExists: true,
		EncryptionAtRest:          true,
		EncryptionInTransit:       true,
		DSARCompliant:             true,
		BreachNotificationReady:   true,
	}
	recs := Analyze(s)
	if len(recs) != 0 {
		t.Fatalf("expected 0 recommendations for compliant signal with jurisdiction, got %d", len(recs))
	}
	if s.Jurisdiction != "EU-GDPR" {
		t.Fatalf("jurisdiction field should be preserved, got %s", s.Jurisdiction)
	}
}
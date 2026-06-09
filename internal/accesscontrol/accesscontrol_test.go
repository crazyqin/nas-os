package accesscontrol

import (
	"testing"
	"time"
)

func TestNewAccessControlService(t *testing.T) {
	config := AccessConfig{
		AntiPassbackEnabled: true,
		DefaultDoorOpenTime: 5 * time.Second,
		HeldOpenAlarmTime:   30 * time.Second,
		MaxFailedAttempts:   3,
		LockoutDuration:     5 * time.Minute,
		EnableAIAnalytics:   false,
		RetentionDays:       90,
	}
	svc := NewAccessControlService(config)
	if svc == nil {
		t.Fatal("NewAccessControlService returned nil")
	}
	status := svc.GetServiceStatus()
	if status["devices"] != 0 {
		t.Errorf("expected 0 devices, got %v", status["devices"])
	}
	if status["cards"] != 0 {
		t.Errorf("expected 0 cards, got %v", status["cards"])
	}
	if status["rules"] != 0 {
		t.Errorf("expected 0 rules, got %v", status["rules"])
	}
	if status["zones"] != 0 {
		t.Errorf("expected 0 zones, got %v", status["zones"])
	}
}

func TestServiceStartStop(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})
	err := svc.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	err = svc.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRegisterDevice(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		AntiPassbackEnabled: false,
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	device, err := svc.RegisterDevice("Front Door Reader", DeviceCardReader, "lobby", "Building A entrance")
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}
	if device.Name != "Front Door Reader" {
		t.Errorf("expected name 'Front Door Reader', got %q", device.Name)
	}
	if device.Type != DeviceCardReader {
		t.Errorf("expected type %q, got %q", DeviceCardReader, device.Type)
	}
	if device.Zone != "lobby" {
		t.Errorf("expected zone 'lobby', got %q", device.Zone)
	}
	if device.Status != "online" {
		t.Errorf("expected status 'online', got %q", device.Status)
	}
	if !device.Enabled {
		t.Error("expected device to be enabled")
	}
	if device.ID == "" {
		t.Error("expected non-empty device ID")
	}

	status := svc.GetServiceStatus()
	if status["devices"] != 1 {
		t.Errorf("expected 1 device, got %v", status["devices"])
	}
}

func TestCreateZone(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	zone, err := svc.CreateZone("Server Room", "Main server room", 8)
	if err != nil {
		t.Fatalf("CreateZone failed: %v", err)
	}
	if zone.Name != "Server Room" {
		t.Errorf("expected name 'Server Room', got %q", zone.Name)
	}
	if zone.Level != 8 {
		t.Errorf("expected level 8, got %d", zone.Level)
	}
	if !zone.Enabled {
		t.Error("expected zone to be enabled")
	}
	if zone.ID == "" {
		t.Error("expected non-empty zone ID")
	}
}

func TestIssueCard(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	card, err := svc.IssueCard("user-001", "CARD-12345", "mifare", []string{"lobby", "office"}, 365)
	if err != nil {
		t.Fatalf("IssueCard failed: %v", err)
	}
	if card.UserID != "user-001" {
		t.Errorf("expected userID 'user-001', got %q", card.UserID)
	}
	if card.CardNumber != "CARD-12345" {
		t.Errorf("expected card number 'CARD-12345', got %q", card.CardNumber)
	}
	if card.CardType != "mifare" {
		t.Errorf("expected card type 'mifare', got %q", card.CardType)
	}
	if card.Status != "active" {
		t.Errorf("expected status 'active', got %q", card.Status)
	}
	if len(card.Zones) != 2 {
		t.Errorf("expected 2 zones, got %d", len(card.Zones))
	}
	if card.ValidTo.Before(card.ValidFrom) {
		t.Error("expected ValidTo to be after ValidFrom")
	}
}

func TestIssueDuplicateCard(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	_, err := svc.IssueCard("user-001", "CARD-DUP", "mifare", []string{"lobby"}, 365)
	if err != nil {
		t.Fatalf("first IssueCard failed: %v", err)
	}
	_, err = svc.IssueCard("user-002", "CARD-DUP", "mifare", []string{"lobby"}, 365)
	if err == nil {
		t.Error("expected error for duplicate card number, got nil")
	}
}

func TestAddAccessRule(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	slots := []TimeSlot{
		{Start: "09:00", End: "18:00"},
	}
	rule, err := svc.AddAccessRule("Office Hours", "user-001", "office", true, slots, []string{"mon", "tue", "wed", "thu", "fri"})
	if err != nil {
		t.Fatalf("AddAccessRule failed: %v", err)
	}
	if rule.Name != "Office Hours" {
		t.Errorf("expected name 'Office Hours', got %q", rule.Name)
	}
	if !rule.Allowed {
		t.Error("expected rule to be allowed")
	}
	if len(rule.TimeSlots) != 1 {
		t.Errorf("expected 1 time slot, got %d", len(rule.TimeSlots))
	}
	if len(rule.DaysOfWeek) != 5 {
		t.Errorf("expected 5 days, got %d", len(rule.DaysOfWeek))
	}
}

func TestVerifyAccessGranted(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	device, _ := svc.RegisterDevice("Front Door", DeviceCardReader, "lobby", "Entrance")
	_, _ = svc.IssueCard("user-001", "CARD-OK", "mifare", []string{"lobby"}, 365)

	event, err := svc.VerifyAccess(device.ID, "CARD-OK")
	if err != nil {
		t.Fatalf("VerifyAccess failed: %v", err)
	}
	if event.Decision != DecisionGranted {
		t.Errorf("expected decision %q, got %q", DecisionGranted, event.Decision)
	}
	if event.EventType != EventAccessGranted {
		t.Errorf("expected event type %q, got %q", EventAccessGranted, event.EventType)
	}
	if event.UserID != "user-001" {
		t.Errorf("expected userID 'user-001', got %q", event.UserID)
	}
}

func TestVerifyAccessCardNotFound(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	device, _ := svc.RegisterDevice("Front Door", DeviceCardReader, "lobby", "Entrance")

	event, err := svc.VerifyAccess(device.ID, "CARD-UNKNOWN")
	if err != nil {
		t.Fatalf("VerifyAccess failed: %v", err)
	}
	if event.Decision != DecisionDenied {
		t.Errorf("expected decision %q, got %q", DecisionDenied, event.Decision)
	}
	if event.Details != "Card not found" {
		t.Errorf("expected details 'Card not found', got %q", event.Details)
	}
}

func TestVerifyAccessDeviceNotFound(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	_, err := svc.VerifyAccess("nonexistent-device", "CARD-123")
	if err == nil {
		t.Error("expected error for nonexistent device, got nil")
	}
}

func TestVerifyAccessZoneDenied(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	device, _ := svc.RegisterDevice("Server Door", DeviceCardReader, "server-room", "Server Room")
	// Card only allowed in lobby, not server-room
	_, _ = svc.IssueCard("user-001", "CARD-LOBBY", "mifare", []string{"lobby"}, 365)

	event, err := svc.VerifyAccess(device.ID, "CARD-LOBBY")
	if err != nil {
		t.Fatalf("VerifyAccess failed: %v", err)
	}
	if event.Decision != DecisionDenied {
		t.Errorf("expected decision %q, got %q", DecisionDenied, event.Decision)
	}
	if event.Details != "Zone not allowed" {
		t.Errorf("expected details 'Zone not allowed', got %q", event.Details)
	}
}

func TestGetServiceStatus(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		EnableAIAnalytics:   true,
		RetentionDays:       30,
	})

	svc.RegisterDevice("Reader 1", DeviceCardReader, "zone1", "Loc1")
	svc.RegisterDevice("Controller 1", DeviceController, "zone1", "Loc2")
	svc.CreateZone("Zone1", "Test zone", 5)
	svc.IssueCard("u1", "C1", "mifare", []string{"zone1"}, 30)
	svc.AddAccessRule("Rule1", "u1", "zone1", true, nil, nil)

	status := svc.GetServiceStatus()
	if status["devices"] != 2 {
		t.Errorf("expected 2 devices, got %v", status["devices"])
	}
	if status["cards"] != 1 {
		t.Errorf("expected 1 card, got %v", status["cards"])
	}
	if status["rules"] != 1 {
		t.Errorf("expected 1 rule, got %v", status["rules"])
	}
	if status["zones"] != 1 {
		t.Errorf("expected 1 zone, got %v", status["zones"])
	}
	if status["ai_enabled"] != true {
		t.Errorf("expected ai_enabled true, got %v", status["ai_enabled"])
	}
}

func TestGetEvents(t *testing.T) {
	svc := NewAccessControlService(AccessConfig{
		DefaultDoorOpenTime: 5 * time.Second,
		RetentionDays:       30,
	})

	svc.Start()
	defer svc.Stop()

	device, _ := svc.RegisterDevice("Reader", DeviceCardReader, "lobby", "Loc")
	_, _ = svc.IssueCard("user-1", "CARD-A", "mifare", []string{"lobby"}, 365)
	_, _ = svc.IssueCard("user-2", "CARD-B", "mifare", []string{"lobby"}, 365)

	// Generate some access events
	svc.VerifyAccess(device.ID, "CARD-A")
	svc.VerifyAccess(device.ID, "CARD-B")
	svc.VerifyAccess(device.ID, "CARD-UNKNOWN")

	// Wait for event processing (needs more time as eventChan has capacity 10000)
	time.Sleep(500 * time.Millisecond)

	// All events (no filter)
	allEvents := svc.GetEvents("", "", nil, nil, 0)
	if len(allEvents) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(allEvents))
	}

	// Filter by device
	deviceEvents := svc.GetEvents(device.ID, "", nil, nil, 0)
	if len(deviceEvents) < 3 {
		t.Errorf("expected at least 3 device events, got %d", len(deviceEvents))
	}

	// Filter by user
	userEvents := svc.GetEvents("", "user-1", nil, nil, 0)
	if len(userEvents) < 1 {
		t.Errorf("expected at least 1 user event, got %d", len(userEvents))
	}

	// Limit
	limitedEvents := svc.GetEvents("", "", nil, nil, 1)
	if len(limitedEvents) > 1 {
		t.Errorf("expected at most 1 event with limit, got %d", len(limitedEvents))
	}
}

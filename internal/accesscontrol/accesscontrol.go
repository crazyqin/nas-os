// Package accesscontrol implements an intelligent access control system
// inspired by Synology's AC100 and AR series. It provides physical security
// management, card reader integration, and intelligent access analytics.
package accesscontrol

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"
)

// DeviceType defines the type of access control device
type DeviceType string

const (
	DeviceController DeviceType = "controller"  // AC100-like controller
	DeviceCardReader DeviceType = "card_reader"  // AR series reader
	DeviceDoorLock   DeviceType = "door_lock"    // Electronic lock
	DeviceSensor     DeviceType = "sensor"       // Door sensor
	DeviceCamera     DeviceType = "camera"       // Associated camera
	DeviceIntercom   DeviceType = "intercom"     // Video intercom
)

// AccessMethod defines how access can be granted
type AccessMethod string

const (
	AccessCard        AccessMethod = "card"        // RFID/NFC card
	AccessPIN         AccessMethod = "pin"         // PIN code
	AccessFace        AccessMethod = "face"        // Facial recognition
	AccessFingerprint AccessMethod = "fingerprint" // Biometric
	AccessMobile      AccessMethod = "mobile"      // Mobile app
	AccessQR          AccessMethod = "qr_code"     // QR code
)

// AccessDecision represents the result of an access request
type AccessDecision string

const (
	DecisionGranted AccessDecision = "granted"
	DecisionDenied  AccessDecision = "denied"
	DecisionPending AccessDecision = "pending"
	DecisionExpired AccessDecision = "expired"
)

// EventType defines the type of access event
type EventType string

const (
	EventAccessGranted EventType = "access_granted"
	EventAccessDenied  EventType = "access_denied"
	EventDoorOpened    EventType = "door_opened"
	EventDoorClosed    EventType = "door_closed"
	EventDoorHeldOpen  EventType = "door_held_open"
	EventDoorForced    EventType = "door_forced"
	EventTamperAlert   EventType = "tamper_alert"
	EventDeviceOffline EventType = "device_offline"
	EventDeviceOnline  EventType = "device_online"
)

// Device represents an access control device
type Device struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Type       DeviceType   `json:"type"`
	Zone       string       `json:"zone"`
	Location   string       `json:"location"`
	IPAddress  string       `json:"ip_address,omitempty"`
	MACAddress string       `json:"mac_address,omitempty"`
	Firmware   string       `json:"firmware"`
	Status     string       `json:"status"` // online, offline, error
	LastSeen   time.Time    `json:"last_seen"`
	Enabled    bool         `json:"enabled"`
	Config     DeviceConfig `json:"config"`
}

// DeviceConfig contains device-specific configuration
type DeviceConfig struct {
	SupportedMethods []AccessMethod `json:"supported_methods"`
	MaxCards         int            `json:"max_cards"`
	AntiPassback     bool           `json:"anti_passback"`
	DoorOpenTime     time.Duration  `json:"door_open_time"`
	AlarmOnHeldOpen  time.Duration  `json:"alarm_on_held_open"`
}

// AccessCredential represents a user's access card
type AccessCredential struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	CardNumber string    `json:"card_number"`
	CardType   string    `json:"card_type"` // mifare, desfire, em4100
	Status     string    `json:"status"`    // active, suspended, lost, expired
	ValidFrom  time.Time `json:"valid_from"`
	ValidTo    time.Time `json:"valid_to"`
	Zones      []string  `json:"zones"` // Allowed zones
	CreatedAt  time.Time `json:"created_at"`
}

// AccessRule defines when and where a user can access
type AccessRule struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	UserID     string     `json:"user_id"`
	Zone       string     `json:"zone"`
	Allowed    bool       `json:"allowed"`
	TimeSlots  []TimeSlot `json:"time_slots"`
	DaysOfWeek []string   `json:"days_of_week"` // mon, tue, wed, thu, fri, sat, sun
	ValidFrom  time.Time  `json:"valid_from"`
	ValidTo    time.Time  `json:"valid_to"`
	Priority   int        `json:"priority"`
}

// TimeSlot represents a time range within a day
type TimeSlot struct {
	Start string `json:"start"` // HH:MM format
	End   string `json:"end"`   // HH:MM format
}

// AccessEvent represents an access control event
type AccessEvent struct {
	ID         string         `json:"id"`
	DeviceID   string         `json:"device_id"`
	UserID     string         `json:"user_id,omitempty"`
	CardNumber string         `json:"card_number,omitempty"`
	EventType  EventType      `json:"event_type"`
	Decision   AccessDecision `json:"decision"`
	Timestamp  time.Time      `json:"timestamp"`
	Zone       string         `json:"zone"`
	Details    string         `json:"details,omitempty"`
	ImageURL   string         `json:"image_url,omitempty"` // Captured image
}

// Zone represents a security zone
type Zone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ParentZone  string   `json:"parent_zone,omitempty"`
	Devices     []string `json:"devices"` // Device IDs in this zone
	Level       int      `json:"level"`  // Security level 1-10
	Enabled     bool     `json:"enabled"`
}

// AccessSchedule defines access schedule for a zone
type AccessSchedule struct {
	ID      string         `json:"id"`
	ZoneID  string         `json:"zone_id"`
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Rules   []ScheduleRule `json:"rules"`
}

// ScheduleRule defines a rule within a schedule
type ScheduleRule struct {
	Name      string   `json:"name"`
	Days      []string `json:"days"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	Access    bool     `json:"access"`
}

// ============================================================
// Audit Logging
// ============================================================

// AuditSeverity classifies audit entry severity
type AuditSeverity string

const (
	AuditInfo     AuditSeverity = "info"
	AuditWarning  AuditSeverity = "warning"
	AuditAlert    AuditSeverity = "alert"
	AuditCritical AuditSeverity = "critical"
)

// AuditEntry is a structured audit log record
type AuditEntry struct {
	Timestamp  time.Time      `json:"timestamp"`
	Severity   AuditSeverity  `json:"severity"`
	EventType  EventType      `json:"event_type"`
	Decision   AccessDecision `json:"decision"`
	DeviceID   string         `json:"device_id"`
	UserID     string         `json:"user_id,omitempty"`
	CardNumber string         `json:"card_number,omitempty"`
	Zone       string         `json:"zone"`
	Details    string         `json:"details"`
	RemoteAddr string         `json:"remote_addr,omitempty"`
}

// AuditWriter writes audit entries to a destination
type AuditWriter interface {
	Write(entry AuditEntry) error
}

// FileAuditWriter writes audit entries to a log file in JSON Lines format
type FileAuditWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileAuditWriter creates an audit writer that appends to the given path
func NewFileAuditWriter(path string) (*FileAuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, fmt.Errorf("audit writer: %w", err)
	}
	return &FileAuditWriter{file: f}, nil
}

// Write appends a single audit entry
func (w *FileAuditWriter) Write(entry AuditEntry) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	line := fmt.Sprintf(`{"ts":"%s","severity":"%s","event":"%s","decision":"%s","device":"%s","user":"%s","zone":"%s","details":"%s"}`,
		entry.Timestamp.Format(time.RFC3339), entry.Severity, entry.EventType,
		entry.Decision, entry.DeviceID, entry.UserID, entry.Zone, entry.Details)
	_, err := w.file.WriteString(line + "\n")
	return err
}

// Close closes the underlying file
func (w *FileAuditWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// MultiAuditWriter fans out entries to multiple writers
type MultiAuditWriter struct {
	writers []AuditWriter
}

// NewMultiAuditWriter creates a writer that dispatches to all provided writers
func NewMultiAuditWriter(writers ...AuditWriter) *MultiAuditWriter {
	return &MultiAuditWriter{writers: writers}
}

// Write sends the entry to every underlying writer
func (m *MultiAuditWriter) Write(entry AuditEntry) error {
	var firstErr error
	for _, w := range m.writers {
		if err := w.Write(entry); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ioAuditWriter wraps an io.Writer (e.g. os.Stdout) as an AuditWriter
type ioAuditWriter struct {
	w io.Writer
}

// NewIOAuditWriter returns an AuditWriter backed by an io.Writer
func NewIOAuditWriter(w io.Writer) AuditWriter {
	return &ioAuditWriter{w: w}
}

func (a *ioAuditWriter) Write(entry AuditEntry) error {
	line := fmt.Sprintf("[AUDIT] %s severity=%s event=%s decision=%s device=%s user=%s zone=%s details=%s\n",
		entry.Timestamp.Format(time.RFC3339), entry.Severity, entry.EventType,
		entry.Decision, entry.DeviceID, entry.UserID, entry.Zone, entry.Details)
	_, err := a.w.Write([]byte(line))
	return err
}

// ============================================================
// Configuration & Service
// ============================================================

// AccessConfig contains access control system configuration
type AccessConfig struct {
	AntiPassbackEnabled bool          `json:"anti_passback_enabled"`
	DefaultDoorOpenTime time.Duration `json:"default_door_open_time"`
	HeldOpenAlarmTime   time.Duration `json:"held_open_alarm_time"`
	MaxFailedAttempts   int           `json:"max_failed_attempts"`
	LockoutDuration     time.Duration `json:"lockout_duration"`
	EnableAIAnalytics   bool          `json:"enable_ai_analytics"`
	RetentionDays       int           `json:"retention_days"`
	AuditWriters        []AuditWriter `json:"-"`
}

// AccessControlService is the main access control service
type AccessControlService struct {
	mu             sync.RWMutex
	config         AccessConfig
	devices        map[string]*Device
	cards          map[string]*AccessCredential
	rules          map[string]*AccessRule
	events         []AccessEvent
	zones          map[string]*Zone
	schedules      map[string]*AccessSchedule
	ctx            context.Context
	cancel         context.CancelFunc
	eventChan      chan AccessEvent
	auditWriters   []AuditWriter
	failedAttempts map[string]*failedAttemptState
}

// failedAttemptState tracks lockout state for a credential
type failedAttemptState struct {
	count    int
	lockedAt time.Time
}

// NewAccessControlService creates a new access control service
func NewAccessControlService(config AccessConfig) *AccessControlService {
	ctx, cancel := context.WithCancel(context.Background())

	// Default: write audit to stdout if no writers configured
	auditWriters := config.AuditWriters
	if len(auditWriters) == 0 {
		auditWriters = []AuditWriter{NewIOAuditWriter(os.Stdout)}
	}

	service := &AccessControlService{
		config:         config,
		devices:        make(map[string]*Device),
		cards:          make(map[string]*AccessCredential),
		rules:          make(map[string]*AccessRule),
		events:         make([]AccessEvent, 0),
		zones:          make(map[string]*Zone),
		schedules:      make(map[string]*AccessSchedule),
		ctx:            ctx,
		cancel:         cancel,
		eventChan:      make(chan AccessEvent, 10000),
		auditWriters:   auditWriters,
		failedAttempts: make(map[string]*failedAttemptState),
	}

	return service
}

// Start begins the access control service
func (s *AccessControlService) Start() error {
	log.Println("[AccessControl] Starting intelligent access control system")

	// Start event processor
	go s.processEvents()

	// Start device monitor
	go s.monitorDevices()

	// Start analytics engine if enabled
	if s.config.EnableAIAnalytics {
		go s.runAnalytics()
	}

	log.Println("[AccessControl] Service started successfully")
	return nil
}

// Stop gracefully stops the service
func (s *AccessControlService) Stop() error {
	s.cancel()
	log.Println("[AccessControl] Service stopped")
	return nil
}

// RegisterDevice registers a new access control device
func (s *AccessControlService) RegisterDevice(name string, deviceType DeviceType, zone, location string) (*Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	device := &Device{
		ID:       generateID(),
		Name:     name,
		Type:     deviceType,
		Zone:     zone,
		Location: location,
		Status:   "online",
		LastSeen: time.Now(),
		Enabled:  true,
		Config: DeviceConfig{
			SupportedMethods: []AccessMethod{AccessCard, AccessPIN},
			MaxCards:         10000,
			AntiPassback:     s.config.AntiPassbackEnabled,
			DoorOpenTime:     s.config.DefaultDoorOpenTime,
			AlarmOnHeldOpen:  s.config.HeldOpenAlarmTime,
		},
	}

	s.devices[device.ID] = device
	log.Printf("[AccessControl] Device registered: %s (%s) in zone %s", name, device.ID, zone)

	return device, nil
}

// CreateZone creates a new security zone
func (s *AccessControlService) CreateZone(name, description string, level int) (*Zone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	zone := &Zone{
		ID:          generateID(),
		Name:        name,
		Description: description,
		Level:       level,
		Enabled:     true,
	}

	s.zones[zone.ID] = zone
	log.Printf("[AccessControl] Zone created: %s (level %d)", name, level)

	return zone, nil
}

// IssueCard issues a new access card to a user
func (s *AccessControlService) IssueCard(userID, cardNumber, cardType string, zones []string, validDays int) (*AccessCredential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if card number already exists
	for _, card := range s.cards {
		if card.CardNumber == cardNumber {
			return nil, fmt.Errorf("card number already exists: %s", cardNumber)
		}
	}

	now := time.Now()
	card := &AccessCredential{
		ID:         generateID(),
		UserID:     userID,
		CardNumber: cardNumber,
		CardType:   cardType,
		Status:     "active",
		ValidFrom:  now,
		ValidTo:    now.AddDate(0, 0, validDays),
		Zones:      zones,
		CreatedAt:  now,
	}

	s.cards[card.ID] = card
	log.Printf("[AccessControl] Card issued to user %s: %s", userID, cardNumber)

	return card, nil
}

// AddAccessRule adds an access rule for a user
func (s *AccessControlService) AddAccessRule(name, userID, zone string, allowed bool, timeSlots []TimeSlot, days []string) (*AccessRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rule := &AccessRule{
		ID:         generateID(),
		Name:       name,
		UserID:     userID,
		Zone:       zone,
		Allowed:    allowed,
		TimeSlots:  timeSlots,
		DaysOfWeek: days,
		ValidFrom:  time.Now(),
		ValidTo:    time.Now().AddDate(1, 0, 0), // Valid for 1 year
		Priority:   100,
	}

	s.rules[rule.ID] = rule
	log.Printf("[AccessControl] Access rule added: %s for user %s", name, userID)

	return rule, nil
}

// emitAudit writes an audit entry to all configured writers
func (s *AccessControlService) emitAudit(entry AuditEntry) {
	for _, w := range s.auditWriters {
		if err := w.Write(entry); err != nil {
			log.Printf("[AccessControl] audit write error: %v", err)
		}
	}
}

// VerifyAccess checks if a user/card is allowed access at a specific device
func (s *AccessControlService) VerifyAccess(deviceID, cardNumber string) (*AccessEvent, error) {
	s.mu.RLock()
	device, exists := s.devices[deviceID]
	if !exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	// Find card
	var card *AccessCredential
	for _, c := range s.cards {
		if c.CardNumber == cardNumber {
			card = c
			break
		}
	}
	s.mu.RUnlock()

	event := AccessEvent{
		ID:         generateID(),
		DeviceID:   deviceID,
		CardNumber: cardNumber,
		Timestamp:  time.Now(),
		Zone:       device.Zone,
	}

	if card == nil {
		event.EventType = EventAccessDenied
		event.Decision = DecisionDenied
		event.Details = "Card not found"
		s.eventChan <- event
		s.emitAudit(AuditEntry{
			Timestamp: event.Timestamp, Severity: AuditWarning,
			EventType: event.EventType, Decision: event.Decision,
			DeviceID: deviceID, CardNumber: cardNumber,
			Zone: device.Zone, Details: event.Details,
		})
		return &event, nil
	}

	if card.Status != "active" {
		event.UserID = card.UserID
		event.EventType = EventAccessDenied
		event.Decision = DecisionDenied
		event.Details = fmt.Sprintf("Card status: %s", card.Status)
		s.eventChan <- event
		s.emitAudit(AuditEntry{
			Timestamp: event.Timestamp, Severity: AuditWarning,
			EventType: event.EventType, Decision: event.Decision,
			DeviceID: deviceID, UserID: card.UserID, CardNumber: cardNumber,
			Zone: device.Zone, Details: event.Details,
		})
		return &event, nil
	}

	if time.Now().After(card.ValidTo) {
		event.UserID = card.UserID
		event.EventType = EventAccessDenied
		event.Decision = DecisionExpired
		event.Details = "Card expired"
		s.eventChan <- event
		s.emitAudit(AuditEntry{
			Timestamp: event.Timestamp, Severity: AuditWarning,
			EventType: event.EventType, Decision: event.Decision,
			DeviceID: deviceID, UserID: card.UserID, CardNumber: cardNumber,
			Zone: device.Zone, Details: event.Details,
		})
		return &event, nil
	}

	// Check lockout
	s.mu.RLock()
	fa, isLocked := s.failedAttempts[cardNumber]
	s.mu.RUnlock()
	if isLocked && s.config.MaxFailedAttempts > 0 && fa.count >= s.config.MaxFailedAttempts {
		if time.Since(fa.lockedAt) < s.config.LockoutDuration {
			event.UserID = card.UserID
			event.EventType = EventAccessDenied
			event.Decision = DecisionDenied
			event.Details = "Account locked out"
			s.eventChan <- event
			s.emitAudit(AuditEntry{
				Timestamp: event.Timestamp, Severity: AuditAlert,
				EventType: event.EventType, Decision: event.Decision,
				DeviceID: deviceID, UserID: card.UserID, CardNumber: cardNumber,
				Zone: device.Zone, Details: event.Details,
			})
			return &event, nil
		}
		// Lockout expired — reset
		s.mu.Lock()
		delete(s.failedAttempts, cardNumber)
		s.mu.Unlock()
	}

	// Check zone access
	zoneAllowed := false
	for _, z := range card.Zones {
		if z == device.Zone {
			zoneAllowed = true
			break
		}
	}

	if !zoneAllowed {
		event.UserID = card.UserID
		event.EventType = EventAccessDenied
		event.Decision = DecisionDenied
		event.Details = "Zone not allowed"
		s.eventChan <- event
		s.incrementFailed(cardNumber)
		s.emitAudit(AuditEntry{
			Timestamp: event.Timestamp, Severity: AuditWarning,
			EventType: event.EventType, Decision: event.Decision,
			DeviceID: deviceID, UserID: card.UserID, CardNumber: cardNumber,
			Zone: device.Zone, Details: event.Details,
		})
		return &event, nil
	}

	// Access granted
	event.UserID = card.UserID
	event.EventType = EventAccessGranted
	event.Decision = DecisionGranted
	event.Details = "Access granted"
	s.eventChan <- event

	// Reset failed attempts on success
	s.mu.Lock()
	delete(s.failedAttempts, cardNumber)
	s.mu.Unlock()

	s.emitAudit(AuditEntry{
		Timestamp: event.Timestamp, Severity: AuditInfo,
		EventType: event.EventType, Decision: event.Decision,
		DeviceID: deviceID, UserID: card.UserID, CardNumber: cardNumber,
		Zone: device.Zone, Details: event.Details,
	})

	// Trigger door open
	go s.triggerDoorOpen(deviceID)

	return &event, nil
}

// incrementFailed tracks consecutive failed attempts
func (s *AccessControlService) incrementFailed(cardNumber string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fa, exists := s.failedAttempts[cardNumber]
	if !exists {
		s.failedAttempts[cardNumber] = &failedAttemptState{count: 1, lockedAt: time.Now()}
		return
	}
	fa.count++
	if fa.count >= s.config.MaxFailedAttempts && s.config.MaxFailedAttempts > 0 {
		fa.lockedAt = time.Now()
	}
}

// triggerDoorOpen sends door open command to device
func (s *AccessControlService) triggerDoorOpen(deviceID string) {
	log.Printf("[AccessControl] Door opened on device %s", deviceID)

	// Schedule door close after configured time
	time.AfterFunc(s.config.DefaultDoorOpenTime, func() {
		event := AccessEvent{
			ID:        generateID(),
			DeviceID:  deviceID,
			EventType: EventDoorClosed,
			Timestamp: time.Now(),
		}
		s.eventChan <- event
	})
}

// GetEvents retrieves access events with optional filters
func (s *AccessControlService) GetEvents(deviceID, userID string, startTime, endTime *time.Time, limit int) []AccessEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []AccessEvent
	for _, event := range s.events {
		if deviceID != "" && event.DeviceID != deviceID {
			continue
		}
		if userID != "" && event.UserID != userID {
			continue
		}
		if startTime != nil && event.Timestamp.Before(*startTime) {
			continue
		}
		if endTime != nil && event.Timestamp.After(*endTime) {
			continue
		}

		results = append(results, event)
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	return results
}

// processEvents processes incoming access events
func (s *AccessControlService) processEvents() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event := <-s.eventChan:
			s.mu.Lock()
			s.events = append(s.events, event)

			// Trim old events based on retention policy
			cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)
			for i, e := range s.events {
				if e.Timestamp.After(cutoff) {
					s.events = s.events[i:]
					break
				}
			}
			s.mu.Unlock()

			// Alert on security events
			if event.EventType == EventDoorForced || event.EventType == EventTamperAlert {
				s.sendSecurityAlert(event)
			}
		}
	}
}

// monitorDevices monitors device health
func (s *AccessControlService) monitorDevices() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkDeviceStatus()
		}
	}
}

// checkDeviceStatus checks all devices are online
func (s *AccessControlService) checkDeviceStatus() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, device := range s.devices {
		if time.Since(device.LastSeen) > 2*time.Minute && device.Status == "online" {
			device.Status = "offline"
			event := AccessEvent{
				ID:        generateID(),
				DeviceID:  device.ID,
				EventType: EventDeviceOffline,
				Timestamp: time.Now(),
				Zone:      device.Zone,
				Details:   fmt.Sprintf("Device %s went offline", device.Name),
			}
			s.eventChan <- event
		}
	}
}

// sendSecurityAlert sends alert for security events
func (s *AccessControlService) sendSecurityAlert(event AccessEvent) {
	log.Printf("[AccessControl] SECURITY ALERT: %s on device %s", event.EventType, event.DeviceID)
}

// runAnalytics runs AI-powered analytics on access patterns
func (s *AccessControlService) runAnalytics() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.analyzePatterns()
		}
	}
}

// analyzePatterns analyzes access patterns for anomalies
func (s *AccessControlService) analyzePatterns() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Analyze recent events for unusual patterns
	// This would use AI in production
	log.Printf("[AccessControl] Running pattern analysis on %d events", len(s.events))
}

// GetServiceStatus returns the current service status
func (s *AccessControlService) GetServiceStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"devices":    len(s.devices),
		"cards":      len(s.cards),
		"rules":      len(s.rules),
		"zones":      len(s.zones),
		"events":     len(s.events),
		"ai_enabled": s.config.EnableAIAnalytics,
	}
}

// generateID generates a random ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

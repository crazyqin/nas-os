package geoipfirewall

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLogger struct{}

func (l *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Error(msg string, keysAndValues ...interface{}) {}
func (l *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (l *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	config := DefaultConfig()
	config.GeoDBPath = tmpDir + "/geoip/test.mmdb"
	mgr, err := NewManager(config, &mockLogger{})
	require.NoError(t, err)
	return mgr
}

func TestCheckIP_DefaultAllow(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	result, err := mgr.CheckIP("8.8.8.8")
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, ActionAllow, result.Action)
}

func TestCheckIP_InvalidIP(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	_, err := mgr.CheckIP("not-an-ip")
	assert.Error(t, err)
}

func TestBlockCountry(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// Block CN
	err := mgr.BlockCountry("CN")
	require.NoError(t, err)

	// Verify it's in the block list
	stats := mgr.GetStats()
	blocked := stats["blockedCountries"].([]string)
	assert.Contains(t, blocked, "CN")
}

func TestUnblockCountry(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	mgr.BlockCountry("CN")
	mgr.UnblockCountry("CN")

	stats := mgr.GetStats()
	blocked := stats["blockedCountries"].([]string)
	assert.NotContains(t, blocked, "CN")
}

func TestAddAndListRules(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	rule := &Rule{
		Name:     "Block Tor Exit Nodes",
		Action:   ActionDeny,
		Priority: 100,
		Enabled:  true,
		Regions:  []string{"T1"}, // Tor
	}
	err := mgr.AddRule(rule)
	require.NoError(t, err)
	assert.NotEmpty(t, rule.ID)

	rules := mgr.ListRules()
	assert.Len(t, rules, 1)
	assert.Equal(t, "Block Tor Exit Nodes", rules[0].Name)
}

func TestRuleWithIPRange(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	rule := &Rule{
		Name:     "Block Range",
		Action:   ActionDeny,
		Priority: 50,
		Enabled:  true,
		IPRanges: []string{"192.168.1.0/24"},
	}
	err := mgr.AddRule(rule)
	require.NoError(t, err)

	// IP in range should match
	result, err := mgr.CheckIP("192.168.1.100")
	require.NoError(t, err)
	assert.Equal(t, ActionDeny, result.Action)
	assert.False(t, result.Allowed)
}

func TestDeleteRule(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	rule := &Rule{Name: "Test", Action: ActionDeny, Enabled: true}
	mgr.AddRule(rule)

	err := mgr.DeleteRule(rule.ID)
	require.NoError(t, err)

	_, err = mgr.GetRule(rule.ID)
	assert.Error(t, err)
}

func TestUpdateRule(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	rule := &Rule{Name: "Old Name", Action: ActionDeny, Enabled: true}
	mgr.AddRule(rule)

	rule.Name = "New Name"
	err := mgr.UpdateRule(rule)
	require.NoError(t, err)

	fetched, _ := mgr.GetRule(rule.ID)
	assert.Equal(t, "New Name", fetched.Name)
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	stats := mgr.GetStats()
	assert.NotNil(t, stats)
	assert.Equal(t, true, stats["enabled"])
}

func TestGetBlockedConnections(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	blocked := mgr.GetBlockedConnections(10)
	assert.NotNil(t, blocked)
	assert.Len(t, blocked, 0)
}

func TestHandler_Check(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geoip/check?ip=1.2.3.4", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result CheckResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
}

func TestHandler_Rules(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Create rule
	rule := Rule{Name: "Test Rule", Action: ActionDeny, Priority: 10, Enabled: true}
	body, _ := json.Marshal(rule)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/geoip/rules", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// List rules
	req = httptest.NewRequest(http.MethodGet, "/api/v1/geoip/rules", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var rules []*Rule
	json.Unmarshal(w.Body.Bytes(), &rules)
	assert.Len(t, rules, 1)
}

func TestHandler_Stats(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/geoip/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGeoDatabase_Lookup(t *testing.T) {
	geoDB := &GeoDatabase{
		entries: map[string]*GeoEntry{
			"192.168": {CountryCode: "US", CountryName: "United States"},
		},
		countries: make(map[string]*CountryInfo),
	}

	// Should find matching entry
	entry := geoDB.Lookup(net.ParseIP("192.168.1.1"))
	assert.NotNil(t, entry)
	assert.Equal(t, "US", entry.CountryCode)

	// Should not find non-matching entry
	entry = geoDB.Lookup(net.ParseIP("10.0.0.1"))
	assert.Nil(t, entry)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, ActionAllow, config.DefaultAction)
	assert.True(t, config.EnableIPv6)
	assert.Equal(t, 10000, config.MaxLogEntries)
}

func TestRulePrioritySorting(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	mgr.AddRule(&Rule{Name: "Low", Action: ActionAllow, Priority: 10, Enabled: true})
	mgr.AddRule(&Rule{Name: "High", Action: ActionDeny, Priority: 100, Enabled: true})
	mgr.AddRule(&Rule{Name: "Mid", Action: ActionLog, Priority: 50, Enabled: true})

	rules := mgr.ListRules()
	assert.Len(t, rules, 3)
	assert.Equal(t, "High", rules[0].Name)
	assert.Equal(t, "Mid", rules[1].Name)
	assert.Equal(t, "Low", rules[2].Name)
}

func TestHandler_Lookup(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Valid IP lookup
	body, _ := json.Marshal(map[string]string{"ip": "8.8.8.8"})
	req := httptest.NewRequest(http.MethodPost, "/api/geoipfirewall/lookup", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var entry GeoEntry
	err := json.Unmarshal(w.Body.Bytes(), &entry)
	require.NoError(t, err)

	// Invalid IP
	body, _ = json.Marshal(map[string]string{"ip": "bad-ip"})
	req = httptest.NewRequest(http.MethodPost, "/api/geoipfirewall/lookup", bytes.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Empty IP
	body, _ = json.Marshal(map[string]string{"ip": ""})
	req = httptest.NewRequest(http.MethodPost, "/api/geoipfirewall/lookup", bytes.NewReader(body))
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Countries(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	handler := NewHandler(mgr)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/geoipfirewall/countries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var countries []CountryInfo
	err := json.Unmarshal(w.Body.Bytes(), &countries)
	require.NoError(t, err)
	assert.NotNil(t, countries)
}

func TestLookupIP(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	// Unknown IP should return unknown entry
	entry, err := mgr.LookupIP("1.2.3.4")
	require.NoError(t, err)
	assert.NotNil(t, entry)
	assert.Equal(t, "--", entry.CountryCode)

	// Invalid IP
	_, err = mgr.LookupIP("not-an-ip")
	assert.Error(t, err)
}

func TestGetCountries(t *testing.T) {
	mgr := setupTestManager(t)
	defer mgr.Stop()

	countries := mgr.GetCountries()
	assert.NotNil(t, countries)
}

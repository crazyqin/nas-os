package downloader

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewTransmissionClient(t *testing.T) {
	client := NewTransmissionClient("http://localhost:9091", "user", "pass")

	if client == nil {
		t.Fatal("NewTransmissionClient() returned nil")
	}

	if client.url != "http://localhost:9091/transmission/rpc" {
		t.Errorf("URL = %s, expected http://localhost:9091/transmission/rpc", client.url)
	}

	if client.username != "user" {
		t.Errorf("Username = %s, expected 'user'", client.username)
	}

	if client.password != "pass" {
		t.Errorf("Password = %s, expected 'pass'", client.password)
	}
}

func TestNewTransmissionClientTrailingSlash(t *testing.T) {
	client := NewTransmissionClient("http://localhost:9091/", "user", "pass")

	if client.url != "http://localhost:9091/transmission/rpc" {
		t.Errorf("URL = %s, expected http://localhost:9091/transmission/rpc", client.url)
	}
}

func TestNewQBittorrentClient(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:8080", "admin", "password")

	if client == nil {
		t.Fatal("NewQBittorrentClient() returned nil")
	}

	if client.url != "http://localhost:8080/api/v2" {
		t.Errorf("URL = %s, expected http://localhost:8080/api/v2", client.url)
	}

	if client.username != "admin" {
		t.Errorf("Username = %s, expected 'admin'", client.username)
	}

	if client.password != "password" {
		t.Errorf("Password = %s, expected 'password'", client.password)
	}
}

func TestNewQBittorrentClientTrailingSlash(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:8080/", "admin", "password")

	if client.url != "http://localhost:8080/api/v2" {
		t.Errorf("URL = %s, expected http://localhost:8080/api/v2", client.url)
	}
}

func TestTransmissionRequest_Structure(t *testing.T) {
	req := TransmissionRequest{
		Method:    "torrent-get",
		Arguments: map[string]interface{}{"ids": []int{1}},
		Tag:       123,
	}

	if req.Method != "torrent-get" {
		t.Errorf("Method = %s, expected 'torrent-get'", req.Method)
	}

	if req.Tag != 123 {
		t.Errorf("Tag = %d, expected 123", req.Tag)
	}
}

func TestTransmissionResponse_Structure(t *testing.T) {
	resp := TransmissionResponse{
		Result:    "success",
		Arguments: json.RawMessage(`{"torrents": []}`),
		Tag:       456,
	}

	if resp.Result != "success" {
		t.Errorf("Result = %s, expected 'success'", resp.Result)
	}

	if resp.Tag != 456 {
		t.Errorf("Tag = %d, expected 456", resp.Tag)
	}
}

func TestTransmissionTorrentAddRequest_Structure(t *testing.T) {
	req := TransmissionTorrentAddRequest{
		Filename:    "https://example.com/test.torrent",
		DownloadDir: "/downloads",
		Paused:      false,
	}

	if req.Filename != "https://example.com/test.torrent" {
		t.Errorf("Filename = %s", req.Filename)
	}

	if req.DownloadDir != "/downloads" {
		t.Errorf("DownloadDir = %s", req.DownloadDir)
	}

	if req.Paused {
		t.Error("Paused should be false")
	}
}

func TestTransmissionTorrentGetRequest_Structure(t *testing.T) {
	req := TransmissionTorrentGetRequest{
		IDs:    []int{1, 2, 3},
		Fields: []string{"id", "name", "status"},
	}

	if len(req.IDs) != 3 {
		t.Errorf("IDs count = %d, expected 3", len(req.IDs))
	}

	if len(req.Fields) != 3 {
		t.Errorf("Fields count = %d, expected 3", len(req.Fields))
	}
}

func TestTransmissionTorrent_Structure(t *testing.T) {
	torrent := TransmissionTorrent{
		ID:             1,
		Name:           "test.torrent",
		HashString:     "abc123",
		Status:         4,
		TotalSize:      1024 * 1024 * 100,
		DownloadedEver: 1024 * 1024 * 50,
		UploadedEver:   1024 * 1024 * 25,
		PercentDone:    0.5,
		RateDownload:   1024 * 100,
		RateUpload:     1024 * 50,
		PeersConnected: 10,
		Seeders:        5,
		Leechers:       10,
		DownloadDir:    "/downloads",
		Error:          0,
		ErrorString:    "",
	}

	if torrent.ID != 1 {
		t.Errorf("ID = %d, expected 1", torrent.ID)
	}

	if torrent.Name != "test.torrent" {
		t.Errorf("Name = %s, expected 'test.torrent'", torrent.Name)
	}

	if torrent.PercentDone != 0.5 {
		t.Errorf("PercentDone = %f, expected 0.5", torrent.PercentDone)
	}
}

func TestTransmissionTorrentRemoveRequest_Structure(t *testing.T) {
	req := TransmissionTorrentRemoveRequest{
		IDs:             []int{1, 2},
		DeleteLocalData: true,
	}

	if len(req.IDs) != 2 {
		t.Errorf("IDs count = %d, expected 2", len(req.IDs))
	}

	if !req.DeleteLocalData {
		t.Error("DeleteLocalData should be true")
	}
}

func TestTransmissionSessionStatsResponse_Structure(t *testing.T) {
	stats := TransmissionSessionStatsResponse{
		ActiveTorrentCount: 5,
		PausedTorrentCount: 2,
		TorrentCount:       7,
		CumulativeStats: Stats{
			DownloadedBytes: 1024 * 1024 * 1000,
			UploadedBytes:   1024 * 1024 * 500,
			FilesAdded:      100,
			SessionCount:    10,
			SecondsActive:   3600,
		},
	}

	if stats.ActiveTorrentCount != 5 {
		t.Errorf("ActiveTorrentCount = %d, expected 5", stats.ActiveTorrentCount)
	}

	if stats.TorrentCount != 7 {
		t.Errorf("TorrentCount = %d, expected 7", stats.TorrentCount)
	}

	if stats.CumulativeStats.DownloadedBytes != 1024*1024*1000 {
		t.Errorf("DownloadedBytes = %d", stats.CumulativeStats.DownloadedBytes)
	}
}

func TestStats_Structure(t *testing.T) {
	stats := Stats{
		DownloadedBytes: 1000,
		UploadedBytes:   500,
		FilesAdded:      10,
		SessionCount:    5,
		SecondsActive:   3600,
	}

	if stats.DownloadedBytes != 1000 {
		t.Errorf("DownloadedBytes = %d, expected 1000", stats.DownloadedBytes)
	}

	if stats.UploadedBytes != 500 {
		t.Errorf("UploadedBytes = %d, expected 500", stats.UploadedBytes)
	}
}

func TestQBittorrentTorrentInfo_Structure(t *testing.T) {
	info := QBittorrentTorrentInfo{
		Hash:        "abc123def456",
		Name:        "test.torrent",
		Progress:    0.75,
		Dlspeed:     1024 * 100,
		Upspeed:     1024 * 50,
		Downloaded:  1024 * 1024 * 100,
		Uploaded:    1024 * 1024 * 50,
		NumSeeds:    10,
		NumLeechs:   5,
		State:       "downloading",
		SavePath:    "/downloads",
		TotalSize:   1024 * 1024 * 200,
		Ratio:       0.5,
		Category:    "movies",
		Tags:        "hd,1080p",
	}

	if info.Hash != "abc123def456" {
		t.Errorf("Hash = %s", info.Hash)
	}

	if info.Name != "test.torrent" {
		t.Errorf("Name = %s", info.Name)
	}

	if info.Progress != 0.75 {
		t.Errorf("Progress = %f, expected 0.75", info.Progress)
	}

	if info.State != "downloading" {
		t.Errorf("State = %s", info.State)
	}
}

func TestTransmissionClient_GetSession(t *testing.T) {
	// Create a test server that returns 409 with session ID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/transmission/rpc" {
			w.Header().Set("X-Transmission-Session-Id", "test-session-id")
			w.WriteHeader(http.StatusConflict)
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "")
	err := client.getSession()

	if err != nil {
		t.Errorf("getSession() returned error: %v", err)
	}

	if client.session != "test-session-id" {
		t.Errorf("session = %s, expected 'test-session-id'", client.session)
	}
}

func TestTransmissionClient_GetSessionAlreadySet(t *testing.T) {
	client := NewTransmissionClient("http://localhost:9091", "", "")
	client.session = "existing-session"

	err := client.getSession()

	if err != nil {
		t.Errorf("getSession() returned error: %v", err)
	}

	if client.session != "existing-session" {
		t.Errorf("session should remain 'existing-session'")
	}
}

func TestTransmissionClient_doRequestWithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for basic auth
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("Basic auth not set")
		}
		if username != "testuser" || password != "testpass" {
			t.Errorf("Basic auth = %s:%s, expected testuser:testpass", username, password)
		}

		// Check for session header
		if r.Header.Get("X-Transmission-Session-Id") == "" {
			w.Header().Set("X-Transmission-Session-Id", "session-123")
			w.WriteHeader(http.StatusConflict)
			return
		}

		// Return success
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"success","arguments":{}}`))
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "testuser", "testpass")
	resp, err := client.doRequest("session-stats", nil)

	if err != nil {
		t.Errorf("doRequest() returned error: %v", err)
	}

	if resp.Result != "success" {
		t.Errorf("Result = %s, expected 'success'", resp.Result)
	}
}

func TestTransmissionClient_AddTorrent_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	_, _, err := client.AddTorrent("https://example.com/test.torrent", "/downloads")

	if err == nil {
		t.Error("AddTorrent should fail with invalid URL")
	}
}

func TestTransmissionClient_GetTorrents_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	_, err := client.GetTorrents()

	if err == nil {
		t.Error("GetTorrents should fail with invalid URL")
	}
}

func TestQBittorrentClient_login_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.login()

	if err == nil {
		t.Error("login should fail with invalid URL")
	}
}

func TestQBittorrentClient_AddTorrent_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.AddTorrent("magnet:?xt=urn:btih:test", "/downloads")

	if err == nil {
		t.Error("AddTorrent should fail with invalid URL")
	}
}

func TestQBittorrentClient_GetTorrents_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	_, err := client.GetTorrents()

	if err == nil {
		t.Error("GetTorrents should fail with invalid URL")
	}
}

func TestQBittorrentClient_AddTorrentFile(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:8080", "", "")
	err := client.AddTorrentFile([]byte("torrent content"), "/downloads")

	if err == nil {
		t.Error("AddTorrentFile should return error (not supported)")
	}
}

func TestQBittorrentClient_login_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			// Return success with SID cookie
			http.SetCookie(w, &http.Cookie{
				Name:  "SID",
				Value: "test-sid-value",
			})
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "admin", "password")
	err := client.login()

	if err != nil {
		t.Errorf("login() returned error: %v", err)
	}

	if client.cookie != "test-sid-value" {
		t.Errorf("cookie = %s, expected 'test-sid-value'", client.cookie)
	}
}

func TestQBittorrentClient_logout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{
				Name:  "SID",
				Value: "test-sid",
			})
			w.WriteHeader(http.StatusOK)
		}
		if r.URL.Path == "/api/v2/auth/logout" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "admin", "password")
	err := client.Logout()

	if err != nil {
		t.Errorf("Logout() returned error: %v", err)
	}

	if client.cookie != "" {
		t.Error("Cookie should be cleared after logout")
	}
}

func TestQBittorrentClient_doRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check login
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{
				Name:  "SID",
				Value: "test-sid",
			})
			w.WriteHeader(http.StatusOK)
			return
		}

		// Check for SID cookie
		cookie, err := r.Cookie("SID")
		if err != nil || cookie.Value != "test-sid" {
			t.Error("SID cookie not set correctly")
		}

		// Return test response
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "admin", "password")
	resp, err := client.doRequest("GET", "/test", nil)

	if err != nil {
		t.Errorf("doRequest() returned error: %v", err)
	}

	if string(resp) != `{"result":"ok"}` {
		t.Errorf("Response = %s", string(resp))
	}
}

func TestTransmissionTorrentAddResponse_Structure(t *testing.T) {
	resp := TransmissionTorrentAddResponse{}
	jsonData := `{"torrent-added":{"hashString":"abc123","id":1,"name":"test"}}`

	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if resp.TorrentAdded.HashString != "abc123" {
		t.Errorf("HashString = %s, expected 'abc123'", resp.TorrentAdded.HashString)
	}

	if resp.TorrentAdded.ID != 1 {
		t.Errorf("ID = %d, expected 1", resp.TorrentAdded.ID)
	}
}

func TestTransmissionTorrentGetResponse_Structure(t *testing.T) {
	jsonData := `{"torrents":[{"id":1,"name":"test","hashString":"abc","status":4}]}`
	var resp TransmissionTorrentGetResponse

	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(resp.Torrents) != 1 {
		t.Fatalf("Torrents count = %d, expected 1", len(resp.Torrents))
	}

	if resp.Torrents[0].Name != "test" {
		t.Errorf("Name = %s, expected 'test'", resp.Torrents[0].Name)
	}
}

func TestTransmissionClient_StartTorrent_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	err := client.StartTorrent(1)

	if err == nil {
		t.Error("StartTorrent should fail with invalid URL")
	}
}

func TestTransmissionClient_StopTorrent_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	err := client.StopTorrent(1)

	if err == nil {
		t.Error("StopTorrent should fail with invalid URL")
	}
}

func TestTransmissionClient_RemoveTorrent_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	err := client.RemoveTorrent(1, false)

	if err == nil {
		t.Error("RemoveTorrent should fail with invalid URL")
	}
}

func TestTransmissionClient_GetSessionStats_InvalidURL(t *testing.T) {
	client := NewTransmissionClient("http://localhost:99999", "", "")
	_, err := client.GetSessionStats()

	if err == nil {
		t.Error("GetSessionStats should fail with invalid URL")
	}
}

func TestQBittorrentClient_PauseTorrent_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.PauseTorrent("abc123")

	if err == nil {
		t.Error("PauseTorrent should fail with invalid URL")
	}
}

func TestQBittorrentClient_ResumeTorrent_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.ResumeTorrent("abc123")

	if err == nil {
		t.Error("ResumeTorrent should fail with invalid URL")
	}
}

func TestQBittorrentClient_DeleteTorrent_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.DeleteTorrent("abc123", false)

	if err == nil {
		t.Error("DeleteTorrent should fail with invalid URL")
	}
}

func TestQBittorrentClient_SetDownloadLimit_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.SetDownloadLimit("abc123", 1024000)

	if err == nil {
		t.Error("SetDownloadLimit should fail with invalid URL")
	}
}

func TestQBittorrentClient_SetUploadLimit_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	err := client.SetUploadLimit("abc123", 512000)

	if err == nil {
		t.Error("SetUploadLimit should fail with invalid URL")
	}
}

func TestQBittorrentClient_GetTransferInfo_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	_, err := client.GetTransferInfo()

	if err == nil {
		t.Error("GetTransferInfo should fail with invalid URL")
	}
}

func TestQBittorrentClient_GetTorrentByHash_InvalidURL(t *testing.T) {
	client := NewQBittorrentClient("http://localhost:99999", "", "")
	_, err := client.GetTorrentByHash("abc123")

	if err == nil {
		t.Error("GetTorrentByHash should fail with invalid URL")
	}
}

func TestTransmissionClient_GetTorrentByHash_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Transmission-Session-Id", "session-id")
		if r.Method == "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"result":"success","arguments":{"torrents":[]}}`))
		}
	}))
	defer server.Close()

	client := NewTransmissionClient(server.URL, "", "")
	_, err := client.GetTorrentByHash("nonexistent")

	if err == nil {
		t.Error("GetTorrentByHash should return error for nonexistent hash")
	}
}

func TestQBittorrentClient_GetTorrentByHash_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "sid"})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"hash":"abc123","name":"test.torrent"}]`))
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "")
	info, err := client.GetTorrentByHash("abc123")

	if err != nil {
		t.Errorf("GetTorrentByHash returned error: %v", err)
	}

	if info.Hash != "abc123" {
		t.Errorf("Hash = %s, expected 'abc123'", info.Hash)
	}
}

func TestQBittorrentClient_GetTorrentByHash_EmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "sid"})
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := NewQBittorrentClient(server.URL, "", "")
	_, err := client.GetTorrentByHash("nonexistent")

	if err == nil {
		t.Error("GetTorrentByHash should return error for empty result")
	}
}
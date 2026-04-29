package filerequest

import (
	"testing"
	"time"
)

func TestCreateRequest(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req, err := m.CreateRequest("Test Upload", "Please upload files", "user1", "/uploads/test", 10, 100, expires, false, false)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	if req.Title != "Test Upload" {
		t.Errorf("expected title 'Test Upload', got %q", req.Title)
	}
	if req.Status != StatusActive {
		t.Errorf("expected status active, got %q", req.Status)
	}
	if req.Token == "" {
		t.Error("expected non-empty token")
	}
	if req.MaxFiles != 10 {
		t.Errorf("expected max files 10, got %d", req.MaxFiles)
	}
}

func TestCreateRequestValidation(t *testing.T) {
	m := NewManager()

	// Missing title
	_, err := m.CreateRequest("", "desc", "user1", "/path", 0, 0, time.Now().Add(time.Hour), false, false)
	if err == nil {
		t.Error("expected error for empty title")
	}

	// Missing target path
	_, err = m.CreateRequest("Title", "desc", "user1", "", 0, 0, time.Now().Add(time.Hour), false, false)
	if err == nil {
		t.Error("expected error for empty target path")
	}

	// Past expiration
	_, err = m.CreateRequest("Title", "desc", "user1", "/path", 0, 0, time.Now().Add(-time.Hour), false, false)
	if err == nil {
		t.Error("expected error for past expiration")
	}
}

func TestRecordUpload(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Upload Test", "", "user1", "/uploads", 2, 50, expires, false, false)

	// First upload
	up1, err := m.RecordUpload(req.ID, "doc.pdf", 1024, "application/pdf", "192.168.1.1")
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	if up1.FileName != "doc.pdf" {
		t.Errorf("expected filename 'doc.pdf', got %q", up1.FileName)
	}

	// Second upload
	_, err = m.RecordUpload(req.ID, "image.jpg", 2048, "image/jpeg", "192.168.1.2")
	if err != nil {
		t.Fatalf("second RecordUpload failed: %v", err)
	}

	// Third upload should fail (max 2)
	_, err = m.RecordUpload(req.ID, "extra.txt", 100, "text/plain", "192.168.1.3")
	if err == nil {
		t.Error("expected error for exceeding max files")
	}

	// Check request auto-completed
	r, _ := m.GetRequest(req.ID)
	if r.Status != StatusCompleted {
		t.Errorf("expected status completed, got %q", r.Status)
	}
}

func TestUploadSizeLimit(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Size Test", "", "user1", "/uploads", 0, 1, expires, false, false) // 1MB limit

	_, err := m.RecordUpload(req.ID, "huge.bin", 2*1024*1024, "application/octet-stream", "10.0.0.1")
	if err == nil {
		t.Error("expected error for exceeding size limit")
	}
}

func TestRevokeRequest(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Revoke Test", "", "user1", "/uploads", 0, 0, expires, false, false)

	if err := m.RevokeRequest(req.ID); err != nil {
		t.Fatalf("RevokeRequest failed: %v", err)
	}

	// Should not be able to upload to revoked request
	_, err := m.RecordUpload(req.ID, "file.txt", 100, "text/plain", "10.0.0.1")
	if err == nil {
		t.Error("expected error for uploading to revoked request")
	}

	// Double revoke should fail
	if err := m.RevokeRequest(req.ID); err == nil {
		t.Error("expected error for double revoke")
	}
}

func TestGetRequestByToken(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Token Test", "", "user1", "/uploads", 0, 0, expires, false, false)

	found, err := m.GetRequestByToken(req.Token)
	if err != nil {
		t.Fatalf("GetRequestByToken failed: %v", err)
	}
	if found.ID != req.ID {
		t.Errorf("expected ID %q, got %q", req.ID, found.ID)
	}

	_, err = m.GetRequestByToken("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent token")
	}
}

func TestListRequests(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	m.CreateRequest("Req1", "", "user1", "/a", 0, 0, expires, false, false)
	m.CreateRequest("Req2", "", "user1", "/b", 0, 0, expires, false, false)
	m.CreateRequest("Req3", "", "user2", "/c", 0, 0, expires, false, false)

	list := m.ListRequests("user1")
	if len(list) != 2 {
		t.Errorf("expected 2 requests for user1, got %d", len(list))
	}

	list = m.ListRequests("user2")
	if len(list) != 1 {
		t.Errorf("expected 1 request for user2, got %d", len(list))
	}

	list = m.ListRequests("nobody")
	if len(list) != 0 {
		t.Errorf("expected 0 requests for nobody, got %d", len(list))
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	expires := time.Now().Add(24 * time.Hour)

	req1, _ := m.CreateRequest("S1", "", "user1", "/a", 0, 0, expires, false, false)
	m.CreateRequest("S2", "", "user1", "/b", 0, 0, expires, false, false)

	m.RecordUpload(req1.ID, "f1.txt", 1000, "text/plain", "10.0.0.1")
	m.RecordUpload(req1.ID, "f2.txt", 2000, "text/plain", "10.0.0.2")

	total, active, uploads, size := m.GetStats("user1")
	if total != 2 {
		t.Errorf("expected 2 total, got %d", total)
	}
	if active != 2 {
		t.Errorf("expected 2 active, got %d", active)
	}
	if uploads != 2 {
		t.Errorf("expected 2 uploads, got %d", uploads)
	}
	if size != 3000 {
		t.Errorf("expected 3000 bytes, got %d", size)
	}
}

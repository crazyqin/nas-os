package filerequest

import (
	"context"
	"testing"
	"time"
)

func TestCreateRequest(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	req, err := m.CreateRequest("Test Upload", "Please upload files", "user1", "/uploads/test", 10, 100, expires, false, false)
	if err != nil {
		t.Fatalf("CreateRequest failed: %v", err)
	}

	if req.Title != "Test Upload" {
		t.Errorf("expected title 'Test Upload', got %q", req.Title)
	}
	if req.Status != RequestStatusActive {
		t.Errorf("expected status active, got %q", req.Status)
	}
	if req.MaxFileCount != 10 {
		t.Errorf("expected max files 10, got %d", req.MaxFileCount)
	}

	// Get request
	got, err := m.GetRequest(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetRequest failed: %v", err)
	}
	if got.ID != req.ID {
		t.Errorf("expected ID %q, got %q", req.ID, got.ID)
	}
}

func TestRecordUpload(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Upload Test", "", "user1", "/uploads", 2, 50, expires, false, false)

	// First upload
	info1 := &UploadInfo{
		RequestID:    req.ID,
		OriginalName: "doc.pdf",
		FileSize:     1024,
		MimeType:     "application/pdf",
		UploaderIP:   "192.168.1.1",
	}
	err := m.RecordUpload(ctx, req.ID, info1)
	if err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}
	if info1.OriginalName != "doc.pdf" {
		t.Errorf("expected filename 'doc.pdf', got %q", info1.OriginalName)
	}

	// Second upload
	info2 := &UploadInfo{
		RequestID:    req.ID,
		OriginalName: "image.jpg",
		FileSize:     2048,
		MimeType:     "image/jpeg",
		UploaderIP:   "192.168.1.2",
	}
	err = m.RecordUpload(ctx, req.ID, info2)
	if err != nil {
		t.Fatalf("second RecordUpload failed: %v", err)
	}

	// Check uploads
	uploads, err := m.GetUploads(ctx, req.ID)
	if err != nil {
		t.Fatalf("GetUploads failed: %v", err)
	}
	if len(uploads) != 2 {
		t.Errorf("expected 2 uploads, got %d", len(uploads))
	}
}

func TestRevokeRequest(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Revoke Test", "", "user1", "/uploads", 0, 0, expires, false, false)

	if err := m.RevokeRequest(ctx, req.ID); err != nil {
		t.Fatalf("RevokeRequest failed: %v", err)
	}

	// Check status
	got, _ := m.GetRequest(ctx, req.ID)
	if got.Status != RequestStatusClosed {
		t.Errorf("expected status closed, got %q", got.Status)
	}
}

func TestListRequests(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	m.CreateRequest("Req1", "", "user1", "/a", 0, 0, expires, false, false)
	m.CreateRequest("Req2", "", "user1", "/b", 0, 0, expires, false, false)
	m.CreateRequest("Req3", "", "user2", "/c", 0, 0, expires, false, false)

	list, total, err := m.ListRequests(ctx, "user1", "", 1, 10)
	if err != nil {
		t.Fatalf("ListRequests failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 requests for user1, got %d", len(list))
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}

	list, total, _ = m.ListRequests(ctx, "user2", "", 1, 10)
	if len(list) != 1 {
		t.Errorf("expected 1 request for user2, got %d", len(list))
	}
	if total != 1 {
		t.Errorf("expected total 1, got %d", total)
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	req1, _ := m.CreateRequest("S1", "", "user1", "/a", 0, 0, expires, false, false)
	m.CreateRequest("S2", "", "user1", "/b", 0, 0, expires, false, false)

	m.RecordUpload(ctx, req1.ID, &UploadInfo{
		RequestID:    req1.ID,
		OriginalName: "f1.txt",
		FileSize:     1000,
	})
	m.RecordUpload(ctx, req1.ID, &UploadInfo{
		RequestID:    req1.ID,
		OriginalName: "f2.txt",
		FileSize:     2000,
	})

	stats, err := m.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if stats.TotalRequests != 2 {
		t.Errorf("expected 2 total, got %d", stats.TotalRequests)
	}
	if stats.ActiveRequests != 2 {
		t.Errorf("expected 2 active, got %d", stats.ActiveRequests)
	}
	if stats.TotalUploads != 2 {
		t.Errorf("expected 2 uploads, got %d", stats.TotalUploads)
	}
	if stats.TotalUploadSize != 3000 {
		t.Errorf("expected 3000 bytes, got %d", stats.TotalUploadSize)
	}
}

func TestCreateLink(t *testing.T) {
	m := NewManager()
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)

	req, _ := m.CreateRequest("Link Test", "", "user1", "/uploads", 0, 0, expires, false, false)

	link, err := m.CreateLink(ctx, req.ID, &CreateLinkRequest{
		Password: "test123",
	})
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}
	if link.Token == "" {
		t.Error("expected non-empty token")
	}

	// Get request by token
	found, err := m.GetRequestByToken(ctx, link.Token)
	if err != nil {
		t.Fatalf("GetRequestByToken failed: %v", err)
	}
	if found.ID != req.ID {
		t.Errorf("expected ID %q, got %q", req.ID, found.ID)
	}
}

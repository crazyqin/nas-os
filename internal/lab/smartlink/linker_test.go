package smartlink

import (
	"sync"
	"testing"
	"time"
)

func TestCreateLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Test basic link creation
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.ID == "" {
		t.Error("expected non-empty ID")
	}
	if link.Token == "" {
		t.Error("expected non-empty Token")
	}
	if link.FileID != "file-123" {
		t.Errorf("expected FileID=file-123, got %s", link.FileID)
	}
	if link.CreatorID != "user-1" {
		t.Errorf("expected CreatorID=user-1, got %s", link.CreatorID)
	}
	if link.Permission != PermissionReadOnly {
		t.Errorf("expected Permission=read_only, got %s", link.Permission)
	}
	if !link.IsActive {
		t.Error("expected IsActive=true")
	}
	if link.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestCreateLinkWithPassword(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadWrite,
		Password:   "secret123",
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.Password != "secret123" {
		t.Errorf("expected Password=secret123, got %s", link.Password)
	}
}

func TestCreateLinkWithExpiration(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	expiresIn := 3600 // 1 hour
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		ExpiresIn:  &expiresIn,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be set")
	}

	if link.ExpiresAt.Before(time.Now()) {
		t.Error("expected ExpiresAt to be in the future")
	}
}

func TestCreateOneTimeLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionPreview,
		IsOneTime:  true,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if !link.IsOneTime {
		t.Error("expected IsOneTime=true")
	}
}

func TestAccessLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Access the link
	accessedLink, err := linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("AccessLink failed: %v", err)
	}

	if accessedLink.VisitCount != 1 {
		t.Errorf("expected VisitCount=1, got %d", accessedLink.VisitCount)
	}
}

func TestAccessLinkWithPassword(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a password-protected link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		Password:   "secret123",
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Access with wrong password
	_, err = linker.AccessLink(link.Token, "wrong", "192.168.1.1", "Mozilla/5.0")
	if err != ErrInvalidPassword {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}

	// Access with correct password
	_, err = linker.AccessLink(link.Token, "secret123", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("AccessLink failed: %v", err)
	}
}

func TestAccessOneTimeLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a one-time link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		IsOneTime:  true,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// First access should succeed
	_, err = linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("first AccessLink failed: %v", err)
	}

	// Second access should fail (link deactivated)
	_, err = linker.AccessLink(link.Token, "", "192.168.1.2", "Mozilla/5.0")
	if err != ErrLinkInactive {
		t.Fatalf("expected ErrLinkInactive, got %v", err)
	}
}

func TestAccessExpiredLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link with short expiration
	expiresIn := 1 // 1 second
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		ExpiresIn:  &expiresIn,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Access should fail
	_, err = linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	if err != ErrLinkExpired {
		t.Fatalf("expected ErrLinkExpired, got %v", err)
	}
}

func TestAccessMaxVisitsReached(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link with max visits = 2
	maxVisits := 2
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		MaxVisits:  maxVisits,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// First access
	_, err = linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("first AccessLink failed: %v", err)
	}

	// Second access
	_, err = linker.AccessLink(link.Token, "", "192.168.1.2", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("second AccessLink failed: %v", err)
	}

	// Third access should fail
	_, err = linker.AccessLink(link.Token, "", "192.168.1.3", "Mozilla/5.0")
	if err != ErrMaxVisitsReached {
		t.Fatalf("expected ErrMaxVisitsReached, got %v", err)
	}
}

func TestAccessNonExistentLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	_, err := linker.AccessLink("nonexistent", "", "192.168.1.1", "Mozilla/5.0")
	if err != ErrLinkNotFound {
		t.Fatalf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestDeactivateLink(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Deactivate the link
	err = linker.DeactivateLink(link.ID, "user-1")
	if err != nil {
		t.Fatalf("DeactivateLink failed: %v", err)
	}

	// Access should fail
	_, err = linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	if err != ErrLinkInactive {
		t.Fatalf("expected ErrLinkInactive, got %v", err)
	}
}

func TestDeactivateLinkUnauthorized(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Try to deactivate with wrong user
	err = linker.DeactivateLink(link.ID, "user-2")
	if err == nil {
		t.Fatal("expected error for unauthorized deactivation")
	}
}

func TestGetLinkStats(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Access from multiple IPs
	linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	linker.AccessLink(link.Token, "", "192.168.1.2", "Mozilla/5.0")
	linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0") // duplicate IP

	stats, err := linker.GetLinkStats(link.ID)
	if err != nil {
		t.Fatalf("GetLinkStats failed: %v", err)
	}

	if stats.TotalVisits != 3 {
		t.Errorf("expected TotalVisits=3, got %d", stats.TotalVisits)
	}
	if stats.UniqueVisitors != 2 {
		t.Errorf("expected UniqueVisitors=2, got %d", stats.UniqueVisitors)
	}
	if stats.LastAccessedAt == nil {
		t.Error("expected LastAccessedAt to be set")
	}
	if stats.FirstAccessedAt == nil {
		t.Error("expected FirstAccessedAt to be set")
	}
}

func TestGetAccessLogs(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Access multiple times
	linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	linker.AccessLink(link.Token, "", "192.168.1.2", "Mozilla/5.0")

	logs := linker.GetAccessLogs(link.ID, 10, 0)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestListLinksByFileID(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create multiple links for same file
	req1 := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}
	req2 := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadWrite,
	}
	req3 := CreateLinkRequest{
		FileID:     "file-456",
		Permission: PermissionReadOnly,
	}

	linker.CreateLink("user-1", req1)
	linker.CreateLink("user-1", req2)
	linker.CreateLink("user-1", req3)

	links := linker.ListLinksByFileID("file-123")
	if len(links) != 2 {
		t.Errorf("expected 2 links, got %d", len(links))
	}
}

func TestListLinksByCreator(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create links for different users
	req1 := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}
	req2 := CreateLinkRequest{
		FileID:     "file-456",
		Permission: PermissionReadOnly,
	}

	linker.CreateLink("user-1", req1)
	linker.CreateLink("user-2", req2)

	links := linker.ListLinksByCreator("user-1")
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}
}

func TestCleanupExpiredLinks(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link that will expire quickly
	expiresIn := 1 // 1 second
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		ExpiresIn:  &expiresIn,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Wait for expiration
	time.Sleep(2 * time.Second)

	// Cleanup
	removed := linker.CleanupExpiredLinks()
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Link should no longer exist
	_, err = linker.GetLink(link.ID)
	if err != ErrLinkNotFound {
		t.Fatalf("expected ErrLinkNotFound, got %v", err)
	}
}

func TestPolicyMaxLinksPerUser(t *testing.T) {
	policy := DefaultSharePolicy()
	policy.MaxLinksPerUser = 2
	linker := NewLinker(policy)

	// Create max links
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}

	_, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("first CreateLink failed: %v", err)
	}

	_, err = linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("second CreateLink failed: %v", err)
	}

	// Third should fail
	_, err = linker.CreateLink("user-1", req)
	if err != ErrPolicyViolation {
		t.Fatalf("expected ErrPolicyViolation, got %v", err)
	}
}

func TestPolicyMaxVisitsPerLink(t *testing.T) {
	policy := DefaultSharePolicy()
	policy.MaxVisitsPerLink = 100
	linker := NewLinker(policy)

	// Try to create link with more than allowed visits
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		MaxVisits:  200,
	}

	_, err := linker.CreateLink("user-1", req)
	if err != ErrPolicyViolation {
		t.Fatalf("expected ErrPolicyViolation, got %v", err)
	}
}

func TestInvalidPermission(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: SharePermission("invalid"),
	}

	_, err := linker.CreateLink("user-1", req)
	if err != ErrInvalidPermission {
		t.Fatalf("expected ErrInvalidPermission, got %v", err)
	}
}

func TestConcurrentAccess(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
		MaxVisits:  100,
	}

	link, err := linker.CreateLink("user-1", req)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	// Concurrent access
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
			if err != nil {
				t.Errorf("AccessLink failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Verify stats
	stats, err := linker.GetLinkStats(link.ID)
	if err != nil {
		t.Fatalf("GetLinkStats failed: %v", err)
	}

	if stats.TotalVisits != 10 {
		t.Errorf("expected TotalVisits=10, got %d", stats.TotalVisits)
	}
}

func TestConcurrentCreation(t *testing.T) {
	linker := NewLinker(DefaultSharePolicy())

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := CreateLinkRequest{
				FileID:     "file-123",
				Permission: PermissionReadOnly,
			}
			_, err := linker.CreateLink("user-1", req)
			if err != nil {
				t.Errorf("CreateLink failed: %v", err)
			}
		}()
	}
	wg.Wait()

	// Verify all links created
	links := linker.ListLinksByCreator("user-1")
	if len(links) != 10 {
		t.Errorf("expected 10 links, got %d", len(links))
	}
}

func BenchmarkCreateLink(b *testing.B) {
	linker := NewLinker(DefaultSharePolicy())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := CreateLinkRequest{
			FileID:     "file-123",
			Permission: PermissionReadOnly,
		}
		linker.CreateLink("user-1", req)
	}
}

func BenchmarkAccessLink(b *testing.B) {
	linker := NewLinker(DefaultSharePolicy())

	// Create a link
	req := CreateLinkRequest{
		FileID:     "file-123",
		Permission: PermissionReadOnly,
	}
	link, _ := linker.CreateLink("user-1", req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		linker.AccessLink(link.Token, "", "192.168.1.1", "Mozilla/5.0")
	}
}

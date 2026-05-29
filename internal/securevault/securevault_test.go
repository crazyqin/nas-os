package securevault

import (
	"testing"
)

func TestNewSecureVault(t *testing.T) {
	sv := NewSecureVault("testpassword")
	if sv == nil {
		t.Fatal("expected non-nil SecureVault")
	}
	if sv.IsLocked() {
		t.Error("expected vault to be unlocked")
	}
}

func TestLockUnlock(t *testing.T) {
	sv := NewSecureVault("testpassword")

	sv.Lock()
	if !sv.IsLocked() {
		t.Error("expected vault to be locked")
	}

	err := sv.Unlock("testpassword")
	if err != nil {
		t.Fatal(err)
	}
	if sv.IsLocked() {
		t.Error("expected vault to be unlocked")
	}
}

func TestUnlockWrongPassword(t *testing.T) {
	sv := NewSecureVault("testpassword")
	sv.Lock()

	err := sv.Unlock("wrongpassword")
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestStoreRetrieve(t *testing.T) {
	sv := NewSecureVault("testpassword")

	entry, err := sv.Store("test-entry", "passwords", "secret-data-123")
	if err != nil {
		t.Fatal(err)
	}

	data, err := sv.Retrieve(entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if data != "secret-data-123" {
		t.Errorf("expected 'secret-data-123', got '%s'", data)
	}
}

func TestStoreLocked(t *testing.T) {
	sv := NewSecureVault("testpassword")
	sv.Lock()

	_, err := sv.Store("test", "cat", "data")
	if err == nil {
		t.Error("expected error when locked")
	}
}

func TestDelete(t *testing.T) {
	sv := NewSecureVault("testpassword")

	entry, _ := sv.Store("test", "cat", "data")
	err := sv.Delete(entry.ID)
	if err != nil {
		t.Fatal(err)
	}

	_, err = sv.Retrieve(entry.ID)
	if err == nil {
		t.Error("expected error for deleted entry")
	}
}

func TestListEntries(t *testing.T) {
	sv := NewSecureVault("testpassword")

	sv.Store("entry1", "cat1", "data1")
	sv.Store("entry2", "cat2", "data2")

	entries := sv.ListEntries()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSearchByCategory(t *testing.T) {
	sv := NewSecureVault("testpassword")

	sv.Store("entry1", "passwords", "data1")
	sv.Store("entry2", "notes", "data2")
	sv.Store("entry3", "passwords", "data3")

	results := sv.SearchByCategory("passwords")
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestSearchByTag(t *testing.T) {
	sv := NewSecureVault("testpassword")

	entry, _ := sv.Store("entry1", "cat", "data")
	entry.Tags = []string{"important", "work"}

	results := sv.SearchByTag("important")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestChangePassword(t *testing.T) {
	sv := NewSecureVault("oldpassword")

	sv.Store("entry1", "cat", "secret-data")

	err := sv.ChangePassword("oldpassword", "newpassword")
	if err != nil {
		t.Fatal(err)
	}

	// Verify data is still accessible
	entries := sv.ListEntries()
	if len(entries) != 1 {
		t.Fatal("expected 1 entry")
	}

	data, err := sv.Retrieve(entries[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if data != "secret-data" {
		t.Errorf("expected 'secret-data', got '%s'", data)
	}
}

func TestChangePasswordWrongOld(t *testing.T) {
	sv := NewSecureVault("oldpassword")

	err := sv.ChangePassword("wrongpassword", "newpassword")
	if err == nil {
		t.Error("expected error for wrong old password")
	}
}

func TestGetStats(t *testing.T) {
	sv := NewSecureVault("testpassword")

	sv.Store("entry1", "passwords", "data1")
	sv.Store("entry2", "notes", "data2")

	stats := sv.GetStats()
	if stats["total_entries"] != 2 {
		t.Errorf("expected 2 entries, got %v", stats["total_entries"])
	}
}

package digitalwellbeing

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	manager := NewManager(nil)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

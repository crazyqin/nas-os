package storage

import (
	"context"
	"testing"
)

func TestManagerHealthEmptyIsOK(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{}}
	if err := m.Health(context.Background()); err != nil {
		t.Fatalf("empty volumes should be healthy: %v", err)
	}
}

func TestManagerHealthUnhealthyVolumeFails(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{
		"bad": {Name: "bad", Status: VolumeStatus{Healthy: false}},
	}}
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("expected unhealthy volume to fail Health")
	}
}

func TestManagerHealthNilFails(t *testing.T) {
	var m *Manager
	if err := m.Health(context.Background()); err == nil {
		t.Fatal("nil manager must fail")
	}
}

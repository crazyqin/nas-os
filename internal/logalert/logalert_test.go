package logalert

import (
\t"testing"
)

func TestNewManager(t *testing.T) {
\tcfg := &Config{Enabled: true, Interval: 60}
\tm := NewManager(cfg)
\tif m == nil {
\t\tt.Fatal("NewManager returned nil")
\t}
}

func TestManagerStartStop(t *testing.T) {
\tcfg := &Config{Enabled: true, Interval: 60}
\tm := NewManager(cfg)
\t
\terr := m.Start()
\tif err != nil {
\t\tt.Fatalf("Start failed: %v", err)
\t}
\tif !m.IsRunning() {
\t\tt.Fatal("Manager should be running")
\t}
\t
\terr = m.Stop()
\tif err != nil {
\t\tt.Fatalf("Stop failed: %v", err)
\t}
\tif m.IsRunning() {
\t\tt.Fatal("Manager should be stopped")
\t}
}

package application

import (
	"testing"

	"nas-os/internal/config"
)

func TestFriendlyAddr(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{name: "all interfaces", cfg: config.ServerConfig{Host: "0.0.0.0", Port: 8080}, want: "localhost:8080"},
		{name: "empty host", cfg: config.ServerConfig{Port: 9090}, want: "localhost:9090"},
		{name: "specific host", cfg: config.ServerConfig{Host: "127.0.0.1", Port: 8081}, want: "127.0.0.1:8081"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FriendlyAddr(tt.cfg); got != tt.want {
				t.Fatalf("FriendlyAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

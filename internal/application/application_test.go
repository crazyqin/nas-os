package application

import (
	"errors"
	"reflect"
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

func TestCleanupStackRunsInReverseOrder(t *testing.T) {
	stack := &cleanupStack{}
	order := []string{}
	stack.add("first", func() error {
		order = append(order, "first")
		return nil
	})
	stack.add("second", func() error {
		order = append(order, "second")
		return nil
	})
	if err := stack.run(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	want := []string{"second", "first"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if len(stack.items) != 0 {
		t.Fatal("cleanup stack should be released after run")
	}
}

func TestCleanupStackAggregatesErrors(t *testing.T) {
	stack := &cleanupStack{}
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	stack.add("first", func() error { return firstErr })
	stack.add("second", func() error { return secondErr })
	err := stack.run()
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("expected joined cleanup errors, got %v", err)
	}
}

func TestCleanupStackReleaseSkipsCleanup(t *testing.T) {
	stack := &cleanupStack{}
	called := false
	stack.add("resource", func() error {
		called = true
		return nil
	})
	stack.release()
	if err := stack.run(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	if called {
		t.Fatal("released cleanup stack should not run callbacks")
	}
}

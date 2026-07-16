package mcpserver

import (
	"context"
	"testing"
	"time"
)

func TestNewMCPServer(t *testing.T) {
	config := &ServerConfig{
		Name:      "test-server",
		Version:   "1.0.0",
		Transport: TransportHTTP,
		Port:      8080,
	}
	server := NewMCPServer(config, nil)
	if server == nil {
		t.Fatal("NewMCPServer returned nil")
	}
	if server.name != "test-server" {
		t.Errorf("expected name 'test-server', got %q", server.name)
	}
	if server.IsRunning() {
		t.Error("new server should not be running")
	}
}

func TestRegisterTool(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	tool := &Tool{
		Name:        "test-tool",
		Description: "A test tool",
		Enabled:     true,
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
	}

	if err := server.RegisterTool(tool); err != nil {
		t.Fatalf("RegisterTool failed: %v", err)
	}

	tools := server.ListTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}

	// Duplicate registration should fail
	if err := server.RegisterTool(tool); err == nil {
		t.Error("duplicate registration should fail")
	}
}

func TestInvokeTool(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	server.RegisterTool(&Tool{
		Name:    "echo",
		Enabled: true,
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return params["input"], nil
		},
	})

	result, err := server.InvokeTool(context.Background(), "echo", map[string]interface{}{"input": "hello"})
	if err != nil {
		t.Fatalf("InvokeTool failed: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}

	// Non-existent tool
	_, err = server.InvokeTool(context.Background(), "missing", nil)
	if err == nil {
		t.Error("invoking non-existent tool should fail")
	}
}

func TestRegisterResource(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	res := &Resource{
		URI:      "file:///test",
		Name:     "Test Resource",
		MimeType: "text/plain",
		Enabled:  true,
		Handler: func(ctx context.Context, uri string) ([]byte, error) {
			return []byte("content"), nil
		},
	}

	if err := server.RegisterResource(res); err != nil {
		t.Fatalf("RegisterResource failed: %v", err)
	}

	data, err := server.ReadResource(context.Background(), "file:///test")
	if err != nil {
		t.Fatalf("ReadResource failed: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("expected 'content', got %q", string(data))
	}
}

func TestRegisterPrompt(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	prompt := &Prompt{
		Name:        "greeting",
		Description: "A greeting prompt",
		Arguments: []PromptArgument{
			{Name: "name", Required: true},
		},
		Enabled: true,
		Handler: func(ctx context.Context, args map[string]string) (string, error) {
			return "Hello, " + args["name"], nil
		},
	}

	if err := server.RegisterPrompt(prompt); err != nil {
		t.Fatalf("RegisterPrompt failed: %v", err)
	}

	result, err := server.GetPrompt(context.Background(), "greeting", map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("GetPrompt failed: %v", err)
	}
	if result != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %q", result)
	}
}

func TestMetrics(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	server.RegisterTool(&Tool{
		Name:    "noop",
		Enabled: true,
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return nil, nil
		},
	})

	for i := 0; i < 5; i++ {
		server.InvokeTool(context.Background(), "noop", nil)
	}

	metrics := server.GetMetrics()
	if metrics.ToolInvocations != 5 {
		t.Errorf("expected 5 invocations, got %d", metrics.ToolInvocations)
	}
	if metrics.TotalRequests != 5 {
		t.Errorf("expected 5 total requests, got %d", metrics.TotalRequests)
	}
}

func TestEnableDisableTool(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	server.RegisterTool(&Tool{
		Name:    "toggle",
		Enabled: true,
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			return "ok", nil
		},
	})

	server.DisableTool("toggle")
	_, err := server.InvokeTool(context.Background(), "toggle", nil)
	if err == nil {
		t.Error("invoking disabled tool should fail")
	}

	server.EnableTool("toggle")
	_, err = server.InvokeTool(context.Background(), "toggle", nil)
	if err != nil {
		t.Errorf("invoking re-enabled tool should succeed: %v", err)
	}
}

func TestTimeout(t *testing.T) {
	config := &ServerConfig{Name: "test", Version: "1.0", Transport: TransportHTTP}
	server := NewMCPServer(config, nil)

	server.RegisterTool(&Tool{
		Name:    "slow",
		Enabled: true,
		Timeout: 50 * time.Millisecond,
		Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			time.Sleep(100 * time.Millisecond)
			return "done", nil
		},
	})

	// The tool should still execute (timeout enforcement is at transport layer)
	result, err := server.InvokeTool(context.Background(), "slow", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "done" {
		t.Errorf("expected 'done', got %v", result)
	}
}

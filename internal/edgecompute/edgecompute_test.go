package edgecompute

import (
	"context"
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:        true,
		MaxFunctions:   100,
		MaxWorkloads:   50,
		DefaultTimeout: 30,
		WasmEnabled:    true,
		GPUEnabled:     true,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	if err := manager.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	manager.Stop()
}

func TestDeployFunction(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	fn := &Function{
		Name:        "hello-world",
		Description: "Test function",
		Runtime:     RuntimeGo,
		Code:        `package main; func main() { println("hello") }`,
		Handler:     "main",
		Config: FunctionConfig{
			Timeout:    30,
			Memory:     128,
			MaxRetries: 3,
		},
	}

	if err := manager.DeployFunction(fn); err != nil {
		t.Fatalf("DeployFunction failed: %v", err)
	}

	if fn.ID == "" {
		t.Error("Function ID not generated")
	}

	if fn.State != StateActive {
		t.Errorf("Expected state active, got %s", fn.State)
	}
}

func TestInvokeFunction(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	fn := &Function{
		Name:    "echo",
		Runtime: RuntimeGo,
		Code:    `package main`,
		Handler: "main",
		Config: FunctionConfig{
			Timeout: 30,
			Memory:  128,
		},
	}

	manager.DeployFunction(fn)

	// Invoke function
	invocation, err := manager.InvokeFunction(context.Background(), fn.ID, map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("InvokeFunction failed: %v", err)
	}

	if invocation.Status != "success" {
		t.Errorf("Expected status success, got %s", invocation.Status)
	}

	if invocation.FunctionID != fn.ID {
		t.Errorf("Expected function ID %s, got %s", fn.ID, invocation.FunctionID)
	}
}

func TestInvokeNonExistentFunction(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	_, err := manager.InvokeFunction(context.Background(), "non-existent", nil)
	if err == nil {
		t.Error("Expected error for non-existent function")
	}
}

func TestDeleteFunction(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	fn := &Function{
		Name:    "test",
		Runtime: RuntimeGo,
	}

	manager.DeployFunction(fn)

	if err := manager.DeleteFunction(fn.ID); err != nil {
		t.Fatalf("DeleteFunction failed: %v", err)
	}

	// Try to get deleted function
	_, err := manager.GetFunction(fn.ID)
	if err == nil {
		t.Error("Expected error for deleted function")
	}
}

func TestListFunctions(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	// Deploy multiple functions
	for i := 0; i < 5; i++ {
		fn := &Function{
			Name:    "test",
			Runtime: RuntimeGo,
		}
		manager.DeployFunction(fn)
	}

	functions := manager.ListFunctions()

	if len(functions) != 5 {
		t.Errorf("Expected 5 functions, got %d", len(functions))
	}
}

func TestSubmitWorkload(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)

	wl := &Workload{
		Name:     "test-workload",
		Type:     "function",
		Priority: 1,
		Resources: ResourceRequest{
			CPU:    1,
			Memory: 256,
		},
	}

	if err := manager.SubmitWorkload(wl); err != nil {
		t.Fatalf("SubmitWorkload failed: %v", err)
	}

	if wl.ID == "" {
		t.Error("Workload ID not generated")
	}

	if wl.Status != "pending" {
		t.Errorf("Expected status pending, got %s", wl.Status)
	}
}

func TestFunctionRuntimes(t *testing.T) {
	runtimes := []FunctionRuntime{
		RuntimeGo, RuntimePython, RuntimeNode, RuntimeWasm, RuntimeContainer,
	}

	for _, rt := range runtimes {
		if string(rt) == "" {
			t.Errorf("Empty runtime: %v", rt)
		}
	}
}

func TestLocalNode(t *testing.T) {
	config := &Config{
		Enabled: true,
	}

	manager := NewManager(config)
	manager.registerLocalNode()

	nodes := manager.nodes

	if _, ok := nodes["local"]; !ok {
		t.Error("Local node not registered")
	}

	local := nodes["local"]
	if local.Status != "online" {
		t.Errorf("Expected local node online, got %s", local.Status)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		Enabled:     true,
		WasmEnabled: true,
		GPUEnabled:  true,
		AutoScaling: true,
	}

	manager := NewManager(config)

	// Deploy some functions
	manager.DeployFunction(&Function{Name: "fn1", Runtime: RuntimeGo})
	manager.DeployFunction(&Function{Name: "fn2", Runtime: RuntimePython})

	// Submit some workloads
	manager.SubmitWorkload(&Workload{Name: "wl1", Type: "function"})

	stats := manager.GetStats()

	if stats["total_functions"] != 2 {
		t.Errorf("Expected 2 functions, got %v", stats["total_functions"])
	}

	if stats["total_workloads"] != 1 {
		t.Errorf("Expected 1 workload, got %v", stats["total_workloads"])
	}

	if stats["wasm_enabled"] != true {
		t.Error("Expected wasm enabled")
	}

	if stats["gpu_enabled"] != true {
		t.Error("Expected gpu enabled")
	}
}

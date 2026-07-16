package airouter

import (
	"context"
	"testing"
)

func TestManager_CreateRoute(t *testing.T) {
	manager := NewManager()

	req := &CreateRouteRequest{
		Name:            "test-route",
		Strategy:        StrategyRoundRobin,
		ModelIDs:        []string{"model1", "model2", "model3"},
		FallbackEnabled: true,
		MaxRetries:      3,
		TimeoutMs:       5000,
	}

	route, err := manager.CreateRoute(req)
	if err != nil {
		t.Fatalf("CreateRoute failed: %v", err)
	}

	if route.Name != "test-route" {
		t.Errorf("expected name 'test-route', got '%s'", route.Name)
	}

	if len(route.Models) != 3 {
		t.Errorf("expected 3 models, got %d", len(route.Models))
	}
}

func TestManager_Select_RoundRobin(t *testing.T) {
	manager := NewManager()

	route, _ := manager.CreateRoute(&CreateRouteRequest{
		Name:     "rr-route",
		Strategy: StrategyRoundRobin,
		ModelIDs: []string{"m1", "m2", "m3"},
	})

	decision, err := manager.Select(context.Background(), &RouteRequest{
		RouteID: route.ID,
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if decision.SelectedModel == "" {
		t.Error("expected a selected model")
	}
}

func TestManager_Select_LeastLatency(t *testing.T) {
	manager := NewManager()

	route, _ := manager.CreateRoute(&CreateRouteRequest{
		Name:     "latency-route",
		Strategy: StrategyLeastLatency,
		ModelIDs: []string{"m1", "m2"},
	})

	decision, err := manager.Select(context.Background(), &RouteRequest{
		RouteID: route.ID,
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if decision.Strategy != StrategyLeastLatency {
		t.Errorf("expected strategy %s, got %s", StrategyLeastLatency, decision.Strategy)
	}
}

func TestManager_ReportResult(t *testing.T) {
	manager := NewManager()

	route, _ := manager.CreateRoute(&CreateRouteRequest{
		Name:     "test-route",
		Strategy: StrategyRoundRobin,
		ModelIDs: []string{"m1", "m2"},
	})

	decision, _ := manager.Select(context.Background(), &RouteRequest{
		RouteID: route.ID,
	})

	err := manager.ReportResult(context.Background(), decision, true, 100, 50)
	if err != nil {
		t.Fatalf("ReportResult failed: %v", err)
	}

	stats, err := manager.GetStats(context.Background(), route.ID)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalRequests != 1 {
		t.Errorf("expected 1 request, got %d", stats.TotalRequests)
	}
}

func TestManager_ListRoutes(t *testing.T) {
	manager := NewManager()

	manager.CreateRoute(&CreateRouteRequest{
		Name:     "route1",
		Strategy: StrategyRoundRobin,
		ModelIDs: []string{"m1"},
	})

	manager.CreateRoute(&CreateRouteRequest{
		Name:     "route2",
		Strategy: StrategyCostOptimized,
		ModelIDs: []string{"m2"},
	})

	routes := manager.ListRoutes()
	if len(routes) != 2 {
		t.Errorf("expected 2 routes, got %d", len(routes))
	}
}

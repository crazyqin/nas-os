package dockerresilience

import (
	"testing"
	"time"
)

func TestNewDockerResilience(t *testing.T) {
	dr := NewDockerResilience()
	if dr == nil {
		t.Fatal("NewDockerResilience returned nil")
	}
}

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy()
	if policy.MaxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", policy.MaxRetries)
	}
	if policy.BackoffFactor != 2.0 {
		t.Errorf("expected backoff 2.0, got %f", policy.BackoffFactor)
	}
}

func TestRecordRun(t *testing.T) {
	dr := NewDockerResilience()
	dr.RecordRun(WorkflowRun{
		ID:       "run-1",
		Workflow: "Docker Publish",
		Status:   "failure",
		StartedAt: time.Now(),
	})
	runs := dr.GetRuns()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
}

func TestShouldRetry(t *testing.T) {
	dr := NewDockerResilience()
	dr.RecordRun(WorkflowRun{
		ID:         "run-1",
		Workflow:   "Docker Publish",
		Status:     "failure",
		RetryCount: 1,
	})
	
	if !dr.ShouldRetry("run-1") {
		t.Fatal("expected should retry")
	}
	
	dr.RecordRun(WorkflowRun{
		ID:         "run-2",
		Workflow:   "Docker Publish",
		Status:     "failure",
		RetryCount: 3,
	})
	if dr.ShouldRetry("run-2") {
		t.Fatal("should not retry after max retries")
	}
}

func TestGetRetryDelay(t *testing.T) {
	dr := NewDockerResilience()
	delay0 := dr.GetRetryDelay(0)
	delay1 := dr.GetRetryDelay(1)
	delay2 := dr.GetRetryDelay(2)
	
	if delay0 >= delay1 {
		t.Errorf("delay should increase: %v >= %v", delay0, delay1)
	}
	if delay1 >= delay2 {
		t.Errorf("delay should increase: %v >= %v", delay1, delay2)
	}
}

func TestRunHealthChecks(t *testing.T) {
	dr := NewDockerResilience()
	checks := dr.RunHealthChecks()
	if len(checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(checks))
	}
}

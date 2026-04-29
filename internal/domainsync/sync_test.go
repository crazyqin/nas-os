package domainsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSyncEngine(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)
	require.NotNil(t, engine)

	assert.False(t, engine.IsRunning())
	assert.Equal(t, SyncStatusIdle, engine.GetStatus())
	assert.Equal(t, 0, engine.GetProgress())
	assert.Nil(t, engine.GetLastResult())
}

func TestSyncEngineGetConfig(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"
	cfg.Strategy = SyncStrategyIncremental

	engine := NewSyncEngine(cfg)
	loaded := engine.GetConfig()
	assert.Equal(t, "dc.example.com", loaded.DCConfig.Host)
	assert.Equal(t, SyncStrategyIncremental, loaded.Strategy)
}

func TestSyncEngineUpdateConfig(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)

	newCfg := DefaultSyncConfig()
	newCfg.DCConfig.Host = "dc2.example.com"
	newCfg.DCConfig.Domain = "new.example.com"
	newCfg.Strategy = SyncStrategyScheduled

	engine.UpdateConfig(newCfg)

	loaded := engine.GetConfig()
	assert.Equal(t, "dc2.example.com", loaded.DCConfig.Host)
	assert.Equal(t, SyncStrategyScheduled, loaded.Strategy)
}

func TestSyncEngineSyncOnceInprogress(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)

	// 模拟正在运行
	engine.mu.Lock()
	engine.running = true
	engine.mu.Unlock()

	ctx := context.Background()
	_, err := engine.SyncOnce(ctx)
	assert.ErrorIs(t, err, ErrSyncInProgress)
}

func TestSyncEngineSyncOnceConnectionFail(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "192.0.2.1" // TEST-NET，不会实际连接
	cfg.DCConfig.Domain = "test.local"
	cfg.DCConfig.ConnectTimeout = 100 * time.Millisecond

	engine := NewSyncEngine(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := engine.SyncOnce(ctx)
	// 连接会失败
	assert.Error(t, err)
	require.NotNil(t, result)
	assert.Equal(t, SyncStatusFailed, result.Status)
	assert.NotEmpty(t, result.ID)
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
	assert.True(t, result.Duration >= 0)
}

func TestSyncEngineStop(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)

	// 标记为 running
	engine.mu.Lock()
	engine.running = true
	engine.status = SyncStatusRunning
	engine.mu.Unlock()

	engine.Stop()

	assert.False(t, engine.IsRunning())
	assert.Equal(t, SyncStatusIdle, engine.GetStatus())
}

func TestSyncEngineStopNotRunning(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"

	engine := NewSyncEngine(cfg)

	// 不应该 panic
	engine.Stop()
	assert.False(t, engine.IsRunning())
}

func TestSyncEngineStartScheduledInprogress(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)

	// 模拟正在运行
	engine.mu.Lock()
	engine.running = true
	engine.mu.Unlock()

	ctx := context.Background()
	err := engine.StartScheduled(ctx)
	assert.ErrorIs(t, err, ErrSyncInProgress)
}

func TestSyncEngineTestConnectionFail(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "192.0.2.1"
	cfg.DCConfig.Domain = "test.local"
	cfg.DCConfig.ConnectTimeout = 100 * time.Millisecond

	engine := NewSyncEngine(cfg)

	ok, err := engine.TestConnection()
	assert.False(t, ok)
	assert.Error(t, err)
}

func TestSyncEngineConcurrentAccess(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "dc.example.com"
	cfg.DCConfig.Domain = "example.com"

	engine := NewSyncEngine(cfg)

	// 并发读取不应 panic
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			_ = engine.IsRunning()
			_ = engine.GetStatus()
			_ = engine.GetProgress()
			_ = engine.GetLastResult()
			_ = engine.GetConfig()
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSyncResultFields(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "192.0.2.1"
	cfg.DCConfig.Domain = "test.local"
	cfg.DCConfig.ConnectTimeout = 50 * time.Millisecond

	engine := NewSyncEngine(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, _ := engine.SyncOnce(ctx)

	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, SyncStrategyFull, result.Strategy)
	assert.False(t, result.StartTime.IsZero())
}

func TestSyncEngineSelectedOUs(t *testing.T) {
	cfg := DefaultSyncConfig()
	cfg.DCConfig.Host = "192.0.2.1"
	cfg.DCConfig.Domain = "test.local"
	cfg.DCConfig.ConnectTimeout = 50 * time.Millisecond
	cfg.SelectedOUs = []string{
		"OU=Engineering,DC=test,DC=local",
		"OU=HR,DC=test,DC=local",
	}

	engine := NewSyncEngine(cfg)

	// 验证配置保留了 SelectedOUs
	loaded := engine.GetConfig()
	assert.Len(t, loaded.SelectedOUs, 2)
}

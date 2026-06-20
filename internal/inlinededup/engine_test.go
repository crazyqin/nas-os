package inlinededup

import (
	"bytes"
	"log/slog"
	"testing"
)

func newTestEngine() *DedupEngine {
	return NewEngine(slog.Default(), DefaultConfig())
}

func TestProcessUniqueBlock(t *testing.T) {
	e := newTestEngine()

	data := []byte("hello world, this is unique data")
	info, status, err := e.ProcessBlock(data)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}
	if status != BlockUnique {
		t.Errorf("expected unique, got %s", status)
	}
	if info.RefCount != 1 {
		t.Errorf("expected refcount 1, got %d", info.RefCount)
	}
}

func TestProcessDuplicateBlock(t *testing.T) {
	e := newTestEngine()

	data := []byte("duplicate data block")
	_, _, _ = e.ProcessBlock(data) //nolint:dogsled
	info, status, err := e.ProcessBlock(data)
	if err != nil {
		t.Fatalf("ProcessBlock failed: %v", err)
	}
	if status != BlockDuplicate {
		t.Errorf("expected duplicate, got %s", status)
	}
	if info.RefCount != 2 {
		t.Errorf("expected refcount 2, got %d", info.RefCount)
	}
}

func TestDedupRatio(t *testing.T) {
	e := newTestEngine()

	// 写入 3 个唯一块 + 2 个重复块
	_, _, _ = e.ProcessBlock([]byte("block-a")) //nolint:dogsled
	_, _, _ = e.ProcessBlock([]byte("block-b")) //nolint:dogsled
	_, _, _ = e.ProcessBlock([]byte("block-c")) //nolint:dogsled
	_, _, _ = e.ProcessBlock([]byte("block-a")) //nolint:dogsled // 重复
	_, _, _ = e.ProcessBlock([]byte("block-b")) //nolint:dogsled // 重复

	stats := e.GetStats()
	if stats.TotalBlocks != 5 {
		t.Errorf("expected 5 total blocks, got %d", stats.TotalBlocks)
	}
	if stats.UniqueBlocks != 3 {
		t.Errorf("expected 3 unique blocks, got %d", stats.UniqueBlocks)
	}
	if stats.DuplicateBlocks != 2 {
		t.Errorf("expected 2 duplicate blocks, got %d", stats.DuplicateBlocks)
	}
	if stats.DedupRatio == 0 {
		t.Error("dedup ratio should be > 0")
	}
}

func TestEmptyBlock(t *testing.T) {
	e := newTestEngine()

	_, _, err := e.ProcessBlock([]byte{})
	if err == nil {
		t.Fatal("expected error for empty block")
	}
}

func TestProcessReader(t *testing.T) {
	e := newTestEngine()

	// 创建大于块大小的数据
	data := bytes.Repeat([]byte("test data "), 1000)
	reader := bytes.NewReader(data)
	writer := &bytes.Buffer{}

	stats, err := e.ProcessReader(reader, writer)
	if err != nil {
		t.Fatalf("ProcessReader failed: %v", err)
	}
	if stats.TotalBlocks == 0 {
		t.Error("expected some blocks to be processed")
	}
}

func TestStartStop(t *testing.T) {
	e := newTestEngine()

	if err := e.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := e.Start(); err == nil {
		t.Fatal("expected error for double start")
	}

	if err := e.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestGetBlockInfo(t *testing.T) {
	e := newTestEngine()

	data := []byte("test block info")
	info, _, _ := e.ProcessBlock(data)

	got, found := e.GetBlockInfo(info.Hash)
	if !found {
		t.Fatal("block not found in index")
	}
	if got.Hash != info.Hash {
		t.Errorf("hash mismatch: %s vs %s", got.Hash, info.Hash)
	}
}

func TestIndexSize(t *testing.T) {
	e := newTestEngine()

	_, _, _ = e.ProcessBlock([]byte("a")) //nolint:dogsled
	_, _, _ = e.ProcessBlock([]byte("b")) //nolint:dogsled
	_, _, _ = e.ProcessBlock([]byte("a")) //nolint:dogsled // 重复

	if e.IndexSize() != 2 {
		t.Errorf("expected index size 2, got %d", e.IndexSize())
	}
}

func TestResetStats(t *testing.T) {
	e := newTestEngine()

	_, _, _ = e.ProcessBlock([]byte("data")) //nolint:dogsled
	e.ResetStats()

	stats := e.GetStats()
	if stats.TotalBlocks != 0 {
		t.Errorf("expected 0 after reset, got %d", stats.TotalBlocks)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BlockSize != 128*1024 {
		t.Errorf("expected block size 131072, got %d", cfg.BlockSize)
	}
	if !cfg.EnableCompress {
		t.Error("expected compress enabled by default")
	}
}

func TestSpaceSavedCalculation(t *testing.T) {
	e := newTestEngine()

	data := []byte("repeated block data for savings calc")
	_, _, _ = e.ProcessBlock(data) //nolint:dogsled
	_, _, _ = e.ProcessBlock(data) //nolint:dogsled
	_, _, _ = e.ProcessBlock(data) //nolint:dogsled

	stats := e.GetStats()
	if stats.SavedBytes <= 0 {
		t.Error("expected saved bytes > 0 for duplicate blocks")
	}
	if stats.SpaceSavedPct <= 0 {
		t.Error("expected space saved pct > 0")
	}
}

package dedupengine

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sync"
	"testing"
)

// ========== 测试用辅助函数 ==========

// makeData 生成指定大小的测试数据.
func makeData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}
	return data
}

// hashOf 计算数据的 SHA-256 哈希.
func hashOf(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// ========== NewEngine 测试 ==========

func TestNewEngine_NilConfig(t *testing.T) {
	engine := NewEngine(nil)
	if engine == nil {
		t.Fatal("NewEngine(nil) 返回 nil")
	}
	if engine.config == nil {
		t.Fatal("默认配置未设置")
	}
	if engine.config.BlockSize != BlockSize64K {
		t.Errorf("默认块大小 = %v, want %v", engine.config.BlockSize, BlockSize64K)
	}
	if engine.config.MaxBlocks != 0 {
		t.Errorf("默认最大块数 = %d, want 0", engine.config.MaxBlocks)
	}
}

func TestNewEngine_CustomConfig(t *testing.T) {
	config := &EngineConfig{
		BlockSize: BlockSize4K,
		MaxBlocks: 1000,
	}
	engine := NewEngine(config)
	if engine.config.BlockSize != BlockSize4K {
		t.Errorf("块大小 = %v, want %v", engine.config.BlockSize, BlockSize4K)
	}
	if engine.config.MaxBlocks != 1000 {
		t.Errorf("最大块数 = %d, want 1000", engine.config.MaxBlocks)
	}
}

func TestNewEngine_InitializesBlocksMap(t *testing.T) {
	engine := NewEngine(nil)
	if engine.blocks == nil {
		t.Fatal("blocks map 未初始化")
	}
	if len(engine.blocks) != 0 {
		t.Errorf("初始块数 = %d, want 0", len(engine.blocks))
	}
}

// ========== DefaultEngineConfig 测试 ==========

func TestDefaultEngineConfig(t *testing.T) {
	config := DefaultEngineConfig()
	if config.BlockSize != BlockSize64K {
		t.Errorf("BlockSize = %v, want %v", config.BlockSize, BlockSize64K)
	}
	if config.MaxBlocks != 0 {
		t.Errorf("MaxBlocks = %d, want 0", config.MaxBlocks)
	}
}

// ========== BlockSize.String 测试 ==========

func TestBlockSize_String(t *testing.T) {
	tests := []struct {
		size BlockSize
		want string
	}{
		{BlockSize4K, "4KB"},
		{BlockSize64K, "64KB"},
		{BlockSize1M, "1MB"},
		{BlockSize(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.size.String(); got != tt.want {
				t.Errorf("BlockSize(%d).String() = %q, want %q", tt.size, got, tt.want)
			}
		})
	}
}

// ========== Store 测试 ==========

func TestStore_EmptyData(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Store([]byte{})
	if err != ErrInvalidData {
		t.Errorf("Store(empty) 错误 = %v, want %v", err, ErrInvalidData)
	}
}

func TestStore_NilData(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Store(nil)
	if err != ErrInvalidData {
		t.Errorf("Store(nil) 错误 = %v, want %v", err, ErrInvalidData)
	}
}

func TestStore_BasicStorage(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("hello world")
	hash, err := engine.Store(data)
	if err != nil {
		t.Fatalf("Store 失败: %v", err)
	}
	if hash == "" {
		t.Fatal("返回的哈希为空")
	}
	expectedHash := hashOf(data)
	if hash != expectedHash {
		t.Errorf("哈希 = %s, want %s", hash, expectedHash)
	}
}

func TestStore_DuplicateData(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("duplicate me")

	hash1, err := engine.Store(data)
	if err != nil {
		t.Fatalf("第一次 Store 失败: %v", err)
	}
	hash2, err := engine.Store(data)
	if err != nil {
		t.Fatalf("第二次 Store 失败: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("重复数据哈希不同: %s vs %s", hash1, hash2)
	}
	if engine.BlockCount() != 1 {
		t.Errorf("唯一块数 = %d, want 1", engine.BlockCount())
	}
}

func TestStore_IncrementsRefCount(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("ref test")

	for i := 0; i < 5; i++ {
		hash, err := engine.Store(data)
		if err != nil {
			t.Fatalf("第 %d 次 Store 失败: %v", i+1, err)
		}
		block := engine.blocks[hash]
		if block.RefCount != i+1 {
			t.Errorf("第 %d 次存储后 RefCount = %d, want %d", i+1, block.RefCount, i+1)
		}
	}
}

func TestStore_DifferentData(t *testing.T) {
	engine := NewEngine(nil)
	hash1, _ := engine.Store([]byte("data1"))
	hash2, _ := engine.Store([]byte("data2"))
	if hash1 == hash2 {
		t.Error("不同数据产生相同哈希")
	}
	if engine.BlockCount() != 2 {
		t.Errorf("唯一块数 = %d, want 2", engine.BlockCount())
	}
}

func TestStore_LargeData(t *testing.T) {
	engine := NewEngine(nil)
	data := makeData(1024 * 1024) // 1MB
	hash, err := engine.Store(data)
	if err != nil {
		t.Fatalf("存储大数据失败: %v", err)
	}
	if hash == "" {
		t.Error("大数据哈希为空")
	}
}

func TestStore_SetsCreatedAt(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("timestamp test")
	hash, _ := engine.Store(data)
	block := engine.blocks[hash]
	if block.CreatedAt.IsZero() {
		t.Error("CreatedAt 未设置")
	}
}

func TestStore_SetsSize(t *testing.T) {
	engine := NewEngine(nil)
	data := makeData(1234)
	hash, _ := engine.Store(data)
	block := engine.blocks[hash]
	if block.Size != 1234 {
		t.Errorf("Size = %d, want 1234", block.Size)
	}
}

// ========== Retrieve 测试 ==========

func TestRetrieve_ExistingBlock(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("retrieve test")
	hash, _ := engine.Store(data)

	retrieved, err := engine.Retrieve(hash)
	if err != nil {
		t.Fatalf("Retrieve 失败: %v", err)
	}
	if !bytes.Equal(retrieved, data) {
		t.Errorf("检索数据不匹配: got %s, want %s", retrieved, data)
	}
}

func TestRetrieve_NonExistentBlock(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Retrieve("nonexistent_hash")
	if err != ErrBlockNotFound {
		t.Errorf("Retrieve 不存在块错误 = %v, want %v", err, ErrBlockNotFound)
	}
}

func TestRetrieve_ReturnsCopy(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("original data")
	hash, _ := engine.Store(data)

	retrieved1, _ := engine.Retrieve(hash)
	// 修改检索的数据
	retrieved1[0] = 'X'

	// 再次检索，应该是原始数据
	retrieved2, _ := engine.Retrieve(hash)
	if !bytes.Equal(retrieved2, data) {
		t.Error("返回的数据不是副本，原始数据被修改")
	}
}

func TestRetrieve_EmptyHash(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Retrieve("")
	if err != ErrBlockNotFound {
		t.Errorf("空哈希错误 = %v, want %v", err, ErrBlockNotFound)
	}
}

// ========== Delete 测试 ==========

func TestDelete_DecrementsRefCount(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("delete test")
	hash, _ := engine.Store(data)
	engine.Store(data) // 增加到 2

	block := engine.blocks[hash]
	if block.RefCount != 2 {
		t.Fatalf("RefCount = %d, want 2", block.RefCount)
	}

	err := engine.Delete(hash)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if block.RefCount != 1 {
		t.Errorf("Delete 后 RefCount = %d, want 1", block.RefCount)
	}
}

func TestDelete_RemovesBlockAtZero(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("zero ref")
	hash, _ := engine.Store(data)

	err := engine.Delete(hash)
	if err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if engine.HasBlock(hash) {
		t.Error("RefCount=0 后块仍存在")
	}
}

func TestDelete_NonExistentBlock(t *testing.T) {
	engine := NewEngine(nil)
	err := engine.Delete("nonexistent")
	if err != ErrBlockNotFound {
		t.Errorf("Delete 不存在块错误 = %v, want %v", err, ErrBlockNotFound)
	}
}

func TestDelete_MultipleRefs(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("multi ref")
	hash, _ := engine.Store(data)
	engine.Store(data)
	engine.Store(data) // RefCount = 3

	// 删除 2 次，块仍存在
	engine.Delete(hash)
	engine.Delete(hash)
	if !engine.HasBlock(hash) {
		t.Error("块不应被删除")
	}

	// 第 3 次删除，块应消失
	engine.Delete(hash)
	if engine.HasBlock(hash) {
		t.Error("块应被删除")
	}
}

func TestDelete_EmptyHash(t *testing.T) {
	engine := NewEngine(nil)
	err := engine.Delete("")
	if err != ErrBlockNotFound {
		t.Errorf("空哈希错误 = %v, want %v", err, ErrBlockNotFound)
	}
}

// ========== Scan 测试 ==========

func TestScan_EmptyData(t *testing.T) {
	engine := NewEngine(nil)
	blocks := engine.Scan([]byte{})
	if blocks != nil {
		t.Errorf("Scan(empty) 返回 %d 块, want nil", len(blocks))
	}
}

func TestScan_NilData(t *testing.T) {
	engine := NewEngine(nil)
	blocks := engine.Scan(nil)
	if blocks != nil {
		t.Errorf("Scan(nil) 返回 %d 块, want nil", len(blocks))
	}
}

func TestScan_SmallData(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)
	data := []byte("small") // 远小于 4KB
	blocks := engine.Scan(data)

	if len(blocks) != 1 {
		t.Fatalf("Scan 返回 %d 块, want 1", len(blocks))
	}
	if blocks[0].Size != len(data) {
		t.Errorf("块大小 = %d, want %d", blocks[0].Size, len(data))
	}
	if !bytes.Equal(blocks[0].Data, data) {
		t.Error("块数据不匹配")
	}
}

func TestScan_ExactBlockSize(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)
	data := makeData(int(BlockSize4K))
	blocks := engine.Scan(data)

	if len(blocks) != 1 {
		t.Errorf("Scan 返回 %d 块, want 1", len(blocks))
	}
}

func TestScan_MultipleBlocks(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)
	// 3 个块大小的数据
	data := makeData(int(BlockSize4K) * 3)
	blocks := engine.Scan(data)

	if len(blocks) != 3 {
		t.Errorf("Scan 返回 %d 块, want 3", len(blocks))
	}

	// 验证每个块
	offset := 0
	for i, block := range blocks {
		expectedSize := int(BlockSize4K)
		if i == len(blocks)-1 {
			// 最后一块可能更小
			expectedSize = len(data) - offset
		}
		if block.Size != expectedSize {
			t.Errorf("块 %d 大小 = %d, want %d", i, block.Size, expectedSize)
		}
		if !bytes.Equal(block.Data, data[offset:offset+expectedSize]) {
			t.Errorf("块 %d 数据不匹配", i)
		}
		offset += expectedSize
	}
}

func TestScan_NonAlignedData(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)
	// 1.5 个块大小
	data := makeData(int(BlockSize4K) + int(BlockSize4K)/2)
	blocks := engine.Scan(data)

	if len(blocks) != 2 {
		t.Errorf("Scan 返回 %d 块, want 2", len(blocks))
	}
	// 第一块应该是完整块大小
	if blocks[0].Size != int(BlockSize4K) {
		t.Errorf("第一块大小 = %d, want %d", blocks[0].Size, BlockSize4K)
	}
	// 第二块应该是剩余部分
	expectedSecondSize := int(BlockSize4K) / 2
	if blocks[1].Size != expectedSecondSize {
		t.Errorf("第二块大小 = %d, want %d", blocks[1].Size, expectedSecondSize)
	}
}

func TestScan_DuplicateChunks(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)
	// 创建重复的块
	chunk := makeData(int(BlockSize4K))
	data := append(chunk, chunk...)
	blocks := engine.Scan(data)

	if len(blocks) != 2 {
		t.Fatalf("Scan 返回 %d 块, want 2", len(blocks))
	}
	// 两个块的哈希应该相同
	if blocks[0].Hash != blocks[1].Hash {
		t.Error("相同内容的块哈希不同")
	}
}

func TestScan_SetsRefCountToZero(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("scan ref test")
	blocks := engine.Scan(data)
	for i, block := range blocks {
		if block.RefCount != 0 {
			t.Errorf("块 %d RefCount = %d, want 0", i, block.RefCount)
		}
	}
}

func TestScan_DifferentBlockSizes(t *testing.T) {
	sizes := []BlockSize{BlockSize4K, BlockSize64K, BlockSize1M}
	for _, size := range sizes {
		t.Run(size.String(), func(t *testing.T) {
			config := &EngineConfig{BlockSize: size}
			engine := NewEngine(config)
			data := makeData(int(size)*2 + 100)
			blocks := engine.Scan(data)
			// 验证块大小正确
			for i, block := range blocks {
				expectedSize := int(size)
				if i == len(blocks)-1 {
					expectedSize = len(data) - i*int(size)
				}
				if block.Size != expectedSize {
					t.Errorf("块 %d 大小 = %d, want %d", i, block.Size, expectedSize)
				}
			}
		})
	}
}

// ========== Stats 测试 ==========

func TestStats_EmptyEngine(t *testing.T) {
	engine := NewEngine(nil)
	stats := engine.Stats()
	if stats.UniqueBlocks != 0 {
		t.Errorf("UniqueBlocks = %d, want 0", stats.UniqueBlocks)
	}
	if stats.TotalBlocks != 0 {
		t.Errorf("TotalBlocks = %d, want 0", stats.TotalBlocks)
	}
	if stats.UniqueSize != 0 {
		t.Errorf("UniqueSize = %d, want 0", stats.UniqueSize)
	}
}

func TestStats_WithBlocks(t *testing.T) {
	engine := NewEngine(nil)
	engine.Store([]byte("block1"))
	engine.Store([]byte("block2"))
	engine.Store([]byte("block1")) // 重复

	stats := engine.Stats()
	if stats.UniqueBlocks != 2 {
		t.Errorf("UniqueBlocks = %d, want 2", stats.UniqueBlocks)
	}
	if stats.TotalBlocks != 3 {
		t.Errorf("TotalBlocks = %d, want 3", stats.TotalBlocks)
	}
	if stats.DuplicateBlocks != 1 {
		t.Errorf("DuplicateBlocks = %d, want 1", stats.DuplicateBlocks)
	}
}

func TestStats_SpaceSaved(t *testing.T) {
	engine := NewEngine(nil)
	data := makeData(1024)
	engine.Store(data)
	engine.Store(data) // 重复

	stats := engine.Stats()
	if stats.SpaceSaved <= 0 {
		t.Errorf("SpaceSaved = %d, want > 0", stats.SpaceSaved)
	}
}

func TestStats_DedupRatio(t *testing.T) {
	engine := NewEngine(nil)
	engine.Store([]byte("a"))
	engine.Store([]byte("b"))
	engine.Store([]byte("a")) // 重复
	engine.Store([]byte("a")) // 重复

	stats := engine.Stats()
	ratio := stats.DedupRatio()
	// 总块数 = 4，重复块 = 2
	expectedRatio := 2.0 / 4.0
	if ratio != expectedRatio {
		t.Errorf("DedupRatio() = %f, want %f", ratio, expectedRatio)
	}
}

func TestStats_DedupRatio_Empty(t *testing.T) {
	engine := NewEngine(nil)
	stats := engine.Stats()
	if stats.DedupRatio() != 0 {
		t.Errorf("空引擎 DedupRatio() = %f, want 0", stats.DedupRatio())
	}
}

func TestStats_SpaceSavingsPercent(t *testing.T) {
	engine := NewEngine(nil)
	engine.Store([]byte("x"))
	engine.Store([]byte("x"))

	stats := engine.Stats()
	pct := stats.SpaceSavingsPercent()
	if pct != 50.0 {
		t.Errorf("SpaceSavingsPercent() = %f, want 50.0", pct)
	}
}

func TestStats_SpaceSavingsPercent_Empty(t *testing.T) {
	engine := NewEngine(nil)
	stats := engine.Stats()
	if stats.SpaceSavingsPercent() != 0 {
		t.Errorf("空引擎 SpaceSavingsPercent() = %f, want 0", stats.SpaceSavingsPercent())
	}
}

// ========== BlockCount 测试 ==========

func TestBlockCount_Empty(t *testing.T) {
	engine := NewEngine(nil)
	if engine.BlockCount() != 0 {
		t.Errorf("BlockCount() = %d, want 0", engine.BlockCount())
	}
}

func TestBlockCount_AfterStore(t *testing.T) {
	engine := NewEngine(nil)
	engine.Store([]byte("a"))
	engine.Store([]byte("b"))
	engine.Store([]byte("a")) // 重复

	if engine.BlockCount() != 2 {
		t.Errorf("BlockCount() = %d, want 2", engine.BlockCount())
	}
}

// ========== HasBlock 测试 ==========

func TestHasBlock_Exists(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("exists")
	hash, _ := engine.Store(data)

	if !engine.HasBlock(hash) {
		t.Error("HasBlock 返回 false，块应存在")
	}
}

func TestHasBlock_NotExists(t *testing.T) {
	engine := NewEngine(nil)
	if engine.HasBlock("nonexistent") {
		t.Error("HasBlock 返回 true，块不应存在")
	}
}

func TestHasBlock_AfterDelete(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("will delete")
	hash, _ := engine.Store(data)
	engine.Delete(hash)

	if engine.HasBlock(hash) {
		t.Error("HasBlock 返回 true，块已删除")
	}
}

// ========== Clear 测试 ==========

func TestClear(t *testing.T) {
	engine := NewEngine(nil)
	engine.Store([]byte("a"))
	engine.Store([]byte("b"))
	engine.Store([]byte("c"))

	engine.Clear()
	if engine.BlockCount() != 0 {
		t.Errorf("Clear 后 BlockCount() = %d, want 0", engine.BlockCount())
	}
}

func TestClear_Idempotent(t *testing.T) {
	engine := NewEngine(nil)
	engine.Clear()
	engine.Clear()
	if engine.BlockCount() != 0 {
		t.Error("多次 Clear 应保持为空")
	}
}

// ========== 并发测试 ==========

func TestConcurrentStore(t *testing.T) {
	engine := NewEngine(nil)
	var wg sync.WaitGroup
	goroutines := 100

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("concurrent-%d", n))
			engine.Store(data)
		}(i)
	}
	wg.Wait()

	if engine.BlockCount() != goroutines {
		t.Errorf("并发存储后块数 = %d, want %d", engine.BlockCount(), goroutines)
	}
}

func TestConcurrentStoreDuplicate(t *testing.T) {
	engine := NewEngine(nil)
	var wg sync.WaitGroup
	goroutines := 100
	data := []byte("same data")

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			engine.Store(data)
		}()
	}
	wg.Wait()

	if engine.BlockCount() != 1 {
		t.Errorf("并发重复存储后块数 = %d, want 1", engine.BlockCount())
	}

	hash := hashOf(data)
	block := engine.blocks[hash]
	if block.RefCount != goroutines {
		t.Errorf("RefCount = %d, want %d", block.RefCount, goroutines)
	}
}

func TestConcurrentRetrieve(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("concurrent retrieve")
	hash, _ := engine.Store(data)

	var wg sync.WaitGroup
	goroutines := 50

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			retrieved, err := engine.Retrieve(hash)
			if err != nil {
				t.Errorf("Retrieve 失败: %v", err)
				return
			}
			if !bytes.Equal(retrieved, data) {
				t.Error("检索数据不匹配")
			}
		}()
	}
	wg.Wait()
}

func TestConcurrentDelete(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("concurrent delete")
	hash, _ := engine.Store(data)
	// 增加引用计数
	for i := 0; i < 50; i++ {
		engine.Store(data)
	}

	var wg sync.WaitGroup
	wg.Add(50)
	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			engine.Delete(hash)
		}()
	}
	wg.Wait()

	// 应该还有 1 个引用
	block, exists := engine.blocks[hash]
	if !exists {
		t.Fatal("块不应被完全删除")
	}
	if block.RefCount != 1 {
		t.Errorf("RefCount = %d, want 1", block.RefCount)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	engine := NewEngine(nil)
	var wg sync.WaitGroup
	goroutines := 50

	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			data := []byte(fmt.Sprintf("mixed-%d", n%10))
			engine.Store(data)
		}(i)
		go func(n int) {
			defer wg.Done()
			engine.Stats()
		}(i)
		go func(n int) {
			defer wg.Done()
			engine.BlockCount()
		}(i)
	}
	wg.Wait()

	// 不应 panic
	if engine.BlockCount() < 1 {
		t.Error("混合操作后块数异常")
	}
}

// ========== 边界条件 ==========

func TestStore_SingleByte(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte{0x42}
	hash, err := engine.Store(data)
	if err != nil {
		t.Fatalf("存储单字节失败: %v", err)
	}
	retrieved, _ := engine.Retrieve(hash)
	if len(retrieved) != 1 || retrieved[0] != 0x42 {
		t.Error("单字节数据不匹配")
	}
}

func TestStore_AllZeros(t *testing.T) {
	engine := NewEngine(nil)
	data := make([]byte, 1024) // 全零
	hash, err := engine.Store(data)
	if err != nil {
		t.Fatalf("存储全零数据失败: %v", err)
	}
	retrieved, _ := engine.Retrieve(hash)
	if !bytes.Equal(retrieved, data) {
		t.Error("全零数据不匹配")
	}
}

func TestStore_MaxByteValues(t *testing.T) {
	engine := NewEngine(nil)
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	hash, err := engine.Store(data)
	if err != nil {
		t.Fatalf("存储失败: %v", err)
	}
	retrieved, _ := engine.Retrieve(hash)
	if !bytes.Equal(retrieved, data) {
		t.Error("数据不匹配")
	}
}

// ========== 集成测试 ==========

func TestIntegration_StoreRetrieveDelete(t *testing.T) {
	engine := NewEngine(nil)

	// 存储多个块
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		data := []byte(fmt.Sprintf("integration-test-%d", i))
		hash, err := engine.Store(data)
		if err != nil {
			t.Fatalf("存储块 %d 失败: %v", i, err)
		}
		hashes[i] = hash
	}

	// 验证所有块
	if engine.BlockCount() != 10 {
		t.Errorf("块数 = %d, want 10", engine.BlockCount())
	}

	// 检索并验证
	for i, hash := range hashes {
		data, err := engine.Retrieve(hash)
		if err != nil {
			t.Fatalf("检索块 %d 失败: %v", i, err)
		}
		expected := []byte(fmt.Sprintf("integration-test-%d", i))
		if !bytes.Equal(data, expected) {
			t.Errorf("块 %d 数据不匹配", i)
		}
	}

	// 删除一半块
	for i := 0; i < 5; i++ {
		err := engine.Delete(hashes[i])
		if err != nil {
			t.Fatalf("删除块 %d 失败: %v", i, err)
		}
	}

	if engine.BlockCount() != 5 {
		t.Errorf("删除后块数 = %d, want 5", engine.BlockCount())
	}

	// 清空
	engine.Clear()
	if engine.BlockCount() != 0 {
		t.Errorf("清空后块数 = %d, want 0", engine.BlockCount())
	}
}

func TestIntegration_Deduplication(t *testing.T) {
	engine := NewEngine(nil)

	// 存储相同数据多次
	data := makeData(4096)
	for i := 0; i < 10; i++ {
		engine.Store(data)
	}

	stats := engine.Stats()
	if stats.UniqueBlocks != 1 {
		t.Errorf("UniqueBlocks = %d, want 1", stats.UniqueBlocks)
	}
	if stats.TotalBlocks != 10 {
		t.Errorf("TotalBlocks = %d, want 10", stats.TotalBlocks)
	}
	if stats.DuplicateBlocks != 9 {
		t.Errorf("DuplicateBlocks = %d, want 9", stats.DuplicateBlocks)
	}
}

func TestIntegration_ScanAndStore(t *testing.T) {
	config := &EngineConfig{BlockSize: BlockSize4K}
	engine := NewEngine(config)

	// 创建不重复的数据（每个字节位置使用不同的值）
	data := make([]byte, int(BlockSize4K)*2+1024)
	for i := range data {
		data[i] = byte((i * 7) % 251) // 251 是素数，避免重复模式
	}
	blocks := engine.Scan(data)

	if len(blocks) != 3 {
		t.Fatalf("Scan 返回 %d 块, want 3", len(blocks))
	}

	// 验证每个块的哈希不同
	if blocks[0].Hash == blocks[1].Hash || blocks[1].Hash == blocks[2].Hash || blocks[0].Hash == blocks[2].Hash {
		t.Error("扫描出的块哈希不应相同")
	}

	// 存储扫描出的块
	for _, block := range blocks {
		engine.Store(block.Data)
	}

	if engine.BlockCount() != 3 {
		t.Errorf("存储后块数 = %d, want 3", engine.BlockCount())
	}

	// 再次存储重复块
	for _, block := range blocks {
		engine.Store(block.Data)
	}

	// 块数不应改变
	if engine.BlockCount() != 3 {
		t.Errorf("重复存储后块数 = %d, want 3", engine.BlockCount())
	}
}

// ========== 错误常量测试 ==========

func TestErrorConstants(t *testing.T) {
	if ErrBlockNotFound == nil {
		t.Error("ErrBlockNotFound 为 nil")
	}
	if ErrInvalidData == nil {
		t.Error("ErrInvalidData 为 nil")
	}
	if ErrBlockInUse == nil {
		t.Error("ErrBlockInUse 为 nil")
	}
	if ErrHashMismatch == nil {
		t.Error("ErrHashMismatch 为 nil")
	}
}

// ========== ContentBlock 字段测试 ==========

func TestContentBlock_Fields(t *testing.T) {
	engine := NewEngine(nil)
	data := []byte("field test")
	hash, _ := engine.Store(data)
	block := engine.blocks[hash]

	if block.Hash != hash {
		t.Errorf("Hash 字段不匹配")
	}
	if !bytes.Equal(block.Data, data) {
		t.Error("Data 字段不匹配")
	}
	if block.Size != len(data) {
		t.Errorf("Size = %d, want %d", block.Size, len(data))
	}
	if block.RefCount != 1 {
		t.Errorf("RefCount = %d, want 1", block.RefCount)
	}
}

// ========== StoreResult 测试 ==========

func TestStoreResult_Fields(t *testing.T) {
	// StoreResult 结构体存在但未被使用，验证结构体定义
	result := StoreResult{
		Hash:     "test_hash",
		IsNew:    true,
		RefCount: 1,
		Size:     1024,
	}
	if result.Hash != "test_hash" {
		t.Error("Hash 字段错误")
	}
	if !result.IsNew {
		t.Error("IsNew 字段错误")
	}
	if result.RefCount != 1 {
		t.Error("RefCount 字段错误")
	}
	if result.Size != 1024 {
		t.Error("Size 字段错误")
	}
}

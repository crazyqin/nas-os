package dedup

import (
	"context"
	"testing"
)

func TestChunkHash(t *testing.T) {
	// Test ChunkHash creation
	var hash ChunkHash
	for i := range hash {
		hash[i] = byte(i)
	}

	// Test String method
	str := hash.String()
	if len(str) != 64 {
		t.Errorf("Hash string length should be 64, got %d", len(str))
	}
}

func TestDDTBasic(t *testing.T) {
	config := DefaultDedupConfig()

	ddt, err := NewDDT(config)
	if err != nil {
		t.Fatalf("Failed to create DDT: %v", err)
	}
	defer ddt.Close()

	// Test ComputeChunkHash
	data := []byte("test chunk data")
	hash := ddt.ComputeChunkHash(data)

	// Test Insert
	entry, existed, err := ddt.Insert(hash, uint32(len(data)))
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}
	if existed {
		t.Error("First insert should not mark as existed")
	}
	if entry.RefCount != 1 {
		t.Errorf("RefCount should be 1, got %d", entry.RefCount)
	}

	// Test Lookup
	found, err := ddt.Lookup(hash)
	if err != nil {
		t.Fatalf("Failed to lookup: %v", err)
	}
	if found.Hash != hash {
		t.Error("Hash mismatch")
	}

	// Test duplicate insert (should increase ref count)
	entry2, existed, err := ddt.Insert(hash, uint32(len(data)))
	if err != nil {
		t.Fatalf("Failed to insert duplicate: %v", err)
	}
	if !existed {
		t.Error("Duplicate insert should mark as existed")
	}
	if entry2.RefCount != 2 {
		t.Errorf("RefCount should be 2, got %d", entry2.RefCount)
	}

	// Test DecrementRef
	if err := ddt.DecrementRef(hash); err != nil {
		t.Fatalf("Failed to decrement ref: %v", err)
	}
	found, _ = ddt.Lookup(hash)
	if found.RefCount != 1 {
		t.Errorf("RefCount should be 1 after decrement, got %d", found.RefCount)
	}

	// Test Remove
	if err := ddt.Remove(hash); err != nil {
		t.Fatalf("Failed to remove: %v", err)
	}
	_, err = ddt.Lookup(hash)
	if err == nil {
		t.Error("Lookup should fail after remove")
	}
}

func TestDDTStats(t *testing.T) {
	config := DefaultDedupConfig()
	ddt, _ := NewDDT(config)
	defer ddt.Close()

	// Insert some entries
	for i := 0; i < 10; i++ {
		data := []byte{byte(i)}
		hash := ddt.ComputeChunkHash(data)
		ddt.Insert(hash, 1)
	}

	stats := ddt.GetStats()
	if stats.TotalEntries != 10 {
		t.Errorf("Expected 10 entries, got %d", stats.TotalEntries)
	}
	if stats.UniqueChunks != 10 {
		t.Errorf("Expected 10 unique chunks, got %d", stats.UniqueChunks)
	}
}

func TestFastPathCache(t *testing.T) {
	cache := NewFastPathCache(3)

	entry1 := &DDTEntry{Hash: ChunkHash{1}}
	entry2 := &DDTEntry{Hash: ChunkHash{2}}
	entry3 := &DDTEntry{Hash: ChunkHash{3}}
	entry4 := &DDTEntry{Hash: ChunkHash{4}}

	// Add entries
	cache.Set(entry1)
	cache.Set(entry2)
	cache.Set(entry3)

	// Check retrieval
	if found, ok := cache.Get(ChunkHash{1}); !ok || found != entry1 {
		t.Error("Should find entry1")
	}

	// Add fourth entry (should evict first)
	cache.Set(entry4)

	// entry1 should be evicted
	if _, ok := cache.Get(ChunkHash{1}); ok {
		t.Error("entry1 should be evicted")
	}

	// Check size
	if len(cache.entries) != 3 {
		t.Errorf("Cache size should be 3, got %d", len(cache.entries))
	}
}

func TestFastDeduplicator(t *testing.T) {
	config := DefaultDedupConfig()
	dedup, err := NewFastDeduplicator(config)
	if err != nil {
		t.Fatalf("Failed to create deduplicator: %v", err)
	}
	defer dedup.Close()

	ctx := context.Background()

	// Test write
	data := []byte("test data")
	entry, deduped, err := dedup.DedupWrite(ctx, data)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if deduped {
		t.Error("First write should not be deduped")
	}

	// Test duplicate write
	entry2, deduped, err := dedup.DedupWrite(ctx, data)
	if err != nil {
		t.Fatalf("Failed to write duplicate: %v", err)
	}
	if !deduped {
		t.Error("Duplicate write should be deduped")
	}
	if entry.Hash != entry2.Hash {
		t.Error("Same data should have same hash")
	}

	// Test stats
	stats := dedup.GetStats()
	if stats["dedupedChunks"].(uint64) != 1 {
		t.Error("Should have 1 deduped chunk")
	}
}

func TestChunkData(t *testing.T) {
	data := []byte("0123456789") // 10 bytes
	chunks := ChunkData(data, 3)

	if len(chunks) != 4 {
		t.Errorf("Expected 4 chunks, got %d", len(chunks))
	}

	// Check first chunk
	if string(chunks[0]) != "012" {
		t.Errorf("First chunk should be '012', got '%s'", chunks[0])
	}

	// Check last chunk
	if string(chunks[3]) != "9" {
		t.Errorf("Last chunk should be '9', got '%s'", chunks[3])
	}
}

func TestWriteBuffer(t *testing.T) {
	wb := NewWriteBuffer(10) // 10 bytes max

	// Test add
	if !wb.Add("hash1", []byte("12345")) {
		t.Error("Should add data")
	}

	// Test get
	data, ok := wb.Get("hash1")
	if !ok || string(data) != "12345" {
		t.Error("Should get correct data")
	}

	// Test overflow
	if wb.Add("hash2", []byte("123456")) {
		t.Error("Should not add data over limit")
	}

	// Test remove
	wb.Remove("hash1")
	if _, ok := wb.Get("hash1"); ok {
		t.Error("Should not find removed data")
	}
}

func TestDedupConfigValidation(t *testing.T) {
	// Test valid chunk sizes
	validSizes := []uint32{4096, 8192, 16384, 32768, 65536, 131072}
	for _, size := range validSizes {
		config := &DedupConfig{ChunkSize: size}
		if err := config.Validate(); err != nil {
			t.Errorf("Valid chunk size %d should pass: %v", size, err)
		}
	}

	// Test invalid hash algorithm defaults to sha256
	config := &DedupConfig{
		ChunkSize:     32768,
		HashAlgorithm: "invalid",
	}
	config.Validate()
	if config.HashAlgorithm != "sha256" {
		t.Errorf("Should default to sha256, got %s", config.HashAlgorithm)
	}

	// Test invalid verify level defaults to onWrite
	config = &DedupConfig{
		ChunkSize:   32768,
		VerifyLevel: "invalid",
	}
	config.Validate()
	if config.VerifyLevel != "onWrite" {
		t.Errorf("Should default to onWrite, got %s", config.VerifyLevel)
	}

	// Test empty config uses valid defaults
	config = DefaultDedupConfig()
	if err := config.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func BenchmarkComputeChunkHash(b *testing.B) {
	config := DefaultDedupConfig()
	ddt, _ := NewDDT(config)
	defer ddt.Close()

	data := make([]byte, 32768) // 32KB
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ddt.ComputeChunkHash(data)
	}
}

func BenchmarkDDTInsert(b *testing.B) {
	config := DefaultDedupConfig()
	ddt, _ := NewDDT(config)
	defer ddt.Close()

	data := make([]byte, 32)
	for i := 0; i < b.N; i++ {
		data[0] = byte(i)
		hash := ddt.ComputeChunkHash(data)
		ddt.Insert(hash, 32)
	}
}

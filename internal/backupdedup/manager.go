package backupdedup

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// DeduplicationStats represents deduplication statistics
type DeduplicationStats struct {
	TotalFiles      int64   `json:"total_files"`
	UniqueFiles     int64   `json:"unique_files"`
	DuplicateFiles  int64   `json:"duplicate_files"`
	TotalBytes      int64   `json:"total_bytes"`
	UniqueBytes     int64   `json:"unique_bytes"`
	SavedBytes      int64   `json:"saved_bytes"`
	DedupRatio      float64 `json:"dedup_ratio"`
	SpaceSavedPct   float64 `json:"space_saved_percent"`
	LastRunTime     time.Time `json:"last_run_time"`
	RunDuration     time.Duration `json:"run_duration"`
}

// ChunkInfo represents information about a deduplicated chunk
type ChunkInfo struct {
	Hash       string    `json:"hash"`
	Size       int64     `json:"size"`
	RefCount   int       `json:"ref_count"`
	FirstSeen  time.Time `json:"first_seen"`
	LastAccess time.Time `json:"last_access"`
	IsCompressed bool    `json:"is_compressed"`
	CompressedSize int64 `json:"compressed_size,omitempty"`
}

// FileDedupInfo represents dedup info for a file
type FileDedupInfo struct {
	FilePath    string    `json:"file_path"`
	FileHash    string    `json:"file_hash"`
	FileSize    int64     `json:"file_size"`
	ChunkHashes []string  `json:"chunk_hashes"`
	IsDuplicate bool      `json:"is_duplicate"`
	RefCount    int       `json:"ref_count"`
	StoragePath string    `json:"storage_path"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DedupConfig represents deduplication configuration
type DedupConfig struct {
	Enabled          bool `json:"enabled"`
	ChunkSize        int  `json:"chunk_size_kb"`
	MinFileSize      int  `json:"min_file_size_bytes"`
	MaxFileSize      int  `json:"max_file_size_mb"`
	EnableCompression bool `json:"enable_compression"`
	CompressionAlgo  string `json:"compression_algorithm"`
	VerifyIntegrity  bool `json:"verify_integrity"`
	RetainOriginals  bool `json:"retain_originals_days"`
	RetainDays       int  `json:"retain_days"`
	AutoDedup        bool `json:"auto_dedup"`
	ScanInterval     int  `json:"scan_interval_hours"`
}

// DedupJob represents a deduplication job
type DedupJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Path      string    `json:"path"`
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Stats     DeduplicationStats `json:"stats"`
	ErrorMsg  string    `json:"error_msg,omitempty"`
}

// Manager manages backup deduplication
type Manager struct {
	mu      sync.RWMutex
	chunks  map[string]*ChunkInfo
	files   map[string]*FileDedupInfo
	jobs    map[string]*DedupJob
	config  DedupConfig
	stats   DeduplicationStats
}

// NewManager creates a new dedup manager
func NewManager(config DedupConfig) *Manager {
	return &Manager{
		chunks: make(map[string]*ChunkInfo),
		files:  make(map[string]*FileDedupInfo),
		jobs:   make(map[string]*DedupJob),
		config: config,
	}
}

// ProcessFile processes a file for deduplication
func (m *Manager) ProcessFile(filePath string, fileSize int64, data []byte) (*FileDedupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("deduplication is disabled")
	}

	if fileSize < int64(m.config.MinFileSize) {
		return nil, fmt.Errorf("file too small for deduplication: %d < %d", fileSize, m.config.MinFileSize)
	}

	if m.config.MaxFileSize > 0 && fileSize > int64(m.config.MaxFileSize)*1024*1024 {
		return nil, fmt.Errorf("file too large for deduplication: %d > %d MB", fileSize, m.config.MaxFileSize)
	}

	// Calculate file hash
	hash := sha256.Sum256(data)
	fileHash := fmt.Sprintf("%x", hash)

	// Check if file already exists
	if existing, ok := m.files[filePath]; ok {
		if existing.FileHash == fileHash {
			existing.RefCount++
			existing.UpdatedAt = time.Now()
			return existing, nil
		}
	}

	// Check for duplicate
	if existing, ok := m.findDuplicate(fileHash); ok {
		existing.RefCount++
		m.stats.DuplicateFiles++
		m.stats.SavedBytes += fileSize

		info := &FileDedupInfo{
			FilePath:    filePath,
			FileHash:    fileHash,
			FileSize:    fileSize,
			IsDuplicate: true,
			RefCount:    1,
			StoragePath: existing.StoragePath,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		m.files[filePath] = info
		m.updateStats()

		return info, nil
	}

	// New unique file
	chunkSize := m.config.ChunkSize * 1024
	chunks := m.splitIntoChunks(data, chunkSize)
	chunkHashes := make([]string, 0, len(chunks))

	for _, chunk := range chunks {
		chunkHash := sha256.Sum256(chunk)
		chunkHashStr := fmt.Sprintf("%x", chunkHash)

		if _, ok := m.chunks[chunkHashStr]; !ok {
			m.chunks[chunkHashStr] = &ChunkInfo{
				Hash:      chunkHashStr,
				Size:      int64(len(chunk)),
				RefCount:  1,
				FirstSeen: time.Now(),
				LastAccess: time.Now(),
			}
		} else {
			m.chunks[chunkHashStr].RefCount++
			m.chunks[chunkHashStr].LastAccess = time.Now()
		}

		chunkHashes = append(chunkHashes, chunkHashStr)
	}

	info := &FileDedupInfo{
		FilePath:    filePath,
		FileHash:    fileHash,
		FileSize:    fileSize,
		ChunkHashes: chunkHashes,
		IsDuplicate: false,
		RefCount:    1,
		StoragePath: fmt.Sprintf("/dedup/%s", fileHash[:2]),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.files[filePath] = info
	m.stats.TotalFiles++
	m.stats.UniqueFiles++
	m.stats.TotalBytes += fileSize
	m.stats.UniqueBytes += fileSize

	return info, nil
}

// findDuplicate finds a file with the same hash
func (m *Manager) findDuplicate(hash string) (*FileDedupInfo, bool) {
	for _, file := range m.files {
		if file.FileHash == hash && !file.IsDuplicate {
			return file, true
		}
	}
	return nil, false
}

// splitIntoChunks splits data into chunks
func (m *Manager) splitIntoChunks(data []byte, chunkSize int) [][]byte {
	chunks := make([][]byte, 0)
	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}
		chunks = append(chunks, data[i:end])
	}
	return chunks
}

// updateStats updates dedup statistics
func (m *Manager) updateStats() {
	if m.stats.TotalBytes > 0 {
		m.stats.DedupRatio = float64(m.stats.TotalBytes) / float64(m.stats.UniqueBytes)
		m.stats.SpaceSavedPct = float64(m.stats.SavedBytes) / float64(m.stats.TotalBytes) * 100
	}
}

// RunDedupJob runs a deduplication job on a path
func (m *Manager) RunDedupJob(path string) (*DedupJob, error) {
	m.mu.Lock()

	job := &DedupJob{
		ID:        fmt.Sprintf("dedup-%d", time.Now().UnixNano()),
		Status:    "running",
		Path:      path,
		StartTime: time.Now(),
	}

	m.jobs[job.ID] = job
	m.mu.Unlock()

	// Simulate dedup job
	m.mu.Lock()
	now := time.Now()
	job.EndTime = &now
	job.Status = "completed"
	job.Stats = m.stats
	job.Stats.LastRunTime = now
	job.Stats.RunDuration = now.Sub(job.StartTime)
	m.mu.Unlock()

	return job, nil
}

// GetStats returns deduplication statistics
func (m *Manager) GetStats() DeduplicationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// GetChunk returns chunk info by hash
func (m *Manager) GetChunk(hash string) (*ChunkInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunk, ok := m.chunks[hash]
	if !ok {
		return nil, fmt.Errorf("chunk not found: %s", hash)
	}

	return chunk, nil
}

// ListChunks lists all chunks
func (m *Manager) ListChunks() []*ChunkInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chunks := make([]*ChunkInfo, 0, len(m.chunks))
	for _, c := range m.chunks {
		chunks = append(chunks, c)
	}

	return chunks
}

// GetFile returns file dedup info
func (m *Manager) GetFile(path string) (*FileDedupInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	file, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	return file, nil
}

// ListFiles lists all deduped files
func (m *Manager) ListFiles() []*FileDedupInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	files := make([]*FileDedupInfo, 0, len(m.files))
	for _, f := range m.files {
		files = append(files, f)
	}

	return files
}

// GetJob returns a dedup job by ID
func (m *Manager) GetJob(id string) (*DedupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", id)
	}

	return job, nil
}

// ListJobs lists all dedup jobs
func (m *Manager) ListJobs() []*DedupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*DedupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}

	return jobs
}

// UpdateConfig updates dedup configuration
func (m *Manager) UpdateConfig(config DedupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() DedupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// Package ai provides AI service integration for NAS-OS
// model_manager.go - Model download, switching, and resource monitoring
package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"nas-os/pkg/config"
)

// ModelManager handles model lifecycle management
type ModelManager struct {
	config      *config.ModelManagerConfig
	storagePath string
	registry    *ModelRegistry
	downloads   map[string]*DownloadProgress
	mu          sync.RWMutex
}

// NewModelManager creates a new model manager
func NewModelManager(cfg *config.ModelManagerConfig) (*ModelManager, error) {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "/var/lib/nas-os/ai/models"
	}

	// Ensure storage path exists
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage path: %w", err)
	}

	mgr := &ModelManager{
		config:      cfg,
		storagePath: cfg.StoragePath,
		registry:    NewModelRegistry(),
		downloads:   make(map[string]*DownloadProgress),
	}

	// Load existing models
	mgr.scanModels()

	return mgr, nil
}

// DownloadProgress tracks download progress
type DownloadProgress struct {
	ModelName   string    `json:"modelName"`
	Source      string    `json:"source"`
	TotalBytes  int64     `json:"totalBytes"`
	Downloaded  int64     `json:"downloaded"`
	Percentage  float64   `json:"percentage"`
	Speed       string    `json:"speed"`
	Status      string    `json:"status"` // downloading, completed, failed
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt,omitempty"`
}

// DownloadModel downloads a model from a registry
func (m *ModelManager) DownloadModel(ctx context.Context, req *ModelDownloadRequest) (*DownloadProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already downloading
	if progress, exists := m.downloads[req.ModelName]; exists {
		if progress.Status == "downloading" {
			return progress, nil
		}
	}

	progress := &DownloadProgress{
		ModelName: req.ModelName,
		Source:    req.Source,
		Status:    "downloading",
		StartedAt: time.Now(),
	}
	m.downloads[req.ModelName] = progress

	go m.doDownload(ctx, req, progress)

	return progress, nil
}

// ModelDownloadRequest represents a model download request
type ModelDownloadRequest struct {
	ModelName string `json:"modelName"`
	Source    string `json:"source"` // ollama, huggingface, url
	Version   string `json:"version,omitempty"`
}

func (m *ModelManager) doDownload(ctx context.Context, req *ModelDownloadRequest, progress *DownloadProgress) {
	defer func() {
		progress.CompletedAt = time.Now()
	}()

	switch req.Source {
	case "ollama":
		m.downloadFromOllama(ctx, req, progress)
	case "huggingface":
		m.downloadFromHuggingFace(ctx, req, progress)
	case "url":
		m.downloadFromURL(ctx, req, progress)
	default:
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("unknown source: %s", req.Source)
	}
}

func (m *ModelManager) downloadFromOllama(ctx context.Context, req *ModelDownloadRequest, progress *DownloadProgress) {
	// Use ollama pull command
	cmd := exec.CommandContext(ctx, "ollama", "pull", req.ModelName)
	cmd.Env = append(os.Environ(), fmt.Sprintf("OLLAMA_MODELS=%s", m.storagePath))

	output, err := cmd.CombinedOutput()
	if err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("ollama pull failed: %s - %s", err, string(output))
		return
	}

	progress.Status = "completed"
	progress.Percentage = 100
	m.registry.Register(req.ModelName, &LocalModel{
		Name:      req.ModelName,
		Source:    "ollama",
		LocalPath: filepath.Join(m.storagePath, "ollama", req.ModelName),
		Installed: true,
	})
}

func (m *ModelManager) downloadFromHuggingFace(ctx context.Context, req *ModelDownloadRequest, progress *DownloadProgress) {
	// Download from Hugging Face
	// Format: owner/repo or owner/repo:branch
	modelName := req.ModelName
	if req.Version != "" {
		modelName = req.ModelName + ":" + req.Version
	}

	// Use huggingface-cli if available
	cmd := exec.CommandContext(ctx, "huggingface-cli", "download", modelName, "--local-dir",
		filepath.Join(m.storagePath, "huggingface", strings.ReplaceAll(req.ModelName, "/", "_")))

	if m.config.HuggingFaceToken != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("HF_TOKEN=%s", m.config.HuggingFaceToken))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("huggingface download failed: %s - %s", err, string(output))
		return
	}

	progress.Status = "completed"
	progress.Percentage = 100
	m.registry.Register(req.ModelName, &LocalModel{
		Name:      req.ModelName,
		Source:    "huggingface",
		Version:   req.Version,
		LocalPath: filepath.Join(m.storagePath, "huggingface", strings.ReplaceAll(req.ModelName, "/", "_")),
		Installed: true,
	})
}

func (m *ModelManager) downloadFromURL(ctx context.Context, req *ModelDownloadRequest, progress *DownloadProgress) {
	if req.Version == "" {
		progress.Status = "failed"
		progress.Error = "URL required in version field"
		return
	}

	url := req.Version
	resp, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("failed to create request: %s", err)
		return
	}

	httpResp, err := http.DefaultClient.Do(resp)
	if err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("download failed: %s", err)
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode >= 400 {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("download failed: %s", httpResp.Status)
		return
	}

	progress.TotalBytes = httpResp.ContentLength

	// Create model directory
	modelDir := filepath.Join(m.storagePath, "custom", req.ModelName)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("failed to create model dir: %s", err)
		return
	}

	// Determine file extension
	ext := ".bin"
	if strings.Contains(url, ".gguf") {
		ext = ".gguf"
	} else if strings.Contains(url, ".safetensors") {
		ext = ".safetensors"
	}

	// Download to file
	outPath := filepath.Join(modelDir, "model"+ext)
	outFile, err := os.Create(outPath)
	if err != nil {
		progress.Status = "failed"
		progress.Error = fmt.Sprintf("failed to create file: %s", err)
		return
	}
	defer func() { _ = outFile.Close() }()

	buf := make([]byte, 32*1024)
	var lastUpdate time.Time
	var downloadedLastUpdate int64

	for {
		n, err := httpResp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				progress.Status = "failed"
				progress.Error = fmt.Sprintf("write failed: %s", writeErr)
				return
			}
			progress.Downloaded += int64(n)
			downloadedLastUpdate += int64(n)

			if progress.TotalBytes > 0 {
				progress.Percentage = float64(progress.Downloaded) / float64(progress.TotalBytes) * 100
			}

			// Update speed every second
			if time.Since(lastUpdate) > time.Second {
				speed := float64(downloadedLastUpdate) / time.Since(lastUpdate).Seconds()
				progress.Speed = formatSpeed(speed)
				lastUpdate = time.Now()
				downloadedLastUpdate = 0
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			progress.Status = "failed"
			progress.Error = fmt.Sprintf("read failed: %s", err)
			return
		}
	}

	progress.Status = "completed"
	progress.Percentage = 100

	m.registry.Register(req.ModelName, &LocalModel{
		Name:      req.ModelName,
		Source:    "url",
		LocalPath: modelDir,
		Installed: true,
	})
}

// DeleteModel deletes a model
func (m *ModelManager) DeleteModel(ctx context.Context, modelName string) error {
	model := m.registry.Get(modelName)
	if model == nil {
		return fmt.Errorf("model %s not found", modelName)
	}

	// Delete from storage
	if model.LocalPath != "" {
		if err := os.RemoveAll(model.LocalPath); err != nil {
			return fmt.Errorf("failed to delete model files: %w", err)
		}
	}

	// Unregister
	m.registry.Unregister(modelName)

	return nil
}

// ListModels lists all available models
func (m *ModelManager) ListModels() []*LocalModel {
	return m.registry.List()
}

// GetModel gets a model by name
func (m *ModelManager) GetModel(name string) *LocalModel {
	return m.registry.Get(name)
}

// GetDownloadProgress gets download progress for a model
func (m *ModelManager) GetDownloadProgress(modelName string) *DownloadProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.downloads[modelName]
}

// GetModelSize returns the size of a model
func (m *ModelManager) GetModelSize(modelName string) (int64, error) {
	model := m.registry.Get(modelName)
	if model == nil {
		return 0, fmt.Errorf("model not found")
	}

	if model.LocalPath == "" {
		return 0, fmt.Errorf("model not downloaded")
	}

	var size int64
	err := filepath.Walk(model.LocalPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}

// GetStorageUsage returns total storage used by models
func (m *ModelManager) GetStorageUsage() (int64, error) {
	var totalSize int64
	err := filepath.Walk(m.storagePath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	return totalSize, err
}

// scanModels scans the storage path for existing models
func (m *ModelManager) scanModels() {
	// Scan Ollama models
	ollamaPath := filepath.Join(m.storagePath, "ollama")
	if _, err := os.Stat(ollamaPath); err == nil {
		entries, _ := os.ReadDir(ollamaPath)
		for _, entry := range entries {
			if entry.IsDir() {
				m.registry.Register(entry.Name(), &LocalModel{
					Name:      entry.Name(),
					Source:    "ollama",
					LocalPath: filepath.Join(ollamaPath, entry.Name()),
					Installed: true,
				})
			}
		}
	}

	// Scan HuggingFace models
	hfPath := filepath.Join(m.storagePath, "huggingface")
	if _, err := os.Stat(hfPath); err == nil {
		entries, _ := os.ReadDir(hfPath)
		for _, entry := range entries {
			if entry.IsDir() {
				name := strings.ReplaceAll(entry.Name(), "_", "/")
				m.registry.Register(name, &LocalModel{
					Name:      name,
					Source:    "huggingface",
					LocalPath: filepath.Join(hfPath, entry.Name()),
					Installed: true,
				})
			}
		}
	}
}

// SearchModels searches for models in registries
func (m *ModelManager) SearchModels(ctx context.Context, query string, source string) ([]ModelSearchResult, error) {
	var results []ModelSearchResult

	switch source {
	case "ollama":
		return m.searchOllama(ctx, query)
	case "huggingface":
		return m.searchHuggingFace(ctx, query)
	default:
		// Search all
		ollama, _ := m.searchOllama(ctx, query)
		hf, _ := m.searchHuggingFace(ctx, query)
		results = append(results, ollama...)
		results = append(results, hf...)
	}

	return results, nil
}

func (m *ModelManager) searchOllama(ctx context.Context, query string) ([]ModelSearchResult, error) {
	// Ollama library API
	url := "https://ollama.com/api/models"
	resp, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	httpResp, err := http.DefaultClient.Do(resp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	var models []struct {
		Name string `json:"name"`
		Desc string `json:"description"`
		Tags []struct {
			Name   string `json:"name"`
			Size   string `json:"size"`
			Params string `json:"parameter_size"`
		} `json:"tags"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&models); err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	var results []ModelSearchResult
	for _, model := range models {
		if query != "" && !strings.Contains(strings.ToLower(model.Name), queryLower) {
			continue
		}
		for _, tag := range model.Tags {
			results = append(results, ModelSearchResult{
				Name:        model.Name + ":" + tag.Name,
				DisplayName: model.Name,
				Description: model.Desc,
				Source:      "ollama",
				Size:        tag.Size,
				Parameters:  tag.Params,
			})
		}
	}

	return results, nil
}

func (m *ModelManager) searchHuggingFace(ctx context.Context, query string) ([]ModelSearchResult, error) {
	// HuggingFace model hub API
	url := fmt.Sprintf("https://huggingface.co/api/models?search=%s&limit=20", query)
	resp, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if m.config.HuggingFaceToken != "" {
		resp.Header.Set("Authorization", "Bearer "+m.config.HuggingFaceToken)
	}

	httpResp, err := http.DefaultClient.Do(resp)
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	var models []struct {
		ID        string   `json:"id"`
		Author    string   `json:"author"`
		ModelID   string   `json:"modelId"`
		Downloads int      `json:"downloads"`
		Tags      []string `json:"tags"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&models); err != nil {
		return nil, err
	}

	results := make([]ModelSearchResult, len(models))
	for i, model := range models {
		results[i] = ModelSearchResult{
			Name:        model.ID,
			DisplayName: model.ModelID,
			Source:      "huggingface",
			Downloads:   model.Downloads,
			Tags:        model.Tags,
		}
	}

	return results, nil
}

// ModelSearchResult represents a model search result
type ModelSearchResult struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source"`
	Size        string   `json:"size,omitempty"`
	Parameters  string   `json:"parameters,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Installed   bool     `json:"installed,omitempty"`
}

// LocalModel represents a locally installed model
type LocalModel struct {
	Name       string                 `json:"name"`
	Source     string                 `json:"source"`
	Version    string                 `json:"version,omitempty"`
	LocalPath  string                 `json:"localPath"`
	Installed  bool                   `json:"installed"`
	Size       int64                  `json:"size,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	ModifiedAt time.Time              `json:"modifiedAt,omitempty"`
}

// ModelRegistry tracks installed models
type ModelRegistry struct {
	models map[string]*LocalModel
	mu     sync.RWMutex
}

// NewModelRegistry creates a new registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*LocalModel),
	}
}

// Register adds a model to the registry
func (r *ModelRegistry) Register(name string, model *LocalModel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	model.ModifiedAt = time.Now()
	r.models[name] = model
}

// Unregister removes a model from the registry
func (r *ModelRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.models, name)
}

// Get retrieves a model from the registry
func (r *ModelRegistry) Get(name string) *LocalModel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.models[name]
}

// List returns all registered models
func (r *ModelRegistry) List() []*LocalModel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	models := make([]*LocalModel, 0, len(r.models))
	for _, m := range r.models {
		models = append(models, m)
	}
	return models
}

// ResourceMonitor monitors system resources for AI workloads
type ResourceMonitor struct {
	_ bool // gpuEnabled - reserved for future GPU detection
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{}
}

// GetGPUInfo returns GPU information
func (m *ResourceMonitor) GetGPUInfo() ([]GPUInfo, error) {
	// Try nvidia-smi
	cmd := exec.CommandContext(context.Background(), "nvidia-smi", "--query-gpu=index,name,memory.total,memory.used,memory.free,utilization.gpu", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi not available: %w", err)
	}

	var resp struct {
		GPUs []struct {
			Index       string `json:"index"`
			Name        string `json:"name"`
			MemoryTotal string `json:"memory.total"`
			MemoryUsed  string `json:"memory.used"`
			MemoryFree  string `json:"memory.free"`
			UtilGPU     string `json:"utilization.gpu"`
		} `json:"gpu"`
	}

	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, err
	}

	gpus := make([]GPUInfo, len(resp.GPUs))
	for i, gpu := range resp.GPUs {
		gpus[i] = GPUInfo{
			Index:       parseInt(gpu.Index),
			Name:        gpu.Name,
			MemoryTotal: parseMemory(gpu.MemoryTotal),
			MemoryUsed:  parseMemory(gpu.MemoryUsed),
			MemoryFree:  parseMemory(gpu.MemoryFree),
			Utilization: parseInt(gpu.UtilGPU),
		}
	}

	return gpus, nil
}

// GetMemoryInfo returns system memory information
func (m *ResourceMonitor) GetMemoryInfo() (*MemoryInfo, error) {
	// Read /proc/meminfo
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	info := &MemoryInfo{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		value := parseInt(parts[1])
		switch parts[0] {
		case "MemTotal:":
			info.Total = int64(value) * 1024
		case "MemFree:":
			info.Free = int64(value) * 1024
		case "MemAvailable:":
			info.Available = int64(value) * 1024
		}
	}

	info.Used = info.Total - info.Available
	return info, nil
}

// GPUInfo represents GPU information
type GPUInfo struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	MemoryTotal int64  `json:"memoryTotal"`
	MemoryUsed  int64  `json:"memoryUsed"`
	MemoryFree  int64  `json:"memoryFree"`
	Utilization int    `json:"utilization"`
}

// MemoryInfo represents system memory information
type MemoryInfo struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Free      int64 `json:"free"`
	Available int64 `json:"available"`
}

// Helper functions
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else if bytesPerSec < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB/s", bytesPerSec/(1024*1024*1024))
}

func parseInt(s string) int {
	var result int
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &result)
	return result
}

func parseMemory(s string) int64 {
	// Parse memory string like "8192 MiB"
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}
	value := parseInt(match)
	if strings.Contains(s, "GiB") || strings.Contains(s, "GB") {
		return int64(value) * 1024 * 1024 * 1024
	}
	return int64(value) * 1024 * 1024
}

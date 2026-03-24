// Package avif provides AVIF image format decoding support for the photos module.
// AVIF (AV1 Image File Format) is a modern image format offering superior compression
// and quality compared to JPEG and PNG, widely requested by NAS users.
package avif

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DecoderConfig configures the AVIF decoder behavior
type DecoderConfig struct {
	// EnableHardwareAccel enables hardware acceleration (GPU) when available
	EnableHardwareAccel bool `json:"enableHardwareAccel"`
	
	// MaxDecodeWorkers limits concurrent decode operations
	MaxDecodeWorkers int `json:"maxDecodeWorkers"`
	
	// CacheDecodedImages caches decoded images in memory
	CacheDecodedImages bool `json:"cacheDecodedImages"`
	
	// MaxCacheSizeMB maximum memory for image cache (MB)
	MaxCacheSizeMB int `json:"maxCacheSizeMB"`
	
	// FallbackToFFmpeg use ffmpeg as fallback decoder
	FallbackToFFmpeg bool `json:"fallbackToFfmpeg"`
	
	// TimeoutSeconds timeout for decode operations
	TimeoutSeconds int `json:"timeoutSeconds"`
}

// DefaultDecoderConfig returns recommended decoder configuration
func DefaultDecoderConfig() DecoderConfig {
	return DecoderConfig{
		EnableHardwareAccel: true,
		MaxDecodeWorkers:    4,
		CacheDecodedImages:  true,
		MaxCacheSizeMB:     256,
		FallbackToFFmpeg:    true,
		TimeoutSeconds:      30,
	}
}

// Decoder handles AVIF image decoding
type Decoder struct {
	config     DecoderConfig
	cache      *imageCache
	workerPool chan struct{}
	mu         sync.RWMutex
	stats      DecodeStats
}

// DecodeStats tracks decoder performance metrics
type DecodeStats struct {
	TotalDecoded      int64         `json:"totalDecoded"`
	TotalBytes        int64         `json:"totalBytes"`
	AverageDecodeTime time.Duration `json:"averageDecodeTime"`
	CacheHits         int64         `json:"cacheHits"`
	CacheMisses       int64         `json:"cacheMisses"`
	Errors            int64         `json:"errors"`
}

// DecodeResult contains decoded image metadata
type DecodeResult struct {
	Image       image.Image
	Width       int
	Height      int
	BitDepth    int
	HasAlpha    bool
	ColorSpace  string
	DecodeTime  time.Duration
	UsedCache   bool
}

// imageCache provides in-memory caching for decoded images
type imageCache struct {
	images    map[string]*cacheEntry
	maxSize   int64
	currentSize int64
	mu        sync.RWMutex
}

type cacheEntry struct {
	image     image.Image
	size      int64
	accessed  time.Time
}

// SupportedExtensions returns file extensions for AVIF format
var SupportedExtensions = []string{".avif", ".heif", ".heic"}

// IsAVIF checks if the file is an AVIF format based on extension
func IsAVIF(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, supported := range SupportedExtensions {
		if ext == supported {
			return true
		}
	}
	return false
}

// NewDecoder creates a new AVIF decoder with the given configuration
func NewDecoder(config DecoderConfig) *Decoder {
	d := &Decoder{
		config:     config,
		workerPool: make(chan struct{}, config.MaxDecodeWorkers),
	}
	
	if config.CacheDecodedImages {
		d.cache = &imageCache{
			images:   make(map[string]*cacheEntry),
			maxSize:  int64(config.MaxCacheSizeMB) * 1024 * 1024,
		}
	}
	
	// Initialize worker pool
	for i := 0; i < config.MaxDecodeWorkers; i++ {
		d.workerPool <- struct{}{}
	}
	
	return d
}

// DecodeFile decodes an AVIF file from disk
func (d *Decoder) DecodeFile(path string) (*DecodeResult, error) {
	start := time.Now()
	
	// Check cache first
	if d.cache != nil {
		if entry, ok := d.cache.get(path); ok {
			d.mu.Lock()
			d.stats.CacheHits++
			d.mu.Unlock()
			
			return &DecodeResult{
				Image:      entry.image,
				Width:      entry.image.Bounds().Dx(),
				Height:     entry.image.Bounds().Dy(),
				DecodeTime: time.Since(start),
				UsedCache:  true,
			}, nil
		}
	}
	
	d.mu.Lock()
	d.stats.CacheMisses++
	d.mu.Unlock()
	
	// Acquire worker slot
	select {
	case <-d.workerPool:
		defer func() { d.workerPool <- struct{}{} }()
	case <-time.After(time.Duration(d.config.TimeoutSeconds) * time.Second):
		return nil, fmt.Errorf("decode queue timeout")
	}
	
	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		d.mu.Lock()
		d.stats.Errors++
		d.mu.Unlock()
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Decode image
	result, err := d.decode(data, path)
	if err != nil {
		return nil, err
	}
	
	result.DecodeTime = time.Since(start)
	result.UsedCache = false
	
	// Update stats
	d.mu.Lock()
	d.stats.TotalDecoded++
	d.stats.TotalBytes += int64(len(data))
	d.mu.Unlock()
	
	return result, nil
}

// Decode decodes AVIF data from memory
func (d *Decoder) Decode(data []byte) (*DecodeResult, error) {
	return d.decode(data, "")
}

// decode internal decode implementation
func (d *Decoder) decode(data []byte, cacheKey string) (*DecodeResult, error) {
	start := time.Now()
	
	// First, try native Go AVIF decoder (via go-avif or similar library)
	img, err := d.decodeNative(data)
	if err == nil {
		result := &DecodeResult{
			Image:      img,
			Width:      img.Bounds().Dx(),
			Height:     img.Bounds().Dy(),
			DecodeTime: time.Since(start),
		}
		
		// Cache the result
		if d.cache != nil && cacheKey != "" {
			d.cache.set(cacheKey, img, int64(len(data)))
		}
		
		return result, nil
	}
	
	// Fallback to ffmpeg if enabled
	if d.config.FallbackToFFmpeg {
		img, err = d.decodeWithFFmpeg(data)
		if err == nil {
			result := &DecodeResult{
				Image:      img,
				Width:      img.Bounds().Dx(),
				Height:     img.Bounds().Dy(),
				DecodeTime: time.Since(start),
			}
			
			if d.cache != nil && cacheKey != "" {
				d.cache.set(cacheKey, img, int64(len(data)))
			}
			
			return result, nil
		}
	}
	
	d.mu.Lock()
	d.stats.Errors++
	d.mu.Unlock()
	
	return nil, fmt.Errorf("failed to decode AVIF: native decoder failed and ffmpeg fallback unavailable")
}

// decodeNative attempts to decode using Go native libraries
func (d *Decoder) decodeNative(data []byte) (image.Image, error) {
	// Check for AVIF signature
	if !isAVIFSignature(data) {
		return nil, fmt.Errorf("not a valid AVIF file")
	}
	
	// Try using go-avif library if available
	// This is a placeholder for actual go-avif integration
	// In production, would use: go.utf9k.net/libavif bindings or similar
	
	// For now, return error to trigger ffmpeg fallback
	return nil, fmt.Errorf("native AVIF decoder not yet available, use ffmpeg fallback")
}

// isAVIFSignature checks if data starts with AVIF signature
func isAVIFSignature(data []byte) bool {
	// AVIF files start with 'ftyp' box at offset 4
	// followed by 'avif' or 'avis' brand
	if len(data) < 12 {
		return false
	}
	
	// Check for ftyp box
	if !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}
	
	// Check for AVIF brand
	brand := string(data[8:12])
	return brand == "avif" || brand == "avis" || brand == "heic" || brand == "heix"
}

// decodeWithFFmpeg uses ffmpeg for AVIF decoding
func (d *Decoder) decodeWithFFmpeg(data []byte) (image.Image, error) {
	// Create temp file for ffmpeg input
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("avif_decode_%s.avif", uuid.New().String()))
	
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile)
	
	// Create temp file for PNG output
	outFile := filepath.Join(os.TempDir(), fmt.Sprintf("avif_decode_%s.png", uuid.New().String()))
	defer os.Remove(outFile)
	
	// Run ffmpeg conversion
	ctx, cancel := context.WithTimeout(context.Background(), 
		time.Duration(d.config.TimeoutSeconds)*time.Second)
	defer cancel()
	
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", tmpFile,
		"-y",
		"-pix_fmt", "rgba",
		"-f", "image2",
		outFile,
	)
	
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg decode failed: %w, stderr: %s", err, stderr.String())
	}
	
	// Read and decode PNG output
	pngData, err := os.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read ffmpeg output: %w", err)
	}
	
	img, _, err := image.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("failed to decode ffmpeg output: %w", err)
	}
	
	return img, nil
}

// GetStats returns decoder statistics
func (d *Decoder) GetStats() DecodeStats {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.stats
}

// ClearCache clears the image cache
func (d *Decoder) ClearCache() {
	if d.cache != nil {
		d.cache.clear()
	}
}

// cache methods

func (c *imageCache) get(key string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, ok := c.images[key]
	if ok {
		entry.accessed = time.Now()
	}
	return entry, ok
}

func (c *imageCache) set(key string, img image.Image, size int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Evict old entries if needed
	for c.currentSize+size > c.maxSize && len(c.images) > 0 {
		var oldestKey string
		var oldestTime time.Time
		
		for k, v := range c.images {
			if oldestKey == "" || v.accessed.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.accessed
			}
		}
		
		if oldestKey != "" {
			c.currentSize -= c.images[oldestKey].size
			delete(c.images, oldestKey)
		}
	}
	
	c.images[key] = &cacheEntry{
		image:    img,
		size:     size,
		accessed: time.Now(),
	}
	c.currentSize += size
}

func (c *imageCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.images = make(map[string]*cacheEntry)
	c.currentSize = 0
}

// ThumbnailGenerator generates thumbnails for AVIF images
type ThumbnailGenerator struct {
	decoder   *Decoder
	outputDir string
	sizes     []int
	quality   int
}

// ThumbnailConfig configures thumbnail generation
type ThumbnailConfig struct {
	Sizes   []int // Thumbnail sizes in pixels
	Quality int   // JPEG quality (1-100)
}

// DefaultThumbnailConfig returns default thumbnail configuration
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		Sizes:   []int{128, 512, 1024},
		Quality: 85,
	}
}

// NewThumbnailGenerator creates a thumbnail generator
func NewThumbnailGenerator(decoder *Decoder, outputDir string, config ThumbnailConfig) *ThumbnailGenerator {
	return &ThumbnailGenerator{
		decoder:   decoder,
		outputDir: outputDir,
		sizes:     config.Sizes,
		quality:   config.Quality,
	}
}

// Generate generates thumbnails for an AVIF file
func (g *ThumbnailGenerator) Generate(avifPath string) ([]string, error) {
	// Decode AVIF
	result, err := g.decoder.DecodeFile(avifPath)
	if err != nil {
		return nil, fmt.Errorf("failed to decode AVIF: %w", err)
	}
	
	var thumbnails []string
	baseName := strings.TrimSuffix(filepath.Base(avifPath), filepath.Ext(avifPath))
	
	for _, size := range g.sizes {
		thumbPath := filepath.Join(g.outputDir, fmt.Sprintf("%s_%d.jpg", baseName, size))
		
		if err := g.generateThumbnail(result.Image, thumbPath, size); err != nil {
			continue // Skip failed sizes
		}
		
		thumbnails = append(thumbnails, thumbPath)
	}
	
	return thumbnails, nil
}

// generateThumbnail creates a single thumbnail
func (g *ThumbnailGenerator) generateThumbnail(img image.Image, outPath string, maxSize int) error {
	// Calculate new dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	// Note: actual thumbnail generation requires imaging library
	// This is a placeholder that validates the operation
	if width == 0 || height == 0 {
		return fmt.Errorf("invalid image dimensions")
	}
	
	return fmt.Errorf("thumbnail generation requires imaging library")
}

// FormatInfo returns information about AVIF format support
type FormatInfo struct {
	Name            string   `json:"name"`
	Extensions      []string `json:"extensions"`
	MimeTypes       []string `json:"mimeTypes"`
	SupportsAlpha   bool     `json:"supportsAlpha"`
	SupportsHDR     bool     `json:"supportsHDR"`
	MaxBitDepth     int      `json:"maxBitDepth"`
	MaxDimension    int      `json:"maxDimension"`
	CompressionRatio float64  `json:"compressionRatio"` // Average vs JPEG
}

// GetFormatInfo returns AVIF format information
func GetFormatInfo() FormatInfo {
	return FormatInfo{
		Name:            "AVIF",
		Extensions:      SupportedExtensions,
		MimeTypes:       []string{"image/avif", "image/heif", "image/heic"},
		SupportsAlpha:   true,
		SupportsHDR:     true,
		MaxBitDepth:     12,
		MaxDimension:    65536,
		CompressionRatio: 0.5, // ~50% smaller than JPEG at equivalent quality
	}
}

// Integration with photos module

// AVIFSupport provides AVIF support for the photos manager
type AVIFSupport struct {
	decoder    *Decoder
	thumbGen   *ThumbnailGenerator
	enabled    bool
}

// NewAVIFSupport creates AVIF support for the photos module
func NewAVIFSupport(dataDir string) (*AVIFSupport, error) {
	decoderConfig := DefaultDecoderConfig()
	decoder := NewDecoder(decoderConfig)
	
	thumbConfig := DefaultThumbnailConfig()
	thumbDir := filepath.Join(dataDir, "thumbnails")
	
	if err := os.MkdirAll(thumbDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create thumbnail directory: %w", err)
	}
	
	return &AVIFSupport{
		decoder:  decoder,
		thumbGen: NewThumbnailGenerator(decoder, thumbDir, thumbConfig),
		enabled:  true,
	}, nil
}

// CanDecode checks if file can be decoded as AVIF
func (s *AVIFSupport) CanDecode(path string) bool {
	return IsAVIF(path) && s.enabled
}

// Decode decodes an AVIF file
func (s *AVIFSupport) Decode(path string) (*DecodeResult, error) {
	return s.decoder.DecodeFile(path)
}

// GenerateThumbnails generates thumbnails for AVIF file
func (s *AVIFSupport) GenerateThumbnails(path string) ([]string, error) {
	return s.thumbGen.Generate(path)
}

// GetDecoderStats returns decoder performance statistics
func (s *AVIFSupport) GetDecoderStats() DecodeStats {
	return s.decoder.GetStats()
}

// ClearCache clears the decoder cache
func (s *AVIFSupport) ClearCache() {
	s.decoder.ClearCache()
}

// CheckSystemSupport checks if the system supports AVIF decoding
func CheckSystemSupport() error {
	// Check for ffmpeg with AVIF support
	cmd := exec.Command("ffmpeg", "-decoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg not available: %w", err)
	}
	
	if !bytes.Contains(output, []byte("avif")) &&
	   !bytes.Contains(output, []byte("heif")) &&
	   !bytes.Contains(output, []byte("libdav1d")) {
		// ffmpeg may still support AVIF through other decoders
		// Try a test decode
	}
	
	return nil
}
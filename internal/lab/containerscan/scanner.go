// Package containerscan provides Docker image vulnerability scanning with CVE detection,
// layer analysis, severity rating, auto-fix suggestions, scheduled scanning, and report generation.
package containerscan

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// VulnDatabase is an interface for vulnerability database lookups.
type VulnDatabase interface {
	Lookup(ctx context.Context, pkg, version, distro string) ([]CVE, error)
	Update(ctx context.Context) error
	LastUpdate() time.Time
}

// Scanner performs vulnerability scanning on container images.
type Scanner struct {
	logger        *zap.Logger
	cache         map[string]*ScanResult
	cacheMu       sync.RWMutex
	cacheTTL      time.Duration
	vulnDB        VulnDatabase
	layerAnalyzer *LayerAnalyzer
}

// LayerAnalyzer analyzes image layers.
type LayerAnalyzer struct {
	logger *zap.Logger
}

// packageInfo represents a package found in an image layer.
type packageInfo struct {
	Name    string
	Version string
	Path    string
}

// NewScanner creates a new image scanner.
func NewScanner(logger *zap.Logger, vulnDB VulnDatabase, cacheTTL time.Duration) *Scanner {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cacheTTL == 0 {
		cacheTTL = 1 * time.Hour
	}
	return &Scanner{
		logger:        logger,
		cache:         make(map[string]*ScanResult),
		cacheTTL:      cacheTTL,
		vulnDB:        vulnDB,
		layerAnalyzer: &LayerAnalyzer{logger: logger},
	}
}

// ScanImage performs a full vulnerability scan on a container image.
func (s *Scanner) ScanImage(ctx context.Context, image, registry string, forceRescan bool) (*ScanResult, error) {
	startTime := time.Now()
	fullImage := image
	if registry != "" && !strings.Contains(image, "/") {
		fullImage = registry + "/" + image
	}

	// Check cache
	if !forceRescan {
		if cached := s.getFromCache(fullImage); cached != nil {
			s.logger.Info("returning cached scan result", zap.String("image", fullImage))
			return cached, nil
		}
	}

	s.logger.Info("starting image scan", zap.String("image", fullImage))

	// Extract image metadata
	digest, layers, err := s.inspectImage(ctx, fullImage)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect image: %w", err)
	}

	// Scan layers for vulnerabilities
	var allVulns []Vulnerability
	for i, layer := range layers {
		layerVulns, err := s.scanLayer(ctx, fullImage, layer, i)
		if err != nil {
			s.logger.Warn("failed to scan layer",
				zap.String("layer", layer.Digest),
				zap.Error(err))
			continue
		}
		allVulns = append(allVulns, layerVulns...)
		layers[i].Vulns = len(layerVulns)
	}

	// Build summary
	summary := s.buildSummary(allVulns)

	// Generate fix suggestions
	fixSuggestions := s.generateFixSuggestions(allVulns)

	result := &ScanResult{
		Image:          fullImage,
		Registry:       registry,
		Digest:         digest,
		ScanTime:       startTime,
		Duration:       time.Since(startTime),
		Vulns:          allVulns,
		Layers:         layers,
		Summary:        summary,
		FixSuggestions: fixSuggestions,
		Compliant:      summary.Critical == 0 && summary.High <= 5,
	}

	// Cache result
	s.setCache(fullImage, result)

	s.logger.Info("scan completed",
		zap.String("image", fullImage),
		zap.Int("total_vulns", summary.Total),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// inspectImage extracts image metadata (digest, layers).
func (s *Scanner) inspectImage(ctx context.Context, image string) (string, []ImageLayer, error) {
	s.logger.Info("inspecting image", zap.String("image", image))

	// Simulate image inspection
	digest := fmt.Sprintf("sha256:%x", time.Now().UnixNano())
	layers := []ImageLayer{
		{
			Digest:    fmt.Sprintf("sha256:%x", 1),
			Size:      1024 * 1024 * 50,
			CreatedAt: time.Now().Add(-48 * time.Hour),
			Command:   "FROM ubuntu:22.04",
		},
		{
			Digest:    fmt.Sprintf("sha256:%x", 2),
			Size:      1024 * 1024 * 10,
			CreatedAt: time.Now().Add(-24 * time.Hour),
			Command:   "RUN apt-get update && apt-get install -y curl",
		},
		{
			Digest:    fmt.Sprintf("sha256:%x", 3),
			Size:      1024 * 1024 * 5,
			CreatedAt: time.Now().Add(-12 * time.Hour),
			Command:   "COPY app /app",
		},
	}

	return digest, layers, nil
}

// scanLayer scans a single image layer for vulnerabilities.
func (s *Scanner) scanLayer(ctx context.Context, image string, layer ImageLayer, index int) ([]Vulnerability, error) {
	if s.vulnDB == nil {
		return nil, nil
	}

	packages := s.extractPackages(layer)

	var vulns []Vulnerability
	for _, pkg := range packages {
		cves, err := s.vulnDB.Lookup(ctx, pkg.Name, pkg.Version, "ubuntu")
		if err != nil {
			s.logger.Warn("vuln lookup failed",
				zap.String("package", pkg.Name),
				zap.Error(err))
			continue
		}

		for _, cve := range cves {
			vulns = append(vulns, Vulnerability{
				CVE:          cve,
				Layer:        layer.Digest,
				InstalledPkg: pkg.Name,
				Path:         pkg.Path,
			})
		}
	}

	return vulns, nil
}

// extractPackages extracts package information from a layer.
func (s *Scanner) extractPackages(layer ImageLayer) []packageInfo {
	var packages []packageInfo

	if strings.Contains(layer.Command, "apt-get") {
		packages = append(packages, packageInfo{
			Name:    "curl",
			Version: "7.81.0-1ubuntu1.16",
			Path:    "/usr/bin/curl",
		})
	}

	return packages
}

// buildSummary creates a vulnerability summary.
func (s *Scanner) buildSummary(vulns []Vulnerability) VulnSummary {
	summary := VulnSummary{Total: len(vulns)}
	for _, v := range vulns {
		switch v.Severity {
		case SeverityCritical:
			summary.Critical++
		case SeverityHigh:
			summary.High++
		case SeverityMedium:
			summary.Medium++
		case SeverityLow:
			summary.Low++
		default:
			summary.Info++
		}
	}
	return summary
}

// generateFixSuggestions creates auto-fix suggestions for vulnerabilities.
func (s *Scanner) generateFixSuggestions(vulns []Vulnerability) []FixSuggestion {
	var suggestions []FixSuggestion
	seen := make(map[string]bool)

	for _, v := range vulns {
		if v.FixedIn == "" {
			continue
		}
		key := v.Package + ":" + v.FixedIn
		if seen[key] {
			continue
		}
		seen[key] = true

		suggestions = append(suggestions, FixSuggestion{
			VulnID:      v.ID,
			Package:     v.Package,
			CurrentVer:  v.Version,
			FixedVer:    v.FixedIn,
			Command:     fmt.Sprintf("apt-get update && apt-get install -y %s=%s", v.Package, v.FixedIn),
			Description: fmt.Sprintf("Upgrade %s from %s to %s to fix %s", v.Package, v.Version, v.FixedIn, v.ID),
		})
	}

	return suggestions
}

// getFromCache retrieves a cached scan result.
func (s *Scanner) getFromCache(image string) *ScanResult {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	result, exists := s.cache[image]
	if !exists {
		return nil
	}

	if time.Since(result.ScanTime) > s.cacheTTL {
		return nil
	}

	return result
}

// setCache stores a scan result in cache.
func (s *Scanner) setCache(image string, result *ScanResult) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache[image] = result
}

// ClearCache clears the scan cache.
func (s *Scanner) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.cache = make(map[string]*ScanResult)
}

// GetCachedResult returns a cached result without scanning.
func (s *Scanner) GetCachedResult(image string) *ScanResult {
	return s.getFromCache(image)
}

// ListCachedImages returns all cached image names.
func (s *Scanner) ListCachedImages() []string {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	images := make([]string, 0, len(s.cache))
	for image := range s.cache {
		images = append(images, image)
	}
	return images
}

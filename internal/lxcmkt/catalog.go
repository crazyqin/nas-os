package lxcmkt

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages the LXC template catalog
type Manager struct {
	logger    *zap.Logger
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewManager creates a new template catalog manager
func NewManager(logger *zap.Logger) *Manager {
	m := &Manager{
		logger:    logger,
		templates: make(map[string]*Template),
	}
	m.loadDefaults()
	return m
}

// loadDefaults loads built-in templates
func (m *Manager) loadDefaults() {
	defaults := []Template{
		{
			ID:          "ubuntu-22.04",
			Name:        "Ubuntu 22.04 LTS",
			Description: "Ubuntu 22.04 LTS (Jammy Jellyfish) - Long Term Support",
			Distro:      "ubuntu",
			Version:     "22.04",
			Arch:        "amd64",
			Tags:        []string{"lts", "server", "desktop"},
			ImageURL:    "https://images.linuxcontainers.org/images/ubuntu/jammy/amd64/default/",
			Size:        256 * 1024 * 1024, // 256MB
			Downloads:   0,
			Rating:      4.5,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "2",
					Memory: "512MB",
					Disk:   "10GB",
				},
			},
		},
		{
			ID:          "ubuntu-24.04",
			Name:        "Ubuntu 24.04 LTS",
			Description: "Ubuntu 24.04 LTS (Noble Numbat) - Latest Long Term Support",
			Distro:      "ubuntu",
			Version:     "24.04",
			Arch:        "amd64",
			Tags:        []string{"lts", "server", "desktop", "latest"},
			ImageURL:    "https://images.linuxcontainers.org/images/ubuntu/noble/amd64/default/",
			Size:        280 * 1024 * 1024,
			Downloads:   0,
			Rating:      4.6,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "2",
					Memory: "512MB",
					Disk:   "10GB",
				},
			},
		},
		{
			ID:          "debian-12",
			Name:        "Debian 12",
			Description: "Debian 12 (Bookworm) - Stable release",
			Distro:      "debian",
			Version:     "12",
			Arch:        "amd64",
			Tags:        []string{"stable", "server", "minimal"},
			ImageURL:    "https://images.linuxcontainers.org/images/debian/bookworm/amd64/default/",
			Size:        200 * 1024 * 1024,
			Downloads:   0,
			Rating:      4.4,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "1",
					Memory: "256MB",
					Disk:   "5GB",
				},
			},
		},
		{
			ID:          "alpine-3.19",
			Name:        "Alpine Linux 3.19",
			Description: "Alpine Linux 3.19 - Lightweight and security-oriented",
			Distro:      "alpine",
			Version:     "3.19",
			Arch:        "amd64",
			Tags:        []string{"minimal", "security", "lightweight"},
			ImageURL:    "https://images.linuxcontainers.org/images/alpine/3.19/amd64/default/",
			Size:        50 * 1024 * 1024,
			Downloads:   0,
			Rating:      4.3,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "1",
					Memory: "128MB",
					Disk:   "2GB",
				},
			},
		},
		{
			ID:          "centos-stream-9",
			Name:        "CentOS Stream 9",
			Description: "CentOS Stream 9 - Rolling release, RHEL upstream",
			Distro:      "centos",
			Version:     "9",
			Arch:        "amd64",
			Tags:        []string{"enterprise", "rhel", "stream"},
			ImageURL:    "https://images.linuxcontainers.org/images/centos/9-Stream/amd64/default/",
			Size:        300 * 1024 * 1024,
			Downloads:   0,
			Rating:      4.0,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "2",
					Memory: "512MB",
					Disk:   "10GB",
				},
			},
		},
		{
			ID:          "fedora-39",
			Name:        "Fedora 39",
			Description: "Fedora 39 - Cutting edge, upstream innovation",
			Distro:      "fedora",
			Version:     "39",
			Arch:        "amd64",
			Tags:        []string{"bleeding-edge", "desktop", "server"},
			ImageURL:    "https://images.linuxcontainers.org/images/fedora/39/amd64/default/",
			Size:        350 * 1024 * 1024,
			Downloads:   0,
			Rating:      4.2,
			RatingCount: 0,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Config: TemplateConfig{
				Network: NetworkConfig{
					Interface: "eth0",
					Type:      "bridge",
				},
				Resources: ResourceLimits{
					CPUs:   "2",
					Memory: "1GB",
					Disk:   "15GB",
				},
			},
		},
	}

	for i := range defaults {
		m.templates[defaults[i].ID] = &defaults[i]
	}

	m.logger.Info("loaded default templates", zap.Int("count", len(defaults)))
}

// GetAll returns all templates
func (m *Manager) GetAll() []Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]Template, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, *t)
	}
	return templates
}

// GetByID returns a template by ID
func (m *Manager) GetByID(id string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return t, nil
}

// Search searches templates by query
func (m *Manager) Search(q SearchQuery) SearchResults {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 100 {
		q.PageSize = 20
	}

	var matched []Template
	for _, t := range m.templates {
		if m.matchTemplate(t, q) {
			matched = append(matched, *t)
		}
	}

	m.sortTemplates(matched, q.SortBy)

	total := len(matched)
	totalPages := (total + q.PageSize - 1) / q.PageSize

	start := (q.Page - 1) * q.PageSize
	end := start + q.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return SearchResults{
		Templates:  matched[start:end],
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}
}

// matchTemplate checks if a template matches search query
func (m *Manager) matchTemplate(t *Template, q SearchQuery) bool {
	// Filter by distro
	if q.Distro != "" && !strings.EqualFold(t.Distro, q.Distro) {
		return false
	}

	// Filter by arch
	if q.Arch != "" && !strings.EqualFold(t.Arch, q.Arch) {
		return false
	}

	// Filter by minimum rating
	if q.MinRating > 0 && t.Rating < q.MinRating {
		return false
	}

	// Filter by tags
	if len(q.Tags) > 0 {
		hasTag := false
		for _, qt := range q.Tags {
			for _, tt := range t.Tags {
				if strings.EqualFold(qt, tt) {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// Filter by query text
	if q.Query != "" {
		query := strings.ToLower(q.Query)
		nameMatch := strings.Contains(strings.ToLower(t.Name), query)
		descMatch := strings.Contains(strings.ToLower(t.Description), query)
		idMatch := strings.Contains(strings.ToLower(t.ID), query)
		if !nameMatch && !descMatch && !idMatch {
			return false
		}
	}

	return true
}

// sortTemplates sorts templates by the given field
func (m *Manager) sortTemplates(templates []Template, sortBy string) {
	// Simple bubble sort for small datasets
	for i := 0; i < len(templates); i++ {
		for j := i + 1; j < len(templates); j++ {
			swap := false
			switch sortBy {
			case "rating":
				swap = templates[j].Rating > templates[i].Rating
			case "downloads":
				swap = templates[j].Downloads > templates[i].Downloads
			case "created_at":
				swap = templates[j].CreatedAt.After(templates[i].CreatedAt)
			default: // name
				swap = templates[j].Name < templates[i].Name
			}
			if swap {
				templates[i], templates[j] = templates[j], templates[i]
			}
		}
	}
}

// Rate adds a rating to a template
func (m *Manager) Rate(templateID string, score int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.templates[templateID]
	if !ok {
		return fmt.Errorf("template not found: %s", templateID)
	}

	if score < 1 || score > 5 {
		return fmt.Errorf("invalid score: %d, must be 1-5", score)
	}

	// Calculate new average
	totalScore := t.Rating * float64(t.RatingCount)
	t.RatingCount++
	t.Rating = (totalScore + float64(score)) / float64(t.RatingCount)
	t.UpdatedAt = time.Now()

	return nil
}

// IncrementDownloads increments the download count
func (m *Manager) IncrementDownloads(templateID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.templates[templateID]
	if !ok {
		return fmt.Errorf("template not found: %s", templateID)
	}

	t.Downloads++
	t.UpdatedAt = time.Now()
	return nil
}

// GetStats returns catalog statistics
func (m *Manager) GetStats() StatsResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := StatsResponse{
		TotalTemplates: len(m.templates),
		TopDistro:      make(map[string]int),
		TopArch:        make(map[string]int),
	}

	for _, t := range m.templates {
		stats.TotalDownloads += t.Downloads
		stats.TopDistro[t.Distro]++
		stats.TopArch[t.Arch]++
	}

	return stats
}

// AddTemplate adds a custom template
func (m *Manager) AddTemplate(t *Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[t.ID]; exists {
		return fmt.Errorf("template already exists: %s", t.ID)
	}

	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	t.UpdatedAt = time.Now()

	m.templates[t.ID] = t
	m.logger.Info("template added", zap.String("id", t.ID))
	return nil
}

// UpdateTemplate updates an existing template
func (m *Manager) UpdateTemplate(t *Template) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[t.ID]; !exists {
		return fmt.Errorf("template not found: %s", t.ID)
	}

	t.UpdatedAt = time.Now()
	m.templates[t.ID] = t
	return nil
}

// DeleteTemplate removes a template
func (m *Manager) DeleteTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.templates[id]; !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	delete(m.templates, id)
	m.logger.Info("template deleted", zap.String("id", id))
	return nil
}

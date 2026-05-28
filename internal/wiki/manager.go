// Package wiki provides Wiki knowledge base management functionality.
package wiki

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// Manager manages Wiki spaces and pages.
type Manager struct {
	mu         sync.RWMutex
	spaces     map[string]*Space
	pages      map[string]*Page
	versions   map[string][]*PageVersion // pageID -> versions
	configPath string
}

// NewManager creates a new Wiki manager.
func NewManager(configPath string) *Manager {
	m := &Manager{
		spaces:     make(map[string]*Space),
		pages:      make(map[string]*Page),
		versions:   make(map[string][]*PageVersion),
		configPath: configPath,
	}

	// Load existing config
	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to load wiki config: %v", err)
	}

	return m
}

// CreateSpace creates a new Wiki space.
func (m *Manager) CreateSpace(req CreateSpaceRequest, ownerID string) (*Space, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	spaceID := uuid.New().String()
	now := time.Now()

	space := &Space{
		ID:          spaceID,
		Name:        req.Name,
		Description: req.Description,
		Icon:        req.Icon,
		IsPublic:    req.IsPublic,
		OwnerID:     ownerID,
		Members: []*Member{
			{
				UserID:   ownerID,
				Role:     "owner",
				JoinedAt: now,
			},
		},
		PageCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.spaces[spaceID] = space

	log.Printf("Created wiki space: %s (%s)", space.Name, spaceID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return space, nil
}

// GetSpace returns a space by ID.
func (m *Manager) GetSpace(spaceID string) (*Space, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	space, exists := m.spaces[spaceID]
	if !exists {
		return nil, fmt.Errorf("space %s not found", spaceID)
	}

	return space, nil
}

// ListSpaces returns all spaces.
func (m *Manager) ListSpaces() []*Space {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spaces := make([]*Space, 0, len(m.spaces))
	for _, space := range m.spaces {
		spaces = append(spaces, space)
	}

	// Sort by creation time
	sort.Slice(spaces, func(i, j int) bool {
		return spaces[i].CreatedAt.After(spaces[j].CreatedAt)
	})

	return spaces
}

// UpdateSpace updates a space.
func (m *Manager) UpdateSpace(spaceID string, req UpdateSpaceRequest) (*Space, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	space, exists := m.spaces[spaceID]
	if !exists {
		return nil, fmt.Errorf("space %s not found", spaceID)
	}

	if req.Name != nil {
		space.Name = *req.Name
	}
	if req.Description != nil {
		space.Description = *req.Description
	}
	if req.Icon != nil {
		space.Icon = *req.Icon
	}
	if req.IsPublic != nil {
		space.IsPublic = *req.IsPublic
	}

	space.UpdatedAt = time.Now()

	log.Printf("Updated wiki space: %s", spaceID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return space, nil
}

// DeleteSpace deletes a space and all its pages.
func (m *Manager) DeleteSpace(spaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.spaces[spaceID]; !exists {
		return fmt.Errorf("space %s not found", spaceID)
	}

	// Delete all pages in the space
	for pageID, page := range m.pages {
		if page.SpaceID == spaceID {
			delete(m.pages, pageID)
			delete(m.versions, pageID)
		}
	}

	delete(m.spaces, spaceID)

	log.Printf("Deleted wiki space: %s", spaceID)

	return m.saveConfig()
}

// CreatePage creates a new Wiki page.
func (m *Manager) CreatePage(req CreatePageRequest, authorID, authorName string) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	space, exists := m.spaces[req.SpaceID]
	if !exists {
		return nil, fmt.Errorf("space %s not found", req.SpaceID)
	}

	// Validate parent page if specified
	if req.ParentID != "" {
		parent, exists := m.pages[req.ParentID]
		if !exists {
			return nil, fmt.Errorf("parent page %s not found", req.ParentID)
		}
		if parent.SpaceID != req.SpaceID {
			return nil, fmt.Errorf("parent page %s is not in space %s", req.ParentID, req.SpaceID)
		}
	}

	pageID := uuid.New().String()
	now := time.Now()

	// Generate path
	path := generatePath(req.Title)
	if req.ParentID != "" {
		parent := m.pages[req.ParentID]
		path = parent.Path + "/" + path
	}

	// Set default status
	status := req.Status
	if status == "" {
		status = "draft"
	}

	page := &Page{
		ID:          pageID,
		SpaceID:     req.SpaceID,
		Title:       req.Title,
		Content:     req.Content,
		HTMLContent: renderMarkdown(req.Content),
		ParentID:    req.ParentID,
		Path:        path,
		Tags:        req.Tags,
		AuthorID:    authorID,
		AuthorName:  authorName,
		Status:      status,
		IsFavorite:  false,
		ViewCount:   0,
		Version:     1,
		Comments:    make([]*Comment, 0),
		Children:    make([]*Page, 0),
		CreatedAt:   now,
		UpdatedAt:   now,
		UpdatedBy:   authorID,
	}

	m.pages[pageID] = page
	space.PageCount++
	space.UpdatedAt = now

	// Create initial version
	m.versions[pageID] = []*PageVersion{
		{
			ID:         uuid.New().String(),
			PageID:     pageID,
			Version:    1,
			Title:      req.Title,
			Content:    req.Content,
			AuthorID:   authorID,
			AuthorName: authorName,
			Comment:    "初始版本",
			CreatedAt:  now,
		},
	}

	log.Printf("Created wiki page: %s in space %s", page.Title, req.SpaceID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return page, nil
}

// GetPage returns a page by ID.
func (m *Manager) GetPage(pageID string) (*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	page, exists := m.pages[pageID]
	if !exists {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	// Increment view count
	page.ViewCount++

	return page, nil
}

// ListPages returns all pages in a space, optionally with tree structure.
func (m *Manager) ListPages(spaceID string, tree bool) ([]*Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.spaces[spaceID]; !exists {
		return nil, fmt.Errorf("space %s not found", spaceID)
	}

	pages := make([]*Page, 0)
	for _, page := range m.pages {
		if page.SpaceID == spaceID {
			pages = append(pages, page)
		}
	}

	if tree {
		// Build tree structure
		return buildPageTree(pages), nil
	}

	// Sort by creation time
	sort.Slice(pages, func(i, j int) bool {
		return pages[i].CreatedAt.After(pages[j].CreatedAt)
	})

	return pages, nil
}

// UpdatePage updates a page.
func (m *Manager) UpdatePage(pageID string, req UpdatePageRequest, editorID, editorName string) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, exists := m.pages[pageID]
	if !exists {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	// Save current version to history
	if req.Content != nil && *req.Content != page.Content {
		// Create new version
		page.Version++
		version := &PageVersion{
			ID:         uuid.New().String(),
			PageID:     pageID,
			Version:    page.Version,
			Title:      page.Title,
			Content:    *req.Content,
			AuthorID:   editorID,
			AuthorName: editorName,
			Comment:    req.Comment,
			CreatedAt:  time.Now(),
		}
		m.versions[pageID] = append(m.versions[pageID], version)
	}

	if req.Title != nil {
		page.Title = *req.Title
	}
	if req.Content != nil {
		page.Content = *req.Content
		page.HTMLContent = renderMarkdown(*req.Content)
	}
	if req.Tags != nil {
		page.Tags = req.Tags
	}
	if req.Status != nil {
		page.Status = *req.Status
	}

	page.UpdatedAt = time.Now()
	page.UpdatedBy = editorID

	log.Printf("Updated wiki page: %s (version %d)", pageID, page.Version)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return page, nil
}

// DeletePage deletes a page.
func (m *Manager) DeletePage(pageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, exists := m.pages[pageID]
	if !exists {
		return fmt.Errorf("page %s not found", pageID)
	}

	// Move children to parent
	for _, p := range m.pages {
		if p.ParentID == pageID {
			p.ParentID = page.ParentID
		}
	}

	// Update space page count
	if space, exists := m.spaces[page.SpaceID]; exists {
		space.PageCount--
	}

	delete(m.pages, pageID)
	delete(m.versions, pageID)

	log.Printf("Deleted wiki page: %s", pageID)

	return m.saveConfig()
}

// GetPageVersions returns version history of a page.
func (m *Manager) GetPageVersions(pageID string) ([]*PageVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.pages[pageID]; !exists {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	versions := m.versions[pageID]
	if versions == nil {
		versions = make([]*PageVersion, 0)
	}

	return versions, nil
}

// RollbackPage rolls back a page to a specific version.
func (m *Manager) RollbackPage(pageID string, version int, editorID, editorName string) (*Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, exists := m.pages[pageID]
	if !exists {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	versions := m.versions[pageID]
	var targetVersion *PageVersion
	for _, v := range versions {
		if v.Version == version {
			targetVersion = v
			break
		}
	}

	if targetVersion == nil {
		return nil, fmt.Errorf("version %d not found for page %s", version, pageID)
	}

	// Save current state as new version before rollback
	page.Version++
	rollbackVersion := &PageVersion{
		ID:         uuid.New().String(),
		PageID:     pageID,
		Version:    page.Version,
		Title:      page.Title,
		Content:    page.Content,
		AuthorID:   editorID,
		AuthorName: editorName,
		Comment:    fmt.Sprintf("回滚到版本 %d", version),
		CreatedAt:  time.Now(),
	}
	m.versions[pageID] = append(m.versions[pageID], rollbackVersion)

	// Apply rollback
	page.Title = targetVersion.Title
	page.Content = targetVersion.Content
	page.HTMLContent = renderMarkdown(targetVersion.Content)
	page.UpdatedAt = time.Now()
	page.UpdatedBy = editorID

	log.Printf("Rolled back page %s to version %d", pageID, version)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return page, nil
}

// SearchPages searches pages by query.
func (m *Manager) SearchPages(query string, spaceID string, limit, offset int) []*SearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*SearchResult, 0)
	queryLower := strings.ToLower(query)

	for _, page := range m.pages {
		// Filter by space if specified
		if spaceID != "" && page.SpaceID != spaceID {
			continue
		}

		// Only search published pages
		if page.Status != "published" {
			continue
		}

		// Calculate relevance score
		score := 0.0
		titleLower := strings.ToLower(page.Title)
		contentLower := strings.ToLower(page.Content)

		// Title match (higher weight)
		if strings.Contains(titleLower, queryLower) {
			score += 10.0
		}

		// Content match
		if strings.Contains(contentLower, queryLower) {
			score += 5.0
		}

		// Tag match
		for _, tag := range page.Tags {
			if strings.Contains(strings.ToLower(tag), queryLower) {
				score += 3.0
			}
		}

		if score > 0 {
			// Generate highlighted content
			highlighted := highlightContent(page.Content, query, 200)

			results = append(results, &SearchResult{
				PageID:      page.ID,
				SpaceID:     page.SpaceID,
				Title:       page.Title,
				Content:     getContentSummary(page.Content, 200),
				Path:        page.Path,
				Score:       score,
				Highlighted: highlighted,
				UpdatedAt:   page.UpdatedAt,
			})
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply pagination
	if offset >= len(results) {
		return make([]*SearchResult, 0)
	}

	end := offset + limit
	if limit <= 0 || end > len(results) {
		end = len(results)
	}

	return results[offset:end]
}

// AddComment adds a comment to a page.
func (m *Manager) AddComment(pageID, userID, username, content, parentID string) (*Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	page, exists := m.pages[pageID]
	if !exists {
		return nil, fmt.Errorf("page %s not found", pageID)
	}

	now := time.Now()
	comment := &Comment{
		ID:        uuid.New().String(),
		PageID:    pageID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	page.Comments = append(page.Comments, comment)
	page.UpdatedAt = now

	log.Printf("Added comment to page %s", pageID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save wiki config: %v", err)
	}

	return comment, nil
}

// ExportPages exports pages to markdown format.
func (m *Manager) ExportPages(pageIDs []string, spaceID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	pages := make([]*Page, 0)

	if spaceID != "" {
		// Export entire space
		for _, page := range m.pages {
			if page.SpaceID == spaceID {
				pages = append(pages, page)
			}
		}
	} else {
		// Export specific pages
		for _, pageID := range pageIDs {
			if page, exists := m.pages[pageID]; exists {
				pages = append(pages, page)
			}
		}
	}

	for i, page := range pages {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("# %s\n\n", page.Title))
		sb.WriteString(page.Content)
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// buildPageTree builds a tree structure from flat page list.
func buildPageTree(pages []*Page) []*Page {
	pageMap := make(map[string]*Page)
	for _, page := range pages {
		page.Children = make([]*Page, 0)
		pageMap[page.ID] = page
	}

 roots := make([]*Page, 0)
	for _, page := range pages {
		if page.ParentID == "" {
			roots = append(roots, page)
		} else if parent, exists := pageMap[page.ParentID]; exists {
			parent.Children = append(parent.Children, page)
		} else {
			roots = append(roots, page)
		}
	}

	// Sort children by creation time
	var sortChildren func(page *Page)
	sortChildren = func(page *Page) {
		sort.Slice(page.Children, func(i, j int) bool {
			return page.Children[i].CreatedAt.After(page.Children[j].CreatedAt)
		})
		for _, child := range page.Children {
			sortChildren(child)
		}
	}

	for _, root := range roots {
		sortChildren(root)
	}

	sort.Slice(roots, func(i, j int) bool {
		return roots[i].CreatedAt.After(roots[j].CreatedAt)
	})

	return roots
}

// generatePath generates a URL-friendly path from title.
func generatePath(title string) string {
	// Convert to lowercase and replace spaces with hyphens
	path := strings.ToLower(title)
	path = strings.ReplaceAll(path, " ", "-")

	// Remove non-alphanumeric characters (keep Chinese characters and hyphens)
	var result strings.Builder
	for _, r := range path {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// renderMarkdown converts markdown to HTML (simplified).
func renderMarkdown(content string) string {
	// Simple markdown rendering (can be replaced with a proper library)
	html := content

	// Headers
	html = regexp.MustCompile(`^### (.+)$`).ReplaceAllString(html, "<h3>$1</h3>")
	html = regexp.MustCompile(`^## (.+)$`).ReplaceAllString(html, "<h2>$1</h2>")
	html = regexp.MustCompile(`^# (.+)$`).ReplaceAllString(html, "<h1>$1</h1>")

	// Bold
	html = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(html, "<strong>$1</strong>")

	// Italic
	html = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(html, "<em>$1</em>")

	// Code blocks
	html = regexp.MustCompile("```[\\s\\S]*?```").ReplaceAllStringFunc(html, func(match string) string {
		code := strings.TrimPrefix(match, "```")
		code = strings.TrimSuffix(code, "```")
		code = strings.TrimSpace(code)
		return fmt.Sprintf("<pre><code>%s</code></pre>", code)
	})

	// Inline code
	html = regexp.MustCompile("`([^`]+)`").ReplaceAllString(html, "<code>$1</code>")

	// Links
	html = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(html, `<a href="$2">$1</a>`)

	// Paragraphs
	lines := strings.Split(html, "\n")
	var paragraphs []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "<h") && !strings.HasPrefix(line, "<pre") && !strings.HasPrefix(line, "<ul") && !strings.HasPrefix(line, "<ol") {
			line = "<p>" + line + "</p>"
		}
		paragraphs = append(paragraphs, line)
	}

	return strings.Join(paragraphs, "\n")
}

// getContentSummary returns a summary of the content.
func getContentSummary(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// highlightContent highlights the query in content.
func highlightContent(content, query string, contextLen int) string {
	idx := strings.Index(strings.ToLower(content), strings.ToLower(query))
	if idx == -1 {
		return getContentSummary(content, contextLen)
	}

	start := idx - contextLen/2
	if start < 0 {
		start = 0
	}

	end := idx + len(query) + contextLen/2
	if end > len(content) {
		end = len(content)
	}

	highlighted := content[start:end]
	highlighted = strings.ReplaceAll(highlighted, query, fmt.Sprintf("<mark>%s</mark>", query))

	return highlighted
}

// saveConfig saves configuration to disk.
func (m *Manager) saveConfig() error {
	cfg := struct {
		Spaces   map[string]*Space         `json:"spaces"`
		Pages    map[string]*Page          `json:"pages"`
		Versions map[string][]*PageVersion `json:"versions"`
	}{
		Spaces:   m.spaces,
		Pages:    m.pages,
		Versions: m.versions,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// loadConfig loads configuration from disk.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Spaces   map[string]*Space         `json:"spaces"`
		Pages    map[string]*Page          `json:"pages"`
		Versions map[string][]*PageVersion `json:"versions"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Spaces != nil {
		m.spaces = cfg.Spaces
	}
	if cfg.Pages != nil {
		m.pages = cfg.Pages
	}
	if cfg.Versions != nil {
		m.versions = cfg.Versions
	}

	return nil
}

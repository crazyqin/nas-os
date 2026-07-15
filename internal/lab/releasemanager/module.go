// Package releasemanager provides version release orchestration for NAS-OS.
// It supports creating structured releases, automatically generating changelogs
// from commit history conventions, building pre-release checklists, and rendering
// release notes from configurable templates.
package releasemanager

import (
	"fmt"
	"strings"
	"time"
)

// Release represents a single NAS-OS version release.
type Release struct {
	// Version is the semantic version string (e.g. "1.2.0").
	Version string
	// Codename is the optional release codename (e.g. "Aurora").
	Codename string
	// ReleaseDate is when the release is published or planned.
	ReleaseDate time.Time
	// Type classifies the release: "major", "minor", "patch", "rc", "beta".
	Type string
	// Status indicates the current state: "planned", "in_progress", "ready", "published".
	Status string
	// ChangelogEntries lists all changelog items for this release.
	ChangelogEntries []ChangelogEntry
	// Checklist is the pre-release verification checklist.
	Checklist ReleaseChecklist
	// Notes is the rendered release notes content.
	Notes ReleaseNote
}

// ChangelogEntry represents a single change in the changelog.
type ChangelogEntry struct {
	// Category classifies the change: "added", "changed", "fixed", "removed", "deprecated", "security".
	Category string
	// Title is a short summary of the change.
	Title string
	// Description provides the full change description.
	Description string
	// CommitHash links to the source commit (optional).
	CommitHash string
	// Author is the contributor who made the change (optional).
	Author string
	// IssueID references a tracking issue (optional).
	IssueID string
}

// ReleaseChecklist represents the pre-release verification checklist.
type ReleaseChecklist struct {
	// Version is the release this checklist belongs to.
	Version string
	// Items contains all checklist items to verify before publishing.
	Items []ChecklistItem
}

// ChecklistItem represents a single verification task in the release checklist.
type ChecklistItem struct {
	// Name is the short name of the checklist item.
	Name string
	// Description explains what needs to be verified.
	Description string
	// Category groups items: "testing", "build", "docs", "security", "infra".
	Category string
	// Required indicates whether this item must pass (vs. optional).
	Required bool
	// Completed is true once the item has been verified.
	Completed bool
	// Assignee is the person responsible for this item (optional).
	Assignee string
}

// ReleaseNote represents the rendered release notes for a release.
type ReleaseNote struct {
	// Version is the release version these notes describe.
	Version string
	// Title is the heading of the release notes.
	Title string
	// Body is the fully rendered markdown content.
	Body string
}

// CreateRelease initialises a new Release with the given metadata and an
// empty changelog and a default checklist template.
func CreateRelease(version, codename, releaseType string, date time.Time) *Release {
	r := &Release{
		Version:     version,
		Codename:    codename,
		ReleaseDate: date,
		Type:        releaseType,
		Status:      "planned",
	}
	r.Checklist = BuildChecklist(version)
	return r
}

// GenerateChangelog builds changelog entries from a list of raw entries and
// attaches them to the release. It deduplicates entries by Title and sorts
// them by category in conventional-changelog order.
func GenerateChangelog(release *Release, entries []ChangelogEntry) []ChangelogEntry {
	seen := make(map[string]bool)
	var deduped []ChangelogEntry
	for _, e := range entries {
		if seen[e.Title] {
			continue
		}
		seen[e.Title] = true
		deduped = append(deduped, e)
	}

	// Sort by category priority.
	categoryOrder := map[string]int{
		"security":   0,
		"removed":    1,
		"deprecated": 2,
		"added":      3,
		"changed":    4,
		"fixed":      5,
	}
	sortEntriesByCategory(deduped, categoryOrder)

	release.ChangelogEntries = deduped
	return deduped
}

// BuildChecklist creates a default pre-release verification checklist for the
// given version. The default template covers testing, build, documentation,
// security, and infrastructure checks.
func BuildChecklist(version string) ReleaseChecklist {
	defaultItems := []ChecklistItem{
		{Name: "unit_tests", Description: "All unit tests pass with >80% coverage", Category: "testing", Required: true},
		{Name: "integration_tests", Description: "Integration tests pass on all supported platforms", Category: "testing", Required: true},
		{Name: "e2e_tests", Description: "End-to-end smoke tests pass on staging environment", Category: "testing", Required: true},
		{Name: "build_artifact", Description: "Production build artifact created and signed", Category: "build", Required: true},
		{Name: "image_push", Description: "Container images pushed to registry with correct tags", Category: "build", Required: true},
		{Name: "changelog_review", Description: "Changelog reviewed and approved by product owner", Category: "docs", Required: true},
		{Name: "release_notes", Description: "Release notes written and reviewed", Category: "docs", Required: true},
		{Name: "security_scan", Description: "Security vulnerability scan completed with no critical findings", Category: "security", Required: true},
		{Name: "dependency_audit", Description: "Dependency audit completed, no known CVEs", Category: "security", Required: true},
		{Name: "infra_ready", Description: "Infrastructure updated and deployment scripts verified", Category: "infra", Required: true},
		{Name: "rollback_plan", Description: "Rollback plan documented and tested", Category: "infra", Required: true},
		{Name: "db_migrations", Description: "Database migrations tested forward and backward", Category: "infra", Required: false},
	}

	return ReleaseChecklist{
		Version: version,
		Items:   defaultItems,
	}
}

// RenderNotes generates a formatted ReleaseNote in markdown from the release's
// changelog entries and checklist status. The output follows the Keep a
// Changelog format.
func RenderNotes(release *Release) ReleaseNote {
	var body strings.Builder

	// Header.
	body.WriteString(fmt.Sprintf("# %s", release.Version))
	if release.Codename != "" {
		body.WriteString(fmt.Sprintf(" — %q", release.Codename))
	}
	body.WriteString("\n\n")

	if !release.ReleaseDate.IsZero() {
		body.WriteString(fmt.Sprintf("**Released:** %s\n\n", release.ReleaseDate.Format("2006-01-02")))
	}

	// Group changelog entries by category.
	categories := []string{"security", "removed", "deprecated", "added", "changed", "fixed"}
	categoryLabels := map[string]string{
		"security":   "Security",
		"removed":    "Removed",
		"deprecated": "Deprecated",
		"added":      "Added",
		"changed":    "Changed",
		"fixed":      "Fixed",
	}

	for _, cat := range categories {
		var items []ChangelogEntry
		for _, e := range release.ChangelogEntries {
			if e.Category == cat {
				items = append(items, e)
			}
		}
		if len(items) == 0 {
			continue
		}
		body.WriteString(fmt.Sprintf("## %s\n\n", categoryLabels[cat]))
		for _, item := range items {
			if item.Description != "" {
				body.WriteString(fmt.Sprintf("- %s — %s\n", item.Title, item.Description))
			} else {
				body.WriteString(fmt.Sprintf("- %s\n", item.Title))
			}
		}
		body.WriteString("\n")
	}

	// Checklist summary.
	completed := 0
	requiredTotal := 0
	requiredDone := 0
	for _, item := range release.Checklist.Items {
		if item.Completed {
			completed++
		}
		if item.Required {
			requiredTotal++
			if item.Completed {
				requiredDone++
			}
		}
	}
	body.WriteString("## Checklist\n\n")
	body.WriteString(fmt.Sprintf("- Overall: %d/%d items completed\n", completed, len(release.Checklist.Items)))
	body.WriteString(fmt.Sprintf("- Required: %d/%d completed\n", requiredDone, requiredTotal))
	body.WriteString("\n")

	return ReleaseNote{
		Version: release.Version,
		Title:   fmt.Sprintf("Release %s", release.Version),
		Body:    body.String(),
	}
}

// sortEntriesByCategory sorts changelog entries in place by the given category
// priority order. Entries with unknown categories are placed at the end.
func sortEntriesByCategory(entries []ChangelogEntry, order map[string]int) {
	// Simple insertion sort – the number of entries is small.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0; j-- {
			oi, oki := order[entries[j].Category]
			if !oki {
				oi = len(order)
			}
			oj, okj := order[entries[j-1].Category]
			if !okj {
				oj = len(order)
			}
			if oi < oj {
				entries[j], entries[j-1] = entries[j-1], entries[j]
			} else {
				break
			}
		}
	}
}
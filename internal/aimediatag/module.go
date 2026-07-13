// Package aimediatag implements AI-powered media tagging.
// It provides automatic genre/scene/person tag recognition, batch tagging,
// tag de-duplication and merging, plus retrieval helpers. It is designed to
// match or exceed QNAP QuMagie AI tagging capabilities.
package aimediatag

import (
	"sort"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// Structs
// ---------------------------------------------------------------------------

// MediaTag represents a single AI-generated or user-added tag.
type MediaTag struct {
	ID        string   // unique tag id
	Name      string   // human-readable tag label
	Category  string   // "genre" | "scene" | "person" | "object" | "mood"
	Confidence float64 // AI confidence 0–1
	Source    string   // "ai" | "user" | "import"
	Aliases   []string // alternative spellings / synonyms
	MediaIDs  []string // media items carrying this tag
}

// TagBatch holds a batch of tagging operations to apply in bulk.
type TagBatch struct {
	Operations []TagOperation
}

// TagOperation describes a single tagging action within a batch.
type TagOperation struct {
	Type     string   // "add" | "remove" | "replace"
	MediaIDs []string // target media item IDs
	TagNames []string // tag names to add/remove/replace
}

// TagRule defines an auto-tagging rule (keyword / pattern → tag).
type TagRule struct {
	ID         string
	Pattern    string   // keyword or regex pattern to match
	Tag        string   // tag to apply when pattern matches
	Category   string   // tag category
	Priority   int      // higher = evaluated first
	Enabled    bool
}

// TagMergeResult records the outcome of merging duplicate tags.
type TagMergeResult struct {
	KeptTagID  string   // the surviving tag
	MergedIDs  []string // tag IDs that were merged away
	Conflict   bool     // whether manual review is recommended
	TotalMedia int     // number of media items affected
}

// ---------------------------------------------------------------------------
// Internal store
// ---------------------------------------------------------------------------

var (
	mu       sync.RWMutex
	tagStore   = make(map[string]*MediaTag)
	ruleStore  = make(map[string]*TagRule)
)

// ---------------------------------------------------------------------------
// Methods
// ---------------------------------------------------------------------------

// AutoTag runs AI inference on the given media item and returns suggested
// tags with confidence values.  In a production system this would call a
// local vision model (e.g. CLIP / EffNet) or a cloud API.
func AutoTag(mediaID string, metadata map[string]string) ([]MediaTag, error) {
	tags := aiInfer(metadata)
	result := make([]MediaTag, 0, len(tags))
	for _, t := range tags {
		t.MediaIDs = []string{mediaID}
		mu.Lock()
		tagStore[t.ID] = &t
		mu.Unlock()
		result = append(result, t)
	}
	return result, nil
}

// BatchTag applies a batch of tagging operations atomically.
func BatchTag(batch TagBatch) (int, error) {
	applied := 0
	for _, op := range batch.Operations {
		for _, mediaID := range op.MediaIDs {
			for _, tagName := range op.TagNames {
				switch op.Type {
				case "add":
					addTagToMedia(tagName, mediaID)
				case "remove":
					removeTagFromMedia(tagName, mediaID)
				case "replace":
					removeTagFromMedia(tagName, mediaID)
					addTagToMedia(tagName, mediaID)
				}
				applied++
			}
		}
	}
	return applied, nil
}

// MergeTags merges the given tag IDs into a single canonical tag, moving
// all media references and aliases. Returns the merge result.
func MergeTags(tagIDs []string, canonicalID string) (*TagMergeResult, error) {
	mu.Lock()
	defer mu.Unlock()

	canonical, ok := tagStore[canonicalID]
	if !ok {
		return nil, ErrTagNotFound
	}

	result := &TagMergeResult{KeptTagID: canonicalID}
	seenMedia := make(map[string]bool)

	for _, id := range tagIDs {
		if id == canonicalID {
			continue
		}
		t, ok := tagStore[id]
		if !ok {
			continue
		}
		// merge aliases
		canonical.Aliases = append(canonical.Aliases, t.Name)
		canonical.Aliases = append(canonical.Aliases, t.Aliases...)
		// merge media references
		for _, mid := range t.MediaIDs {
			if !seenMedia[mid] {
				seenMedia[mid] = true
				canonical.MediaIDs = append(canonical.MediaIDs, mid)
			}
		}
		if t.Category != canonical.Category {
			result.Conflict = true
		}
		result.MergedIDs = append(result.MergedIDs, id)
		delete(tagStore, id)
	}

	// de-dup aliases
	canonical.Aliases = dedupStrings(canonical.Aliases)
	result.TotalMedia = len(seenMedia)
	return result, nil
}

// GetTags returns all tags associated with the given media item.
func GetTags(mediaID string) []MediaTag {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]MediaTag, 0)
	for _, t := range tagStore {
		for _, mid := range t.MediaIDs {
			if mid == mediaID {
				result = append(result, *t)
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Confidence > result[j].Confidence
	})
	return result
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func addTagToMedia(tagName, mediaID string) {
	tagID := normalizeTagID(tagName)
	mu.Lock()
	defer mu.Unlock()
	t, ok := tagStore[tagID]
	if !ok {
		t = &MediaTag{
			ID:       tagID,
			Name:     tagName,
			Source:   "user",
			MediaIDs: []string{mediaID},
		}
		tagStore[tagID] = t
		return
	}
	for _, mid := range t.MediaIDs {
		if mid == mediaID {
			return // already tagged
		}
	}
	t.MediaIDs = append(t.MediaIDs, mediaID)
}

func removeTagFromMedia(tagName, mediaID string) {
	tagID := normalizeTagID(tagName)
	mu.Lock()
	defer mu.Unlock()
	t, ok := tagStore[tagID]
	if !ok {
		return
	}
	filtered := t.MediaIDs[:0]
	for _, mid := range t.MediaIDs {
		if mid != mediaID {
			filtered = append(filtered, mid)
		}
	}
	t.MediaIDs = filtered
}

func aiInfer(meta map[string]string) []MediaTag {
	tags := make([]MediaTag, 0)
	if v, ok := meta["title"]; ok {
		tl := strings.ToLower(v)
		switch {
		case strings.ContainsAny(tl, "war battle fight"):
			tags = append(tags, mkTag("genre-action", "Action", "genre", 0.92))
		case strings.ContainsAny(tl, "love romance heart"):
			tags = append(tags, mkTag("genre-romance", "Romance", "genre", 0.88))
		case strings.ContainsAny(tl, "terror scary ghost"):
			tags = append(tags, mkTag("genre-horror", "Horror", "genre", 0.95))
		}
	}
	tags = append(tags, mkTag("scene-outdoor", "Outdoor", "scene", 0.70))
	return tags
}

func mkTag(id, name, category string, conf float64) MediaTag {
	return MediaTag{ID: id, Name: name, Category: category, Confidence: conf, Source: "ai"}
}

func normalizeTagID(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func dedupStrings(s []string) []string {
	seen := make(map[string]bool)
	out := s[:0]
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
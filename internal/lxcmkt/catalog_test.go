package lxcmkt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return NewManager(logger)
}

func TestNewManager(t *testing.T) {
	mgr := newTestManager(t)
	assert.NotNil(t, mgr)

	templates := mgr.GetAll()
	assert.Greater(t, len(templates), 0, "should have default templates")
}

func TestGetAll(t *testing.T) {
	mgr := newTestManager(t)
	templates := mgr.GetAll()

	// Should have our default templates
	assert.Contains(t, templateIDs(templates), "ubuntu-22.04")
	assert.Contains(t, templateIDs(templates), "debian-12")
	assert.Contains(t, templateIDs(templates), "alpine-3.19")
}

func TestGetByID(t *testing.T) {
	mgr := newTestManager(t)

	t.Run("existing template", func(t *testing.T) {
		tmpl, err := mgr.GetByID("ubuntu-22.04")
		assert.NoError(t, err)
		assert.Equal(t, "Ubuntu 22.04 LTS", tmpl.Name)
		assert.Equal(t, "ubuntu", tmpl.Distro)
	})

	t.Run("non-existing template", func(t *testing.T) {
		_, err := mgr.GetByID("nonexistent")
		assert.Error(t, err)
	})
}

func TestSearch(t *testing.T) {
	mgr := newTestManager(t)

	t.Run("search by distro", func(t *testing.T) {
		results := mgr.Search(SearchQuery{Distro: "ubuntu"})
		assert.Greater(t, results.Total, 0)
		for _, tmpl := range results.Templates {
			assert.Equal(t, "ubuntu", tmpl.Distro)
		}
	})

	t.Run("search by query", func(t *testing.T) {
		results := mgr.Search(SearchQuery{Query: "alpine"})
		assert.Greater(t, results.Total, 0)
		assert.Contains(t, results.Templates[0].Name, "Alpine")
	})

	t.Run("search by tag", func(t *testing.T) {
		results := mgr.Search(SearchQuery{Tags: []string{"lts"}})
		assert.Greater(t, results.Total, 0)
		for _, tmpl := range results.Templates {
			assert.Contains(t, tmpl.Tags, "lts")
		}
	})

	t.Run("search by arch", func(t *testing.T) {
		results := mgr.Search(SearchQuery{Arch: "amd64"})
		assert.Greater(t, results.Total, 0)
		for _, tmpl := range results.Templates {
			assert.Equal(t, "amd64", tmpl.Arch)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		results := mgr.Search(SearchQuery{Page: 1, PageSize: 2})
		assert.LessOrEqual(t, len(results.Templates), 2)
		assert.Equal(t, 1, results.Page)
	})
}

func TestRate(t *testing.T) {
	mgr := newTestManager(t)

	t.Run("valid rating", func(t *testing.T) {
		err := mgr.Rate("ubuntu-22.04", 5)
		assert.NoError(t, err)

		tmpl, _ := mgr.GetByID("ubuntu-22.04")
		assert.Equal(t, 1, tmpl.RatingCount)
		assert.Equal(t, 5.0, tmpl.Rating)
	})

	t.Run("multiple ratings", func(t *testing.T) {
		mgr.Rate("debian-12", 4)
		mgr.Rate("debian-12", 5)
		mgr.Rate("debian-12", 3)

		tmpl, _ := mgr.GetByID("debian-12")
		assert.Equal(t, 3, tmpl.RatingCount)
		assert.InDelta(t, 4.0, tmpl.Rating, 0.01)
	})

	t.Run("invalid score", func(t *testing.T) {
		err := mgr.Rate("ubuntu-22.04", 6)
		assert.Error(t, err)
	})

	t.Run("non-existing template", func(t *testing.T) {
		err := mgr.Rate("nonexistent", 3)
		assert.Error(t, err)
	})
}

func TestIncrementDownloads(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.IncrementDownloads("alpine-3.19")
	assert.NoError(t, err)

	tmpl, _ := mgr.GetByID("alpine-3.19")
	assert.Equal(t, int64(1), tmpl.Downloads)

	// Increment again
	mgr.IncrementDownloads("alpine-3.19")
	tmpl, _ = mgr.GetByID("alpine-3.19")
	assert.Equal(t, int64(2), tmpl.Downloads)
}

func TestAddTemplate(t *testing.T) {
	mgr := newTestManager(t)

	newTmpl := &Template{
		ID:     "custom-1",
		Name:   "Custom Template",
		Distro: "ubuntu",
		Version: "22.04",
		Arch:   "amd64",
	}

	err := mgr.AddTemplate(newTmpl)
	assert.NoError(t, err)

	// Verify it exists
	tmpl, err := mgr.GetByID("custom-1")
	assert.NoError(t, err)
	assert.Equal(t, "Custom Template", tmpl.Name)
}

func TestAddDuplicateTemplate(t *testing.T) {
	mgr := newTestManager(t)

	newTmpl := &Template{
		ID:     "ubuntu-22.04", // already exists
		Name:   "Duplicate",
		Distro: "ubuntu",
	}

	err := mgr.AddTemplate(newTmpl)
	assert.Error(t, err)
}

func TestUpdateTemplate(t *testing.T) {
	mgr := newTestManager(t)

	tmpl, _ := mgr.GetByID("alpine-3.19")
	tmpl.Description = "Updated description"

	err := mgr.UpdateTemplate(tmpl)
	assert.NoError(t, err)

	updated, _ := mgr.GetByID("alpine-3.19")
	assert.Equal(t, "Updated description", updated.Description)
}

func TestDeleteTemplate(t *testing.T) {
	mgr := newTestManager(t)

	// Add a template to delete
	newTmpl := &Template{
		ID:     "to-delete",
		Name:   "To Delete",
		Distro: "ubuntu",
	}
	mgr.AddTemplate(newTmpl)

	// Delete it
	err := mgr.DeleteTemplate("to-delete")
	assert.NoError(t, err)

	// Verify it's gone
	_, err = mgr.GetByID("to-delete")
	assert.Error(t, err)
}

func TestDeleteNonExisting(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.DeleteTemplate("nonexistent")
	assert.Error(t, err)
}

func TestGetStats(t *testing.T) {
	mgr := newTestManager(t)

	// Add some downloads
	mgr.IncrementDownloads("ubuntu-22.04")
	mgr.IncrementDownloads("ubuntu-22.04")
	mgr.IncrementDownloads("alpine-3.19")

	stats := mgr.GetStats()
	assert.Greater(t, stats.TotalTemplates, 0)
	assert.Equal(t, int64(3), stats.TotalDownloads)
	assert.Greater(t, stats.TopDistro["ubuntu"], 0)
	assert.Greater(t, stats.TopDistro["alpine"], 0)
}

// helper function
func templateIDs(templates []Template) []string {
	ids := make([]string, len(templates))
	for i, t := range templates {
		ids[i] = t.ID
	}
	return ids
}

package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppDiscovery(t *testing.T) {
	t.Run("with valid directory", func(t *testing.T) {
		tempDir := t.TempDir()
		store, _ := NewAppStore(nil, tempDir)

		ad, err := NewAppDiscovery(store, tempDir)
		require.NoError(t, err)
		assert.NotNil(t, ad)
		assert.NotNil(t, ad.httpClient)
		assert.Equal(t, 24*time.Hour, ad.cacheExpiry)
	})

	t.Run("with empty directory", func(t *testing.T) {
		store, _ := NewAppStore(nil, t.TempDir())

		ad, err := NewAppDiscovery(store, "")
		require.NoError(t, err)
		assert.NotNil(t, ad)
	})
}

func TestAppDiscovery_InferCategory(t *testing.T) {
	ad := &AppDiscovery{}

	tests := []struct {
		name        string
		topics      []string
		description string
		appName     string
		expected    string
	}{
		{
			name:        "media from topics",
			topics:      []string{"plex", "media-server"},
			description: "",
			appName:     "",
			expected:    "Media",
		},
		{
			name:        "productivity from description",
			topics:      []string{},
			description: "A cloud storage solution",
			appName:     "nextcloud",
			expected:    "Productivity",
		},
		{
			name:        "smart home from name",
			topics:      []string{},
			description: "",
			appName:     "homeassistant",
			expected:    "Smart Home",
		},
		{
			name:        "network from topics",
			topics:      []string{"nginx", "proxy"},
			description: "",
			appName:     "",
			expected:    "Network",
		},
		{
			name:        "download from name",
			topics:      []string{},
			description: "",
			appName:     "transmission",
			expected:    "Download",
		},
		{
			name:        "development from topics",
			topics:      []string{"git", "gitea"},
			description: "",
			appName:     "",
			expected:    "Development",
		},
		{
			name:        "security from description",
			topics:      []string{},
			description: "Password manager vault",
			appName:     "",
			expected:    "Security",
		},
		{
			name:        "database from topics",
			topics:      []string{"mysql", "database"},
			description: "",
			appName:     "",
			expected:    "Database",
		},
		{
			name:        "other category",
			topics:      []string{},
			description: "Some random app",
			appName:     "random-app",
			expected:    "Other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ad.inferCategory(tt.topics, tt.description, tt.appName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppDiscovery_ParseGitHubRepo(t *testing.T) {
	ad := &AppDiscovery{
		discovered: make(map[string]*DiscoveredApp),
	}

	tests := []struct {
		name     string
		repo     *GitHubRepo
		wantNil  bool
		checkKey string
	}{
		{
			name:    "nil repo",
			repo:    nil,
			wantNil: true,
		},
		{
			name: "valid repo",
			repo: &GitHubRepo{
				ID:          12345,
				Name:        "test-app",
				FullName:    "user/test-app",
				Description: "Test application",
				Stars:       1000,
				Topics:      []string{"docker", "media"},
				HTMLURL:     "https://github.com/user/test-app",
				UpdatedAt:   time.Now(),
			},
			wantNil:  false,
			checkKey: "gh-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ad.parseGitHubRepo(tt.repo)
			if tt.wantNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.checkKey, result.ID)
				assert.Equal(t, "github", result.Source)
			}
		})
	}
}

func TestAppDiscovery_GetDiscoveredApps(t *testing.T) {
	ad := &AppDiscovery{
		discovered: map[string]*DiscoveredApp{
			"app1": {ID: "app1", Name: "app1", Source: "github", Category: "Media", Stars: 100},
			"app2": {ID: "app2", Name: "app2", Source: "dockerhub", Category: "Network", Stars: 200},
			"app3": {ID: "app3", Name: "app3", Source: "github", Category: "Media", Stars: 50},
		},
	}

	t.Run("no filters", func(t *testing.T) {
		result := ad.GetDiscoveredApps("", "", 0)
		assert.Len(t, result, 3)
	})

	t.Run("filter by source", func(t *testing.T) {
		result := ad.GetDiscoveredApps("github", "", 0)
		assert.Len(t, result, 2)
	})

	t.Run("filter by category", func(t *testing.T) {
		result := ad.GetDiscoveredApps("", "Media", 0)
		assert.Len(t, result, 2)
	})

	t.Run("filter by both", func(t *testing.T) {
		result := ad.GetDiscoveredApps("github", "Media", 0)
		assert.Len(t, result, 2)
	})

	t.Run("with limit", func(t *testing.T) {
		result := ad.GetDiscoveredApps("", "", 2)
		assert.Len(t, result, 2)
		// Should be sorted by stars (highest first)
		assert.Equal(t, 200, result[0].Stars)
	})
}

func TestAppDiscovery_SortDiscoveredByStars(t *testing.T) {
	apps := []*DiscoveredApp{
		{ID: "1", Stars: 100},
		{ID: "2", Stars: 500},
		{ID: "3", Stars: 50},
		{ID: "4", Stars: 1000},
	}

	sortDiscoveredByStars(apps)

	assert.Equal(t, 1000, apps[0].Stars)
	assert.Equal(t, 500, apps[1].Stars)
	assert.Equal(t, 100, apps[2].Stars)
	assert.Equal(t, 50, apps[3].Stars)
}

func TestAppDiscovery_IsCacheValid(t *testing.T) {
	ad := &AppDiscovery{
		cacheExpiry: 1 * time.Hour,
	}

	t.Run("fresh cache", func(t *testing.T) {
		ad.lastUpdate = time.Now()
		assert.True(t, ad.IsCacheValid())
	})

	t.Run("expired cache", func(t *testing.T) {
		ad.lastUpdate = time.Now().Add(-2 * time.Hour)
		assert.False(t, ad.IsCacheValid())
	})
}

func TestAppDiscovery_GetLastUpdateTime(t *testing.T) {
	now := time.Now()
	ad := &AppDiscovery{
		lastUpdate: now,
	}

	result := ad.GetLastUpdateTime()
	assert.Equal(t, now, result)
}

func TestGitHubRepo_Struct(t *testing.T) {
	repo := GitHubRepo{
		ID:          12345,
		Name:        "test-repo",
		FullName:    "owner/test-repo",
		Description: "Test repository",
		Stars:       1000,
		Forks:       100,
		Language:    "Go",
		Topics:      []string{"docker", "backup"},
		HTMLURL:     "https://github.com/owner/test-repo",
		CloneURL:    "https://github.com/owner/test-repo.git",
	}

	assert.Equal(t, int64(12345), repo.ID)
	assert.Equal(t, "test-repo", repo.Name)
	assert.Equal(t, 1000, repo.Stars)
}

func TestDockerHubImage_Struct(t *testing.T) {
	img := DockerHubImage{
		Name:        "nginx",
		Namespace:   "library",
		Description: "Official nginx image",
		Stars:       10000,
		Official:    true,
	}

	assert.Equal(t, "nginx", img.Name)
	assert.True(t, img.Official)
}

func TestDiscoveredApp_Struct(t *testing.T) {
	now := time.Now()
	app := DiscoveredApp{
		ID:          "gh-123",
		Name:        "test-app",
		DisplayName: "Test App",
		Description: "A test application",
		Source:      "github",
		Stars:       500,
		Category:    "Media",
		Image:       "test/app:latest",
		GitHubURL:   "https://github.com/test/app",
		HasCompose:  true,
		UpdatedAt:   now,
	}

	assert.Equal(t, "gh-123", app.ID)
	assert.Equal(t, "github", app.Source)
	assert.True(t, app.HasCompose)
}

func TestAppDiscovery_SaveCache(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewAppStore(nil, tempDir)

	ad, err := NewAppDiscovery(store, tempDir)
	require.NoError(t, err)

	ad.discovered["test"] = &DiscoveredApp{
		ID:   "test",
		Name: "test-app",
	}
	ad.lastUpdate = time.Now()

	err = ad.saveCache()
	require.NoError(t, err)
}

func TestAppDiscovery_LoadCache(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		ad := &AppDiscovery{
			cacheFile:  "/nonexistent/cache.json",
			discovered: make(map[string]*DiscoveredApp),
		}

		err := ad.loadCache()
		assert.Error(t, err)
	})
}

func TestAppDiscovery_RefreshDiscovery(t *testing.T) {
	tempDir := t.TempDir()
	store, _ := NewAppStore(nil, tempDir)

	ad, err := NewAppDiscovery(store, tempDir)
	require.NoError(t, err)

	// RefreshDiscovery will fail without network, but should not panic
	err = ad.RefreshDiscovery()
	// Just verify it doesn't crash
	_ = err
}
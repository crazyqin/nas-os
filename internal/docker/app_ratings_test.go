package docker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRatingManager(t *testing.T) {
	t.Run("with valid directory", func(t *testing.T) {
		tempDir := t.TempDir()
		rm, err := NewRatingManager(tempDir)
		require.NoError(t, err)
		assert.NotNil(t, rm)
		assert.NotNil(t, rm.ratings)
		assert.NotNil(t, rm.stats)
	})

	t.Run("with empty directory", func(t *testing.T) {
		rm, err := NewRatingManager("")
		require.NoError(t, err)
		assert.NotNil(t, rm)
	})
}

func TestRatingManager_AddRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating := &Rating{
		ID:         "rating-1",
		TemplateID: "template-1",
		UserID:     "user-1",
		Score:      5,
		Comment:    "Great app!",
		CreatedAt:  time.Now(),
		Helpful:    0,
	}

	err = rm.AddRating(rating)
	require.NoError(t, err)

	ratings := rm.GetRatings("template-1")
	assert.Len(t, ratings, 1)
	assert.Equal(t, 5, ratings[0].Score)
}

func TestRatingManager_AddRating_Duplicate(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating := &Rating{
		ID:         "rating-1",
		TemplateID: "template-1",
		UserID:     "user-1",
		Score:      5,
	}

	rm.AddRating(rating)

	// Same user tries to rate again
	rating2 := &Rating{
		ID:         "rating-2",
		TemplateID: "template-1",
		UserID:     "user-1", // Same user
		Score:      4,
	}

	err = rm.AddRating(rating2)
	assert.Error(t, err) // Should error for duplicate
}

func TestRatingManager_GetRatings(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// Add multiple ratings
	for i := 1; i <= 3; i++ {
		rm.AddRating(&Rating{
			ID:         "rating-" + string(rune('0'+i)),
			TemplateID: "template-1",
			UserID:     "user-" + string(rune('0'+i)),
			Score:      i + 2,
			CreatedAt:  time.Now(),
		})
	}

	ratings := rm.GetRatings("template-1")
	assert.Len(t, ratings, 3)
}

func TestRatingManager_GetRatings_Empty(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	ratings := rm.GetRatings("nonexistent")
	assert.Empty(t, ratings)
}

func TestRatingManager_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// Add ratings
	rm.AddRating(&Rating{ID: "r1", TemplateID: "t1", UserID: "u1", Score: 5})
	rm.AddRating(&Rating{ID: "r2", TemplateID: "t1", UserID: "u2", Score: 4})
	rm.AddRating(&Rating{ID: "r3", TemplateID: "t1", UserID: "u3", Score: 3})

	stats := rm.GetStats("t1")
	require.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalRatings)
	assert.Equal(t, 4.0, stats.AverageScore) // (5+4+3)/3 = 4
	assert.Equal(t, 5, stats.Score5)
	assert.Equal(t, 1, stats.Score4)
	assert.Equal(t, 1, stats.Score3)
}

func TestRatingManager_DeleteRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rm.AddRating(&Rating{
		ID:         "rating-1",
		TemplateID: "template-1",
		UserID:     "user-1",
		Score:      5,
	})

	err = rm.DeleteRating("template-1", "rating-1")
	require.NoError(t, err)

	ratings := rm.GetRatings("template-1")
	assert.Empty(t, ratings)
}

func TestRatingManager_DeleteRating_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	err = rm.DeleteRating("nonexistent", "nonexistent")
	assert.Error(t, err)
}

func TestRatingManager_MarkHelpful(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rm.AddRating(&Rating{
		ID:         "rating-1",
		TemplateID: "template-1",
		UserID:     "user-1",
		Score:      5,
	})

	err = rm.MarkHelpful("template-1", "rating-1")
	require.NoError(t, err)

	ratings := rm.GetRatings("template-1")
	assert.Equal(t, 1, ratings[0].Helpful)
}

func TestRatingManager_VerifyPurchase(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// This is a placeholder - actual implementation may vary
	result := rm.VerifyPurchase("user-1", "template-1")
	// Default behavior - adjust based on actual implementation
	_ = result
}

func TestRatingManager_GetUserRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rm.AddRating(&Rating{
		ID:         "rating-1",
		TemplateID: "template-1",
		UserID:     "user-1",
		Score:      5,
		Comment:    "Great!",
	})

	rating, err := rm.GetUserRating("template-1", "user-1")
	require.NoError(t, err)
	assert.Equal(t, 5, rating.Score)
	assert.Equal(t, "Great!", rating.Comment)
}

func TestRatingManager_GetUserRating_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	_, err = rm.GetUserRating("nonexistent", "nonexistent")
	assert.Error(t, err)
}

func TestRating_CalculateStats(t *testing.T) {
	rm := &RatingManager{
		ratings: make(map[string][]*Rating),
		stats:   make(map[string]*RatingStats),
	}

	// Add ratings manually
	rm.ratings["t1"] = []*Rating{
		{Score: 5},
		{Score: 4},
		{Score: 4},
		{Score: 3},
	}

	rm.calculateStats("t1")

	stats := rm.stats["t1"]
	require.NotNil(t, stats)
	assert.Equal(t, 4, stats.TotalRatings)
	assert.InDelta(t, 4.0, stats.AverageScore, 0.01)
	assert.Equal(t, 1, stats.Score5)
	assert.Equal(t, 2, stats.Score4)
	assert.Equal(t, 1, stats.Score3)
}

func TestRating_Struct(t *testing.T) {
	now := time.Now()
	rating := Rating{
		ID:         "rating-123",
		TemplateID: "template-456",
		UserID:     "user-789",
		Score:      5,
		Comment:    "Excellent application!",
		CreatedAt:  now,
		Helpful:    10,
		Verified:   true,
	}

	assert.Equal(t, "rating-123", rating.ID)
	assert.Equal(t, 5, rating.Score)
	assert.True(t, rating.Verified)
	assert.Equal(t, 10, rating.Helpful)
}

func TestRatingStats_Struct(t *testing.T) {
	stats := RatingStats{
		TemplateID:      "template-1",
		TotalRatings:    100,
		AverageScore:    4.5,
		Score5:          50,
		Score4:          30,
		Score3:          15,
		Score2:          4,
		Score1:          1,
		Distribution:    map[int]int{5: 50, 4: 30, 3: 15, 2: 4, 1: 1},
		LastUpdated:     time.Now(),
		TotalHelpful:    200,
		AverageHelpful:  2.0,
	}

	assert.Equal(t, 100, stats.TotalRatings)
	assert.Equal(t, 4.5, stats.AverageScore)
	assert.Equal(t, 50, stats.Score5)
}

func TestRatingManager_Save(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rm.AddRating(&Rating{
		ID:         "r1",
		TemplateID: "t1",
		UserID:     "u1",
		Score:      5,
	})

	err = rm.save()
	require.NoError(t, err)
}

func TestRatingManager_Load(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		rm := &RatingManager{
			dataFile: "/nonexistent/ratings.json",
			ratings:  make(map[string][]*Rating),
			stats:    make(map[string]*RatingStats),
		}

		err := rm.load()
		// Should handle gracefully
		_ = err
	})
}
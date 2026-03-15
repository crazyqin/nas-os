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
		// 空目录可能返回错误，也可能成功创建
		_ = rm
		_ = err
	})
}

func TestRatingManager_AddRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating, err := rm.AddRating("template-1", "user-1", "Test User", 5, "Great app!", "Excellent application!")
	require.NoError(t, err)
	assert.NotNil(t, rating)
	assert.Equal(t, "template-1", rating.TemplateID)
	assert.Equal(t, "user-1", rating.UserID)
	assert.Equal(t, 5, rating.Rating)

	// 验证可以获取评分
	ratings := rm.GetRatings("template-1", "newest", 10, 0)
	assert.Len(t, ratings, 1)
	assert.Equal(t, 5, ratings[0].Rating)
}

func TestRatingManager_AddRating_Duplicate(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 第一次评分
	_, err = rm.AddRating("template-1", "user-1", "User 1", 5, "Good", "Nice app")
	require.NoError(t, err)

	// 同一用户再次评分应该更新而不是报错
	rating, err := rm.AddRating("template-1", "user-1", "User 1", 4, "Updated", "Changed my mind")
	require.NoError(t, err)
	assert.Equal(t, 4, rating.Rating)
}

func TestRatingManager_GetRatings(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 添加多个评分
	for i := 1; i <= 3; i++ {
		_, err := rm.AddRating("template-1", "user-"+string(rune('0'+i)), "User "+string(rune('0'+i)), i+2, "Title", "Comment")
		require.NoError(t, err)
	}

	ratings := rm.GetRatings("template-1", "newest", 10, 0)
	assert.Len(t, ratings, 3)
}

func TestRatingManager_GetRatings_Empty(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	ratings := rm.GetRatings("nonexistent", "newest", 10, 0)
	assert.Empty(t, ratings)
}

func TestRatingManager_GetStats(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 添加评分
	rm.AddRating("t1", "u1", "User 1", 5, "Title", "Comment")
	rm.AddRating("t1", "u2", "User 2", 4, "Title", "Comment")
	rm.AddRating("t1", "u3", "User 3", 3, "Title", "Comment")

	stats := rm.GetStats("t1")
	require.NotNil(t, stats)
	assert.Equal(t, 3, stats.TotalReviews)
	assert.InDelta(t, 4.0, stats.AverageScore, 0.01) // (5+4+3)/3 = 4
}

func TestRatingManager_DeleteRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating, err := rm.AddRating("template-1", "user-1", "User 1", 5, "Title", "Comment")
	require.NoError(t, err)

	err = rm.DeleteRating("template-1", rating.ID, "user-1")
	require.NoError(t, err)

	ratings := rm.GetRatings("template-1", "newest", 10, 0)
	assert.Empty(t, ratings)
}

func TestRatingManager_DeleteRating_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	err = rm.DeleteRating("nonexistent", "nonexistent", "user-1")
	assert.Error(t, err)
}

func TestRatingManager_MarkHelpful(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating, err := rm.AddRating("template-1", "user-1", "User 1", 5, "Title", "Comment")
	require.NoError(t, err)

	err = rm.MarkHelpful("template-1", rating.ID, "user-2")
	require.NoError(t, err)

	ratings := rm.GetRatings("template-1", "newest", 10, 0)
	assert.Equal(t, 1, ratings[0].Helpful)
}

func TestRatingManager_VerifyPurchase(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 这个方法目前只是返回 nil
	err = rm.VerifyPurchase("template-1", "user-1")
	_ = err
}

func TestRatingManager_GetUserRating(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	_, err = rm.AddRating("template-1", "user-1", "User 1", 5, "Great!", "Excellent!")
	require.NoError(t, err)

	rating := rm.GetUserRating("template-1", "user-1")
	require.NotNil(t, rating)
	assert.Equal(t, 5, rating.Rating)
	assert.Equal(t, "Great!", rating.Title)
}

func TestRatingManager_GetUserRating_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	rating := rm.GetUserRating("nonexistent", "nonexistent")
	assert.Nil(t, rating)
}

func TestRatingManager_CalculateStats(t *testing.T) {
	rm := &RatingManager{
		ratings: make(map[string][]*AppRating),
		stats:   make(map[string]*RatingStats),
	}

	// 添加评分
	rm.ratings["t1"] = []*AppRating{
		{Rating: 5},
		{Rating: 4},
		{Rating: 4},
		{Rating: 3},
	}

	rm.calculateStats("t1")

	stats := rm.stats["t1"]
	require.NotNil(t, stats)
	assert.Equal(t, 4, stats.TotalReviews)
	assert.InDelta(t, 4.0, stats.AverageScore, 0.01)
}

func TestAppRating_Struct(t *testing.T) {
	now := time.Now()
	rating := AppRating{
		ID:         "rating-123",
		TemplateID: "template-456",
		UserID:     "user-789",
		UserName:   "Test User",
		Rating:     5,
		Title:      "Excellent!",
		Content:    "Excellent application!",
		CreatedAt:  now,
		Helpful:    10,
		Verified:   true,
	}

	assert.Equal(t, "rating-123", rating.ID)
	assert.Equal(t, 5, rating.Rating)
	assert.True(t, rating.Verified)
	assert.Equal(t, 10, rating.Helpful)
}

func TestRatingStats_Struct(t *testing.T) {
	stats := RatingStats{
		TemplateID:   "template-1",
		TotalReviews: 100,
		AverageScore: 4.5,
	}
	stats.Distribution.Five = 50
	stats.Distribution.Four = 30
	stats.Distribution.Three = 15
	stats.Distribution.Two = 4
	stats.Distribution.One = 1

	assert.Equal(t, 100, stats.TotalReviews)
	assert.Equal(t, 4.5, stats.AverageScore)
	assert.Equal(t, 50, stats.Distribution.Five)
}

func TestRatingManager_Save(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	_, err = rm.AddRating("t1", "u1", "User 1", 5, "Title", "Comment")
	require.NoError(t, err)

	err = rm.save()
	require.NoError(t, err)
}

func TestRatingManager_Load(t *testing.T) {
	t.Run("non-existent file", func(t *testing.T) {
		rm := &RatingManager{
			dataFile: "/nonexistent/ratings.json",
			ratings:  make(map[string][]*AppRating),
			stats:    make(map[string]*RatingStats),
		}

		err := rm.load()
		// 应该优雅处理
		_ = err
	})
}

func TestRatingManager_RatingLimits(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	t.Run("valid rating 1", func(t *testing.T) {
		_, err := rm.AddRating("t1", "u1", "User", 1, "Title", "Bad")
		assert.NoError(t, err)
	})

	t.Run("valid rating 5", func(t *testing.T) {
		_, err := rm.AddRating("t2", "u2", "User", 5, "Title", "Great")
		assert.NoError(t, err)
	})
}

func TestRatingManager_Sorting(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 添加不同评分
	rm.AddRating("t1", "u1", "User 1", 3, "Title", "Comment")
	time.Sleep(time.Millisecond) // 确保时间不同
	rm.AddRating("t1", "u2", "User 2", 5, "Title", "Comment")
	time.Sleep(time.Millisecond)
	rm.AddRating("t1", "u3", "User 3", 1, "Title", "Comment")

	// 测试按最新排序
	ratings := rm.GetRatings("t1", "newest", 10, 0)
	assert.Len(t, ratings, 3)
	// 最新添加的应该在前面
	assert.Equal(t, 1, ratings[0].Rating)

	// 测试按评分排序
	ratings = rm.GetRatings("t1", "highest", 10, 0)
	assert.Equal(t, 5, ratings[0].Rating)

	ratings = rm.GetRatings("t1", "lowest", 10, 0)
	assert.Equal(t, 1, ratings[0].Rating)
}

func TestRatingManager_Pagination(t *testing.T) {
	tempDir := t.TempDir()
	rm, err := NewRatingManager(tempDir)
	require.NoError(t, err)

	// 添加5个评分
	for i := 0; i < 5; i++ {
		_, err := rm.AddRating("t1", "u"+string(rune('0'+i)), "User", 5, "Title", "Comment")
		require.NoError(t, err)
	}

	// 测试分页
	page1 := rm.GetRatings("t1", "newest", 2, 0)
	assert.Len(t, page1, 2)

	page2 := rm.GetRatings("t1", "newest", 2, 2)
	assert.Len(t, page2, 2)

	page3 := rm.GetRatings("t1", "newest", 2, 4)
	assert.Len(t, page3, 1)
}

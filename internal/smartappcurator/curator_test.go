package smartappcurator

import (
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() 返回 nil")
	}
	if len(c.apps) == 0 {
		t.Error("默认应用列表为空")
	}
}

func TestRegisterApp(t *testing.T) {
	c := New()
	app := AppInfo{
		ID:          "test-app",
		Name:        "测试应用",
		Category:    CategoryMedia,
		Description: "测试用应用",
		Rating:      4.5,
		Downloads:   1000,
	}

	c.RegisterApp(app)

	got, err := c.GetApp("test-app")
	if err != nil {
		t.Fatalf("GetApp() 失败: %v", err)
	}
	if got.Name != "测试应用" {
		t.Errorf("应用名 = %s, 期望 测试应用", got.Name)
	}
}

func TestUpdateAndGetProfile(t *testing.T) {
	c := New()
	profile := UserProfile{
		UserID:        "user1",
		InstalledApps: []string{"photos", "drive"},
		Preferences: UserPreferences{
			Categories: []AppCategory{CategoryMedia, CategoryAI},
		},
	}

	c.UpdateProfile(profile)

	got, err := c.GetProfile("user1")
	if err != nil {
		t.Fatalf("GetProfile() 失败: %v", err)
	}
	if got.UserID != "user1" {
		t.Errorf("用户 ID = %s, 期望 user1", got.UserID)
	}
	if len(got.InstalledApps) != 2 {
		t.Errorf("已安装应用数 = %d, 期望 2", len(got.InstalledApps))
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	c := New()
	_, err := c.GetProfile("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestRecommend_WithProfile(t *testing.T) {
	c := New()
	profile := UserProfile{
		UserID:        "user1",
		InstalledApps: []string{"photos"},
		Preferences: UserPreferences{
			Categories: []AppCategory{CategoryAI},
		},
	}
	c.UpdateProfile(profile)

	result, err := c.Recommend(RecommendRequest{
		UserID: "user1",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("Recommend() 失败: %v", err)
	}
	if result.UserID != "user1" {
		t.Errorf("用户 ID = %s, 期望 user1", result.UserID)
	}
	if len(result.Recommendations) == 0 {
		t.Error("推荐列表为空")
	}
	if len(result.Recommendations) > 5 {
		t.Errorf("推荐数 = %d, 期望 <= 5", len(result.Recommendations))
	}

	// 已安装的应用不应出现在推荐中
	for _, rec := range result.Recommendations {
		if rec.App.ID == "photos" {
			t.Error("已安装应用不应被推荐")
		}
	}
}

func TestRecommend_WithoutProfile(t *testing.T) {
	c := New()

	result, err := c.Recommend(RecommendRequest{
		UserID: "new-user",
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("Recommend() 失败: %v", err)
	}
	if len(result.Recommendations) == 0 {
		t.Error("推荐列表为空")
	}
	if len(result.TrendingApps) == 0 {
		t.Error("热门应用列表为空")
	}
}

func TestRecommend_WithExclude(t *testing.T) {
	c := New()

	result, err := c.Recommend(RecommendRequest{
		UserID:  "user1",
		Limit:   10,
		Exclude: []string{"photos", "drive", "ai-assistant"},
	})
	if err != nil {
		t.Fatalf("Recommend() 失败: %v", err)
	}

	for _, rec := range result.Recommendations {
		for _, excluded := range []string{"photos", "drive", "ai-assistant"} {
			if rec.App.ID == excluded {
				t.Errorf("排除的应用 %s 出现在推荐中", excluded)
			}
		}
	}
}

func TestListApps(t *testing.T) {
	c := New()

	// 列出所有应用
	all := c.ListApps("")
	if len(all) == 0 {
		t.Error("应用列表为空")
	}

	// 按类别筛选
	media := c.ListApps(CategoryMedia)
	for _, app := range media {
		if app.Category != CategoryMedia {
			t.Errorf("应用类别 = %s, 期望 %s", app.Category, CategoryMedia)
		}
	}
}

func TestGetApp_NotFound(t *testing.T) {
	c := New()
	_, err := c.GetApp("nonexistent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

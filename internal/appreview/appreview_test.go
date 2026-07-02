package appreview

import (
	"strings"
	"testing"
	"time"
)

// ============================================================
// ReviewManager 测试
// ============================================================

func TestCreateReview(t *testing.T) {
	rm := NewReviewManager()
	r, err := rm.CreateReview("app1", "user1", "张三", "很好用", "体验不错，推荐", "1.0.0", 5)
	if err != nil {
		t.Fatalf("CreateReview 失败: %v", err)
	}
	if r.AppID != "app1" {
		t.Errorf("AppID = %q, 期望 %q", r.AppID, "app1")
	}
	if r.UserID != "user1" {
		t.Errorf("UserID = %q, 期望 %q", r.UserID, "user1")
	}
	if r.Rating != 5 {
		t.Errorf("Rating = %d, 期望 5", r.Rating)
	}
	if r.Content != "体验不错，推荐" {
		t.Errorf("Content = %q, 期望 %q", r.Content, "体验不错，推荐")
	}
	if r.ID == "" {
		t.Error("ID 不应为空")
	}
	if !r.Verified {
		t.Error("新评价应默认已验证")
	}
	if r.CreatedAt.IsZero() {
		t.Error("CreatedAt 不应为零值")
	}
}

func TestCreateReview_InvalidRating(t *testing.T) {
	rm := NewReviewManager()
	_, err := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 0)
	if err != ErrInvalidRating {
		t.Errorf("评分 0 应返回 ErrInvalidRating, 得到: %v", err)
	}
	_, err = rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 6)
	if err != ErrInvalidRating {
		t.Errorf("评分 6 应返回 ErrInvalidRating, 得到: %v", err)
	}
}

func TestCreateReview_EmptyFields(t *testing.T) {
	rm := NewReviewManager()
	_, err := rm.CreateReview("", "user1", "张三", "", "内容", "1.0", 5)
	if err != ErrEmptyAppID {
		t.Errorf("空 AppID 应返回 ErrEmptyAppID, 得到: %v", err)
	}
	_, err = rm.CreateReview("app1", "", "张三", "", "内容", "1.0", 5)
	if err != ErrEmptyUserID {
		t.Errorf("空 UserID 应返回 ErrEmptyUserID, 得到: %v", err)
	}
	_, err = rm.CreateReview("app1", "user1", "张三", "", "  ", "1.0", 5)
	if err != ErrEmptyContent {
		t.Errorf("空内容应返回 ErrEmptyContent, 得到: %v", err)
	}
}

func TestCreateReview_Duplicate(t *testing.T) {
	rm := NewReviewManager()
	_, _ = rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	_, err := rm.CreateReview("app1", "user1", "张三", "", "另一条", "1.0", 4)
	if err != ErrDuplicateReview {
		t.Errorf("重复评价应返回 ErrDuplicateReview, 得到: %v", err)
	}
}

func TestCreateReview_DifferentApps(t *testing.T) {
	rm := NewReviewManager()
	_, err := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}
	_, err = rm.CreateReview("app2", "user1", "张三", "", "内容", "1.0", 4)
	if err != nil {
		t.Fatalf("同一用户不同应用应允许创建: %v", err)
	}
}

func TestGetReview(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)

	got, err := rm.GetReview(r.ID)
	if err != nil {
		t.Fatalf("GetReview 失败: %v", err)
	}
	if got.ID != r.ID {
		t.Errorf("ID = %q, 期望 %q", got.ID, r.ID)
	}
}

func TestGetReview_NotFound(t *testing.T) {
	rm := NewReviewManager()
	_, err := rm.GetReview("nonexistent")
	if err != ErrReviewNotFound {
		t.Errorf("期望 ErrReviewNotFound, 得到: %v", err)
	}
}

func TestUpdateReview(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "原始内容", "1.0", 5)

	updated, err := rm.UpdateReview(r.ID, "user1", "修改后内容", 3)
	if err != nil {
		t.Fatalf("UpdateReview 失败: %v", err)
	}
	if updated.Content != "修改后内容" {
		t.Errorf("Content = %q, 期望 %q", updated.Content, "修改后内容")
	}
	if updated.Rating != 3 {
		t.Errorf("Rating = %d, 期望 3", updated.Rating)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) && !updated.UpdatedAt.Equal(updated.CreatedAt) {
		t.Error("UpdatedAt 应 >= CreatedAt")
	}
}

func TestUpdateReview_NotOwner(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	_, err := rm.UpdateReview(r.ID, "user2", "篡改", 1)
	if err != ErrNotReviewOwner {
		t.Errorf("非所有者应返回 ErrNotReviewOwner, 得到: %v", err)
	}
}

func TestUpdateReview_InvalidRating(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	_, err := rm.UpdateReview(r.ID, "user1", "内容", 0)
	if err != ErrInvalidRating {
		t.Errorf("期望 ErrInvalidRating, 得到: %v", err)
	}
}

func TestDeleteReview(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	err := rm.DeleteReview(r.ID, "user1", false)
	if err != nil {
		t.Fatalf("DeleteReview 失败: %v", err)
	}
	_, err = rm.GetReview(r.ID)
	if err != ErrReviewNotFound {
		t.Errorf("删除后应返回 ErrReviewNotFound, 得到: %v", err)
	}
}

func TestDeleteReview_Admin(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	err := rm.DeleteReview(r.ID, "admin", true)
	if err != nil {
		t.Fatalf("管理员删除应成功: %v", err)
	}
}

func TestDeleteReview_NotOwner(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	err := rm.DeleteReview(r.ID, "user2", false)
	if err != ErrNotReviewOwner {
		t.Errorf("非所有者应返回 ErrNotReviewOwner, 得到: %v", err)
	}
}

func TestGetReviewsByApp(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 5)
	rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 4)
	rm.CreateReview("app2", "user3", "王五", "", "内容3", "1.0", 3)

	reviews := rm.GetReviewsByApp("app1", false)
	if len(reviews) != 2 {
		t.Errorf("app1 评价数 = %d, 期望 2", len(reviews))
	}
}

func TestGetReviewsByApp_Hidden(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	rm.HideReview(r.ID, "test")

	reviews := rm.GetReviewsByApp("app1", false)
	if len(reviews) != 0 {
		t.Errorf("隐藏后不应返回, 得到 %d", len(reviews))
	}
	reviews = rm.GetReviewsByApp("app1", true)
	if len(reviews) != 1 {
		t.Errorf("包含隐藏应返回 1, 得到 %d", len(reviews))
	}
}

func TestGetReviewsByUser(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 5)
	rm.CreateReview("app2", "user1", "张三", "", "内容2", "1.0", 4)
	rm.CreateReview("app3", "user2", "李四", "", "内容3", "1.0", 3)

	reviews := rm.GetReviewsByUser("user1")
	if len(reviews) != 2 {
		t.Errorf("user1 评价数 = %d, 期望 2", len(reviews))
	}
}

func TestVoteHelpful(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)

	err := rm.VoteHelpful(r.ID, true)
	if err != nil {
		t.Fatalf("VoteHelpful 失败: %v", err)
	}
	got, _ := rm.GetReview(r.ID)
	if got.Helpful != 1 {
		t.Errorf("Helpful = %d, 期望 1", got.Helpful)
	}

	err = rm.VoteHelpful(r.ID, false)
	if err != nil {
		t.Fatalf("VoteHelpful(false) 失败: %v", err)
	}
	got, _ = rm.GetReview(r.ID)
	if got.NotHelpful != 1 {
		t.Errorf("NotHelpful = %d, 期望 1", got.NotHelpful)
	}
}

func TestSetDeveloperReply(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)

	err := rm.SetDeveloperReply(r.ID, "dev1", "开发者", "感谢反馈！")
	if err != nil {
		t.Fatalf("SetDeveloperReply 失败: %v", err)
	}
	got, _ := rm.GetReview(r.ID)
	if got.DeveloperReply == nil {
		t.Fatal("DeveloperReply 不应为 nil")
	}
	if got.DeveloperReply.Content != "感谢反馈！" {
		t.Errorf("Content = %q, 期望 %q", got.DeveloperReply.Content, "感谢反馈！")
	}
	if got.DeveloperReply.AuthorID != "dev1" {
		t.Errorf("AuthorID = %q, 期望 dev1", got.DeveloperReply.AuthorID)
	}
}

func TestSetDeveloperReply_Update(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)

	rm.SetDeveloperReply(r.ID, "dev1", "开发者", "回复1")
	rm.SetDeveloperReply(r.ID, "dev1", "开发者", "回复2")
	got, _ := rm.GetReview(r.ID)
	if got.DeveloperReply.Content != "回复2" {
		t.Errorf("Content = %q, 期望 %q", got.DeveloperReply.Content, "回复2")
	}
}

func TestSetDeveloperReply_EmptyContent(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	err := rm.SetDeveloperReply(r.ID, "dev1", "开发者", "  ")
	if err != ErrEmptyContent {
		t.Errorf("期望 ErrEmptyContent, 得到: %v", err)
	}
}

func TestHideAndUnhideReview(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)

	err := rm.HideReview(r.ID, "不当内容")
	if err != nil {
		t.Fatalf("HideReview 失败: %v", err)
	}
	got, _ := rm.GetReview(r.ID)
	if !got.Hidden {
		t.Error("评价应被隐藏")
	}
	if got.HiddenReason != "不当内容" {
		t.Errorf("HiddenReason = %q, 期望 %q", got.HiddenReason, "不当内容")
	}

	err = rm.UnhideReview(r.ID)
	if err != nil {
		t.Fatalf("UnhideReview 失败: %v", err)
	}
	got, _ = rm.GetReview(r.ID)
	if got.Hidden {
		t.Error("评价应取消隐藏")
	}
	if got.HiddenReason != "" {
		t.Errorf("HiddenReason = %q, 期望空", got.HiddenReason)
	}
}

func TestCount(t *testing.T) {
	rm := NewReviewManager()
	if rm.Count() != 0 {
		t.Errorf("初始 Count = %d, 期望 0", rm.Count())
	}
	rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	rm.CreateReview("app2", "user2", "李四", "", "内容", "1.0", 4)
	if rm.Count() != 2 {
		t.Errorf("Count = %d, 期望 2", rm.Count())
	}
}

// ============================================================
// ReviewModerator 测试
// ============================================================

func TestReportReview(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 5)

	err := mod.ReportReview(r.ID, "user2", "垃圾广告", "这是广告内容")
	if err != nil {
		t.Fatalf("ReportReview 失败: %v", err)
	}
	if mod.GetReportCount(r.ID) != 1 {
		t.Errorf("ReportCount = %d, 期望 1", mod.GetReportCount(r.ID))
	}
}

func TestReportReview_Duplicate(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 5)

	mod.ReportReview(r.ID, "user2", "垃圾广告", "")
	err := mod.ReportReview(r.ID, "user2", "垃圾广告", "")
	if err != ErrDuplicateReport {
		t.Errorf("重复举报应返回 ErrDuplicateReport, 得到: %v", err)
	}
}

func TestReportReview_EmptyReason(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 5)

	err := mod.ReportReview(r.ID, "user2", "", "")
	if err != ErrEmptyReason {
		t.Errorf("空原因应返回 ErrEmptyReason, 得到: %v", err)
	}
}

func TestReportReview_AutoHide(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 3) // 阈值 3

	for i := 0; i < 3; i++ {
		userID := "reporter" + string(rune('A'+i))
		mod.ReportReview(r.ID, userID, "举报", "")
	}

	got, _ := rm.GetReview(r.ID)
	if !got.Hidden {
		t.Error("达到阈值应自动隐藏")
	}
	if !strings.Contains(got.HiddenReason, "too many reports") {
		t.Errorf("HiddenReason = %q, 应包含 'too many reports'", got.HiddenReason)
	}
}

func TestSetThreshold(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 10)
	mod.SetThreshold(2)

	mod.ReportReview(r.ID, "u2", "r", "")
	err := mod.ReportReview(r.ID, "u3", "r", "")
	if err != nil {
		t.Fatalf("第二次举报失败: %v", err)
	}
	got, _ := rm.GetReview(r.ID)
	if !got.Hidden {
		t.Error("阈值改为 2 后，2 次举报应自动隐藏")
	}
}

func TestGetReports(t *testing.T) {
	rm := NewReviewManager()
	r, _ := rm.CreateReview("app1", "user1", "张三", "", "内容", "1.0", 5)
	mod := NewReviewModerator(rm, 10)

	mod.ReportReview(r.ID, "u2", "原因A", "详情A")
	mod.ReportReview(r.ID, "u3", "原因B", "详情B")

	reports := mod.GetReports(r.ID)
	if len(reports) != 2 {
		t.Fatalf("报告数 = %d, 期望 2", len(reports))
	}
	if reports[0].Reason != "原因A" {
		t.Errorf("第一个举报原因 = %q, 期望 %q", reports[0].Reason, "原因A")
	}
}

func TestReportReview_NotFound(t *testing.T) {
	rm := NewReviewManager()
	mod := NewReviewModerator(rm, 5)
	err := mod.ReportReview("nonexistent", "u2", "reason", "")
	if err != ErrReviewNotFound {
		t.Errorf("期望 ErrReviewNotFound, 得到: %v", err)
	}
}

// ============================================================
// ReviewSearcher 测试
// ============================================================

func TestSearch_ByApp(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "很好", "1.0", 5)
	rm.CreateReview("app1", "user2", "李四", "", "一般", "1.0", 3)
	rm.CreateReview("app2", "user3", "王五", "", "差", "1.0", 1)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1"})
	if len(result) != 2 {
		t.Errorf("app1 搜索结果 = %d, 期望 2", len(result))
	}
}

func TestSearch_ByRating(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "很好", "1.0", 5)
	rm.CreateReview("app1", "user2", "李四", "", "一般", "1.0", 3)
	rm.CreateReview("app1", "user3", "王五", "", "差", "1.0", 1)

	rs := NewReviewSearcher(rm)
	rating := 5
	result := rs.Search(ReviewFilter{AppID: "app1", Rating: &rating})
	if len(result) != 1 {
		t.Fatalf("结果 = %d, 期望 1", len(result))
	}
	if result[0].Rating != 5 {
		t.Errorf("Rating = %d, 期望 5", result[0].Rating)
	}
}

func TestSearch_ByKeyword(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "标题", "这个应用很好用", "1.0", 5)
	rm.CreateReview("app1", "user2", "李四", "", "体验一般", "1.0", 3)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", Keyword: "很好"})
	if len(result) != 1 {
		t.Fatalf("关键词搜索结果 = %d, 期望 1", len(result))
	}
	if !strings.Contains(result[0].Content, "很好") {
		t.Errorf("结果应包含关键词")
	}
}

func TestSearch_ByHasReply(t *testing.T) {
	rm := NewReviewManager()
	r1, _ := rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 5)
	r2, _ := rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 4)
	rm.SetDeveloperReply(r1.ID, "dev1", "开发者", "谢谢")

	rs := NewReviewSearcher(rm)
	hasReply := true
	result := rs.Search(ReviewFilter{AppID: "app1", HasReply: &hasReply})
	if len(result) != 1 {
		t.Fatalf("有回复的结果 = %d, 期望 1", len(result))
	}
	if result[0].ID != r1.ID {
		t.Errorf("应返回有回复的评价")
	}

	hasReply = false
	result = rs.Search(ReviewFilter{AppID: "app1", HasReply: &hasReply})
	if len(result) != 1 {
		t.Fatalf("无回复的结果 = %d, 期望 1", len(result))
	}
	if result[0].ID != r2.ID {
		t.Errorf("应返回无回复的评价")
	}
}

func TestSearch_SortNewest(t *testing.T) {
	rm := NewReviewManager()
	r1, _ := rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 5)
	time.Sleep(10 * time.Millisecond)
	r2, _ := rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 4)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", Sort: SortNewest})
	if len(result) != 2 {
		t.Fatalf("结果 = %d, 期望 2", len(result))
	}
	if result[0].ID != r2.ID {
		t.Error("SortNewest 应返回最新的在前")
	}
	if result[1].ID != r1.ID {
		t.Error("SortNewest 第二条应为最早的评价")
	}
}

func TestSearch_SortHighest(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 3)
	rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 5)
	rm.CreateReview("app1", "user3", "王五", "", "内容3", "1.0", 1)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", Sort: SortHighest})
	if result[0].Rating != 5 {
		t.Errorf("SortHighest 第一条 Rating = %d, 期望 5", result[0].Rating)
	}
	if result[2].Rating != 1 {
		t.Errorf("SortHighest 最后一条 Rating = %d, 期望 1", result[2].Rating)
	}
}

func TestSearch_SortLowest(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 3)
	rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 5)
	rm.CreateReview("app1", "user3", "王五", "", "内容3", "1.0", 1)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", Sort: SortLowest})
	if result[0].Rating != 1 {
		t.Errorf("SortLowest 第一条 Rating = %d, 期望 1", result[0].Rating)
	}
}

func TestSearch_SortHelpful(t *testing.T) {
	rm := NewReviewManager()
	r1, _ := rm.CreateReview("app1", "user1", "张三", "", "内容1", "1.0", 5)
	r2, _ := rm.CreateReview("app1", "user2", "李四", "", "内容2", "1.0", 5)
	rm.VoteHelpful(r1.ID, true)
	rm.VoteHelpful(r1.ID, true)
	rm.VoteHelpful(r2.ID, true)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", Sort: SortHelpful})
	if result[0].ID != r1.ID {
		t.Error("SortHelpful 应返回 Helpful 最多的在前")
	}
}

func TestSearch_MinMaxRating(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "u1", "A", "", "内容1", "1.0", 1)
	rm.CreateReview("app1", "u2", "B", "", "内容2", "1.0", 3)
	rm.CreateReview("app1", "u3", "C", "", "内容3", "1.0", 5)

	rs := NewReviewSearcher(rm)
	result := rs.Search(ReviewFilter{AppID: "app1", MinRating: 3})
	if len(result) != 2 {
		t.Errorf("MinRating=3 结果 = %d, 期望 2", len(result))
	}

	result = rs.Search(ReviewFilter{AppID: "app1", MaxRating: 3})
	if len(result) != 2 {
		t.Errorf("MaxRating=3 结果 = %d, 期望 2", len(result))
	}

	result = rs.Search(ReviewFilter{AppID: "app1", MinRating: 2, MaxRating: 4})
	if len(result) != 1 {
		t.Errorf("MinRating=2,MaxRating=4 结果 = %d, 期望 1", len(result))
	}
}

// ============================================================
// ReviewAggregator 测试
// ============================================================

func TestGetStats(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "u1", "A", "", "内容1", "1.0", 5)
	rm.CreateReview("app1", "u2", "B", "", "内容2", "1.0", 3)
	rm.CreateReview("app1", "u3", "C", "", "内容3", "1.0", 4)

	agg := NewReviewAggregator(rm)
	stats := agg.GetStats("app1")

	if stats.TotalReviews != 3 {
		t.Errorf("TotalReviews = %d, 期望 3", stats.TotalReviews)
	}
	if stats.AverageRating != 4.0 {
		t.Errorf("AverageRating = %f, 期望 4.0", stats.AverageRating)
	}
	if stats.Distribution[5] != 1 {
		t.Errorf("Distribution[5] = %d, 期望 1", stats.Distribution[5])
	}
	if stats.Distribution[3] != 1 {
		t.Errorf("Distribution[3] = %d, 期望 1", stats.Distribution[3])
	}
	if stats.Distribution[4] != 1 {
		t.Errorf("Distribution[4] = %d, 期望 1", stats.Distribution[4])
	}
	if stats.VerifiedCount != 3 {
		t.Errorf("VerifiedCount = %d, 期望 3", stats.VerifiedCount)
	}
}

func TestGetStats_Empty(t *testing.T) {
	rm := NewReviewManager()
	agg := NewReviewAggregator(rm)
	stats := agg.GetStats("nonexistent")

	if stats.TotalReviews != 0 {
		t.Errorf("TotalReviews = %d, 期望 0", stats.TotalReviews)
	}
	if stats.AverageRating != 0 {
		t.Errorf("AverageRating = %f, 期望 0", stats.AverageRating)
	}
}

func TestGetStats_Rounding(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "u1", "A", "", "内容", "1.0", 5)
	rm.CreateReview("app1", "u2", "B", "", "内容", "1.0", 4)
	rm.CreateReview("app1", "u3", "C", "", "内容", "1.0", 4)

	agg := NewReviewAggregator(rm)
	stats := agg.GetStats("app1")
	// (5+4+4)/3 = 4.333... → 4.3
	if stats.AverageRating != 4.3 {
		t.Errorf("AverageRating = %f, 期望 4.3", stats.AverageRating)
	}
}

func TestGetStats_RepliedCount(t *testing.T) {
	rm := NewReviewManager()
	r1, _ := rm.CreateReview("app1", "u1", "A", "", "内容1", "1.0", 5)
	rm.CreateReview("app1", "u2", "B", "", "内容2", "1.0", 4)
	rm.SetDeveloperReply(r1.ID, "dev1", "开发者", "谢谢")

	agg := NewReviewAggregator(rm)
	stats := agg.GetStats("app1")
	if stats.RepliedCount != 1 {
		t.Errorf("RepliedCount = %d, 期望 1", stats.RepliedCount)
	}
}

func TestGetTopRatedApps(t *testing.T) {
	rm := NewReviewManager()
	// app1: avg 5.0
	rm.CreateReview("app1", "u1", "A", "", "内容", "1.0", 5)
	// app2: avg 3.0
	rm.CreateReview("app2", "u2", "B", "", "内容", "1.0", 3)
	// app3: avg 4.0
	rm.CreateReview("app3", "u3", "C", "", "内容", "1.0", 4)

	agg := NewReviewAggregator(rm)
	top := agg.GetTopRatedApps([]string{"app1", "app2", "app3"}, 2)
	if len(top) != 2 {
		t.Fatalf("结果 = %d, 期望 2", len(top))
	}
	if top[0].AppID != "app1" {
		t.Errorf("第一名 AppID = %q, 期望 app1", top[0].AppID)
	}
	if top[1].AppID != "app3" {
		t.Errorf("第二名 AppID = %q, 期望 app3", top[1].AppID)
	}
}

func TestGetTopRatedApps_SkipEmpty(t *testing.T) {
	rm := NewReviewManager()
	rm.CreateReview("app1", "u1", "A", "", "内容", "1.0", 5)

	agg := NewReviewAggregator(rm)
	top := agg.GetTopRatedApps([]string{"app1", "app2"}, 10)
	if len(top) != 1 {
		t.Errorf("结果 = %d, 期望 1 (无评价的应用应跳过)", len(top))
	}
}

// ============================================================
// APIHandler 测试
// ============================================================

func TestAPI_CreateReview(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	resp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "张三",
		Title: "好评", Content: "很好用", Version: "1.0", Rating: 5,
	})
	if !resp.Success {
		t.Fatalf("创建失败: %s", resp.Error)
	}
	r, ok := resp.Data.(*Review)
	if !ok {
		t.Fatal("返回类型应为 *Review")
	}
	if r.AppID != "app1" {
		t.Errorf("AppID = %q, 期望 app1", r.AppID)
	}
}

func TestAPI_CreateReview_Invalid(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	resp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "", UserID: "u1", Content: "内容", Rating: 5,
	})
	if resp.Success {
		t.Error("空 AppID 应失败")
	}
	if resp.Error != ErrEmptyAppID.Error() {
		t.Errorf("Error = %q, 期望 %q", resp.Error, ErrEmptyAppID.Error())
	}
}

func TestAPI_GetReviews(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容1", Version: "1.0", Rating: 5,
	})
	h.HandleCreateReview(CreateReviewRequest{
		AppID: "app2", UserID: "u2", UserName: "B", Content: "内容2", Version: "1.0", Rating: 3,
	})

	resp := h.HandleGetReviews(ReviewFilter{AppID: "app1"})
	if !resp.Success {
		t.Fatalf("获取失败: %s", resp.Error)
	}
	reviews, ok := resp.Data.([]*Review)
	if !ok {
		t.Fatal("返回类型应为 []*Review")
	}
	if len(reviews) != 1 {
		t.Errorf("结果 = %d, 期望 1", len(reviews))
	}
}

func TestAPI_ReplyReview(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 4,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleReplyReview(ReplyReviewRequest{
		ReviewID: review.ID, AuthorID: "dev1", AuthorName: "开发者", Content: "感谢反馈",
	})
	if !resp.Success {
		t.Fatalf("回复失败: %s", resp.Error)
	}
	r := resp.Data.(*Review)
	if r.DeveloperReply == nil {
		t.Fatal("DeveloperReply 不应为 nil")
	}
	if r.DeveloperReply.Content != "感谢反馈" {
		t.Errorf("Content = %q, 期望 %q", r.DeveloperReply.Content, "感谢反馈")
	}
}

func TestAPI_ReplyReview_NotFound(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	resp := h.HandleReplyReview(ReplyReviewRequest{
		ReviewID: "nonexistent", AuthorID: "dev1", AuthorName: "dev", Content: "回复",
	})
	if resp.Success {
		t.Error("不存在的评价应失败")
	}
}

func TestAPI_ReportReview(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 1,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleReportReview(ReportReviewRequest{
		ReviewID: review.ID, UserID: "u2", Reason: "垃圾广告", Detail: "含广告链接",
	})
	if !resp.Success {
		t.Fatalf("举报失败: %s", resp.Error)
	}
}

func TestAPI_ReportReview_EmptyReason(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 5,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleReportReview(ReportReviewRequest{
		ReviewID: review.ID, UserID: "u2", Reason: "", Detail: "",
	})
	if resp.Success {
		t.Error("空原因应失败")
	}
}

func TestAPI_GetReviewStats(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	h.HandleCreateReview(CreateReviewRequest{AppID: "app1", UserID: "u1", UserName: "A", Content: "c", Version: "1.0", Rating: 5})
	h.HandleCreateReview(CreateReviewRequest{AppID: "app1", UserID: "u2", UserName: "B", Content: "c", Version: "1.0", Rating: 3})

	resp := h.HandleGetReviewStats("app1")
	if !resp.Success {
		t.Fatalf("获取统计失败: %s", resp.Error)
	}
	stats, ok := resp.Data.(*ReviewStats)
	if !ok {
		t.Fatal("返回类型应为 *ReviewStats")
	}
	if stats.TotalReviews != 2 {
		t.Errorf("TotalReviews = %d, 期望 2", stats.TotalReviews)
	}
	if stats.AverageRating != 4.0 {
		t.Errorf("AverageRating = %f, 期望 4.0", stats.AverageRating)
	}
}

func TestAPI_DeleteReview(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 5,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleDeleteReview(review.ID, "u1", false)
	if !resp.Success {
		t.Fatalf("删除失败: %s", resp.Error)
	}

	// 确认已删除
	getResp := h.HandleGetReviews(ReviewFilter{AppID: "app1"})
	reviews := getResp.Data.([]*Review)
	if len(reviews) != 0 {
		t.Errorf("删除后应无评价, 得到 %d", len(reviews))
	}
}

func TestAPI_DeleteReview_NotOwner(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 5,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleDeleteReview(review.ID, "u2", false)
	if resp.Success {
		t.Error("非所有者删除应失败")
	}
}

func TestAPI_VoteHelpful(t *testing.T) {
	h := NewAPIHandler(NewReviewManager())
	createResp := h.HandleCreateReview(CreateReviewRequest{
		AppID: "app1", UserID: "u1", UserName: "A", Content: "内容", Version: "1.0", Rating: 5,
	})
	review := createResp.Data.(*Review)

	resp := h.HandleVoteHelpful(review.ID, true)
	if !resp.Success {
		t.Fatalf("投票失败: %s", resp.Error)
	}

	got, _ := h.manager.GetReview(review.ID)
	if got.Helpful != 1 {
		t.Errorf("Helpful = %d, 期望 1", got.Helpful)
	}
}

// ============================================================
// 并发测试
// ============================================================

func TestConcurrentCreateReview(t *testing.T) {
	rm := NewReviewManager()
	done := make(chan error, 10)

	for i := 0; i < 10; i++ {
		i := i
		go func() {
			_, err := rm.CreateReview("app1", "user"+string(rune('0'+i)), "name", "", "内容", "1.0", 5)
			done <- err
		}()
	}

	for i := 0; i < 10; i++ {
		err := <-done
		if err != nil {
			t.Errorf("并发创建 %d 失败: %v", i, err)
		}
	}

	if rm.Count() != 10 {
		t.Errorf("Count = %d, 期望 10", rm.Count())
	}
}

func TestGenerateReviewID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateReviewID()
		if ids[id] {
			t.Fatalf("生成了重复 ID: %s", id)
		}
		ids[id] = true
		if !strings.HasPrefix(id, "rev_") {
			t.Errorf("ID %q 应以 rev_ 开头", id)
		}
	}
}

func TestReviewError(t *testing.T) {
	e := reviewError("测试错误")
	if e.Error() != "测试错误" {
		t.Errorf("Error() = %q, 期望 %q", e.Error(), "测试错误")
	}
}

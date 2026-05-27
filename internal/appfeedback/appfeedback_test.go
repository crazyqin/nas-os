package appfeedback

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRating_Validate(t *testing.T) {
	tests := []struct {
		name   string
		rating Rating
		valid  bool
	}{
		{"valid 1", 1.0, true},
		{"valid 1.5", 1.5, true},
		{"valid 2", 2.0, true},
		{"valid 3.5", 3.5, true},
		{"valid 5", 5.0, true},
		{"valid 4.5", 4.5, true},
		{"invalid 0", 0.0, false},
		{"invalid 0.3", 0.3, false},
		{"invalid 5.5", 5.5, false},
		{"invalid -1", -1.0, false},
		{"invalid 1.1", 1.1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rating.Validate(); got != tt.valid {
				t.Errorf("Rating.Validate() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestMemoryStore_CreateAndGetFeedback(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:       "test-1",
		AppID:    "app-1",
		Version:  "1.0.0",
		UserID:   "user-1",
		Rating:   4.5,
		Content:  "Great app!",
		Category: CategoryPraise,
	}

	err := store.CreateFeedback(feedback)
	if err != nil {
		t.Fatalf("CreateFeedback failed: %v", err)
	}

	got, err := store.GetFeedback("test-1")
	if err != nil {
		t.Fatalf("GetFeedback failed: %v", err)
	}

	if got.ID != feedback.ID {
		t.Errorf("expected ID %s, got %s", feedback.ID, got.ID)
	}
	if got.Rating != feedback.Rating {
		t.Errorf("expected Rating %v, got %v", feedback.Rating, got.Rating)
	}
}

func TestMemoryStore_GetFeedbackNotFound(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.GetFeedback("nonexistent")
	if err != ErrFeedbackNotFound {
		t.Errorf("expected ErrFeedbackNotFound, got %v", err)
	}
}

func TestMemoryStore_UpdateFeedback(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:      "test-1",
		AppID:   "app-1",
		Version: "1.0.0",
		UserID:  "user-1",
		Rating:  4.0,
		Content: "Good app",
	}
	store.CreateFeedback(feedback)

	feedback.Rating = 5.0
	feedback.Content = "Excellent app!"
	err := store.UpdateFeedback(feedback)
	if err != nil {
		t.Fatalf("UpdateFeedback failed: %v", err)
	}

	got, _ := store.GetFeedback("test-1")
	if got.Rating != 5.0 {
		t.Errorf("expected Rating 5.0, got %v", got.Rating)
	}
}

func TestMemoryStore_DeleteFeedback(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:      "test-1",
		AppID:   "app-1",
		Version: "1.0.0",
		UserID:  "user-1",
		Rating:  4.0,
		Content: "Good app",
	}
	store.CreateFeedback(feedback)

	err := store.DeleteFeedback("test-1")
	if err != nil {
		t.Fatalf("DeleteFeedback failed: %v", err)
	}

	_, err = store.GetFeedback("test-1")
	if err != ErrFeedbackNotFound {
		t.Errorf("expected ErrFeedbackNotFound, got %v", err)
	}
}

func TestMemoryStore_Vote(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:      "test-1",
		AppID:   "app-1",
		Version: "1.0.0",
		UserID:  "user-1",
		Rating:  4.0,
		Content: "Good app",
	}
	store.CreateFeedback(feedback)

	vote := &Vote{
		ID:         "vote-1",
		FeedbackID: "test-1",
		UserID:     "user-2",
		IsHelpful:  true,
	}

	err := store.CreateVote(vote)
	if err != nil {
		t.Fatalf("CreateVote failed: %v", err)
	}

	got, _ := store.GetFeedback("test-1")
	if got.Helpful != 1 {
		t.Errorf("expected Helpful 1, got %d", got.Helpful)
	}

	// 尝试重复投票
	err = store.CreateVote(vote)
	if err != ErrAlreadyVoted {
		t.Errorf("expected ErrAlreadyVoted, got %v", err)
	}
}

func TestMemoryStore_Report(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:      "test-1",
		AppID:   "app-1",
		Version: "1.0.0",
		UserID:  "user-1",
		Rating:  4.0,
		Content: "Good app",
	}
	store.CreateFeedback(feedback)

	report := &Report{
		ID:         "report-1",
		FeedbackID: "test-1",
		UserID:     "user-2",
		Reason:     ReportSpam,
	}

	err := store.CreateReport(report)
	if err != nil {
		t.Fatalf("CreateReport failed: %v", err)
	}

	got, _ := store.GetFeedback("test-1")
	if got.ReportCount != 1 {
		t.Errorf("expected ReportCount 1, got %d", got.ReportCount)
	}

	// 尝试重复举报
	err = store.CreateReport(report)
	if err != ErrAlreadyReported {
		t.Errorf("expected ErrAlreadyReported, got %v", err)
	}
}

func TestMemoryStore_Reply(t *testing.T) {
	store := NewMemoryStore()

	feedback := &Feedback{
		ID:         "test-1",
		AppID:      "app-1",
		Version:    "1.0.0",
		UserID:     "user-1",
		Rating:     4.0,
		Content:    "Good app",
		ReplyCount: 0,
	}
	store.CreateFeedback(feedback)

	reply := &FeedbackReply{
		ID:         "reply-1",
		FeedbackID: "test-1",
		UserID:     "user-2",
		IsDev:      true,
		Content:    "Thanks for the feedback!",
	}

	err := store.CreateReply(reply)
	if err != nil {
		t.Fatalf("CreateReply failed: %v", err)
	}

	got, _ := store.GetFeedback("test-1")
	if got.ReplyCount != 1 {
		t.Errorf("expected ReplyCount 1, got %d", got.ReplyCount)
	}

	replies, _ := store.ListReplies("test-1")
	if len(replies) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies))
	}
}

func TestMemoryStore_RatingStats(t *testing.T) {
	store := NewMemoryStore()

	feedbacks := []*Feedback{
		{ID: "1", AppID: "app-1", Rating: 5.0, Content: "Excellent"},
		{ID: "2", AppID: "app-1", Rating: 4.0, Content: "Good"},
		{ID: "3", AppID: "app-1", Rating: 3.0, Content: "Average"},
		{ID: "4", AppID: "app-1", Rating: 2.0, Content: "Poor"},
		{ID: "5", AppID: "app-1", Rating: 1.0, Content: "Bad"},
	}

	for _, f := range feedbacks {
		store.CreateFeedback(f)
	}

	stats, err := store.GetRatingStats("app-1")
	if err != nil {
		t.Fatalf("GetRatingStats failed: %v", err)
	}

	if stats.TotalCount != 5 {
		t.Errorf("expected TotalCount 5, got %d", stats.TotalCount)
	}

	expectedAvg := 3.0
	if stats.Average != expectedAvg {
		t.Errorf("expected Average %v, got %v", expectedAvg, stats.Average)
	}

	if stats.Distribution[5] != 1 {
		t.Errorf("expected 1 five-star, got %d", stats.Distribution[5])
	}
	if stats.Distribution[4] != 1 {
		t.Errorf("expected 1 four-star, got %d", stats.Distribution[4])
	}
}

func TestService_CreateFeedback(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	req := CreateFeedbackRequest{
		AppID:    "app-1",
		Version:  "1.0.0",
		Rating:   4.5,
		Content:  "Great app!",
		Category: CategoryPraise,
	}

	feedback, err := service.CreateFeedback("user-1", req)
	if err != nil {
		t.Fatalf("CreateFeedback failed: %v", err)
	}

	if feedback.AppID != req.AppID {
		t.Errorf("expected AppID %s, got %s", req.AppID, feedback.AppID)
	}
	if feedback.Rating != req.Rating {
		t.Errorf("expected Rating %v, got %v", req.Rating, feedback.Rating)
	}
}

func TestService_CreateFeedbackInvalidRating(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	req := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  1.1, // invalid
		Content: "Test",
	}

	_, err := service.CreateFeedback("user-1", req)
	if err != ErrInvalidRating {
		t.Errorf("expected ErrInvalidRating, got %v", err)
	}
}

func TestService_UpdateFeedbackUnauthorized(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	// 创建反馈
	req := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  4.0,
		Content: "Good",
	}
	feedback, _ := service.CreateFeedback("user-1", req)

	// 尝试用另一个用户更新
	updateReq := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  5.0,
		Content: "Updated",
	}
	_, err := service.UpdateFeedback(feedback.ID, "user-2", updateReq)
	if err.Error() != "unauthorized: can only update own feedback" {
		t.Errorf("expected unauthorized error, got %v", err)
	}
}

func TestHandler_CreateFeedback(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	body := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  4.5,
		Content: "Great app!",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/feedbacks", bytes.NewBuffer(jsonBody))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.CreateFeedback(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var feedback Feedback
	json.NewDecoder(w.Body).Decode(&feedback)
	if feedback.AppID != "app-1" {
		t.Errorf("expected AppID app-1, got %s", feedback.AppID)
	}
}

func TestHandler_GetFeedback(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 先创建一个反馈
	req := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  4.5,
		Content: "Great app!",
	}
	feedback, _ := service.CreateFeedback("user-1", req)

	// 获取反馈
	httpReq := httptest.NewRequest(http.MethodGet, "/feedbacks?id="+feedback.ID, nil)
	w := httptest.NewRecorder()
	handler.GetFeedback(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var got Feedback
	json.NewDecoder(w.Body).Decode(&got)
	if got.ID != feedback.ID {
		t.Errorf("expected ID %s, got %s", feedback.ID, got.ID)
	}
}

func TestHandler_ListFeedbacks(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 创建多个反馈
	for i := 0; i < 5; i++ {
		req := CreateFeedbackRequest{
			AppID:   "app-1",
			Version: "1.0.0",
			Rating:  Rating(float32(i+1) * 1.0),
			Content: "Feedback",
		}
		service.CreateFeedback("user-1", req)
	}

	// 列出反馈
	httpReq := httptest.NewRequest(http.MethodGet, "/feedbacks?app_id=app-1&page=1&page_size=10", nil)
	w := httptest.NewRecorder()
	handler.ListFeedbacks(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result PaginatedFeedbacks
	json.NewDecoder(w.Body).Decode(&result)
	if result.Total != 5 {
		t.Errorf("expected total 5, got %d", result.Total)
	}
}

func TestHandler_Vote(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 创建反馈
	req := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  4.0,
		Content: "Good app",
	}
	feedback, _ := service.CreateFeedback("user-1", req)

	// 投票
	voteReq := CreateVoteRequest{IsHelpful: true}
	jsonBody, _ := json.Marshal(voteReq)

	httpReq := httptest.NewRequest(http.MethodPost, "/feedbacks/vote?feedback_id="+feedback.ID, bytes.NewBuffer(jsonBody))
	httpReq.Header.Set("X-User-ID", "user-2")
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Vote(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandler_Report(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 创建反馈
	req := CreateFeedbackRequest{
		AppID:   "app-1",
		Version: "1.0.0",
		Rating:  4.0,
		Content: "Good app",
	}
	feedback, _ := service.CreateFeedback("user-1", req)

	// 举报
	reportReq := CreateReportRequest{
		Reason:  ReportSpam,
		Details: "This is spam",
	}
	jsonBody, _ := json.Marshal(reportReq)

	httpReq := httptest.NewRequest(http.MethodPost, "/feedbacks/report?feedback_id="+feedback.ID, bytes.NewBuffer(jsonBody))
	httpReq.Header.Set("X-User-ID", "user-2")
	httpReq.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Report(w, httpReq)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}
}

func TestHandler_GetRatingStats(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 创建反馈
	for i := 0; i < 3; i++ {
		req := CreateFeedbackRequest{
			AppID:   "app-1",
			Version: "1.0.0",
			Rating:  4.0,
			Content: "Good",
		}
		service.CreateFeedback("user-1", req)
	}

	// 获取统计
	httpReq := httptest.NewRequest(http.MethodGet, "/stats?app_id=app-1", nil)
	w := httptest.NewRecorder()
	handler.GetRatingStats(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var stats RatingStats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", stats.TotalCount)
	}
	if stats.Average != 4.0 {
		t.Errorf("expected Average 4.0, got %v", stats.Average)
	}
}

func TestHandler_GenerateSummary(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)
	handler := NewHandler(service)

	// 创建反馈
	reqs := []CreateFeedbackRequest{
		{AppID: "app-1", Version: "1.0.0", Rating: 5.0, Content: "Excellent app, love it!"},
		{AppID: "app-1", Version: "1.0.0", Rating: 4.0, Content: "Good app, works well"},
		{AppID: "app-1", Version: "1.0.0", Rating: 1.0, Content: "Terrible app, crashes"},
	}
	for _, req := range reqs {
		service.CreateFeedback("user-1", req)
	}

	// 生成摘要
	httpReq := httptest.NewRequest(http.MethodGet, "/summary?app_id=app-1", nil)
	w := httptest.NewRecorder()
	handler.GenerateSummary(w, httpReq)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var summary CommentSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", summary.TotalCount)
	}
}

func TestService_GenerateSummary(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	// 创建反馈
	reqs := []CreateFeedbackRequest{
		{AppID: "app-1", Version: "1.0.0", Rating: 5.0, Content: "Excellent app, love it!"},
		{AppID: "app-1", Version: "1.0.0", Rating: 4.0, Content: "Good app, works well"},
		{AppID: "app-1", Version: "1.0.0", Rating: 2.0, Content: "Poor performance, slow"},
		{AppID: "app-1", Version: "1.0.0", Rating: 1.0, Content: "Terrible app, crashes constantly"},
	}
	for _, req := range reqs {
		service.CreateFeedback("user-1", req)
	}

	summary, err := service.GenerateSummary("app-1")
	if err != nil {
		t.Fatalf("GenerateSummary failed: %v", err)
	}

	if summary.TotalCount != 4 {
		t.Errorf("expected TotalCount 4, got %d", summary.TotalCount)
	}

	if len(summary.Positive) != 2 {
		t.Errorf("expected 2 positive, got %d", len(summary.Positive))
	}

	if len(summary.Negative) != 2 {
		t.Errorf("expected 2 negative, got %d", len(summary.Negative))
	}
}

package contentworkflow

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.contents == nil {
		t.Error("contents map not initialized")
	}
	if m.workflows == nil {
		t.Error("workflows map not initialized")
	}
	if m.templates == nil {
		t.Error("templates map not initialized")
	}
}

func TestCreateContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Type:   TypeDocument,
		Body:   "This is a test article",
		Author: "testuser",
		Tags:   []string{"test", "article"},
	}

	err := m.CreateContent(content)
	if err != nil {
		t.Fatalf("CreateContent failed: %v", err)
	}

	if content.ID == "" {
		t.Error("content ID not generated")
	}
	if content.Status != StatusDraft {
		t.Errorf("expected status '%s', got '%s'", StatusDraft, content.Status)
	}
	if content.Version != 1 {
		t.Errorf("expected version 1, got %d", content.Version)
	}
}

func TestUpdateContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Test Article",
		Type:  TypeDocument,
		Body:  "Original body",
	}
	m.CreateContent(content)

	content.Body = "Updated body"
	err := m.UpdateContent(content)
	if err != nil {
		t.Fatalf("UpdateContent failed: %v", err)
	}

	if content.Version != 2 {
		t.Errorf("expected version 2, got %d", content.Version)
	}
}

func TestUpdateContentNotFound(t *testing.T) {
	m := NewManager()

	content := &Content{
		ID:   "nonexistent",
		Body: "test",
	}

	err := m.UpdateContent(content)
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestDeleteContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Test Article",
		Type:  TypeDocument,
	}
	m.CreateContent(content)

	err := m.DeleteContent(content.ID)
	if err != nil {
		t.Fatalf("DeleteContent failed: %v", err)
	}

	if _, exists := m.contents[content.ID]; exists {
		t.Error("content still exists after deletion")
	}
}

func TestDeleteContentNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteContent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestGetContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Test Article",
		Type:  TypeDocument,
	}
	m.CreateContent(content)

	got, err := m.GetContent(content.ID)
	if err != nil {
		t.Fatalf("GetContent failed: %v", err)
	}
	if got.Title != "Test Article" {
		t.Errorf("expected title 'Test Article', got '%s'", got.Title)
	}
}

func TestGetContentNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetContent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestListContents(t *testing.T) {
	m := NewManager()

	m.CreateContent(&Content{Title: "Article 1", Type: TypeDocument})
	m.CreateContent(&Content{Title: "Article 2", Type: TypeDocument})
	m.CreateContent(&Content{Title: "Image 1", Type: TypeImage})

	contents := m.ListContents(TypeDocument, "")
	if len(contents) != 2 {
		t.Errorf("expected 2 documents, got %d", len(contents))
	}
}

func TestCreateWorkflow(t *testing.T) {
	m := NewManager()

	workflow := &Workflow{
		Name:        "Article Workflow",
		Description: "Standard article workflow",
		ContentType: TypeDocument,
		Stages: []Stage{
			{
				ID:   "draft",
				Name: "Draft",
				Status: StatusDraft,
			},
			{
				ID:   "review",
				Name: "Review",
				Status: StatusReview,
				Assignees: []string{"editor"},
			},
			{
				ID:   "publish",
				Name: "Publish",
				Status: StatusPublished,
			},
		},
		Enabled: true,
	}

	err := m.CreateWorkflow(workflow)
	if err != nil {
		t.Fatalf("CreateWorkflow failed: %v", err)
	}

	if workflow.ID == "" {
		t.Error("workflow ID not generated")
	}
}

func TestGetWorkflow(t *testing.T) {
	m := NewManager()

	workflow := &Workflow{
		Name: "Test Workflow",
	}
	m.CreateWorkflow(workflow)

	got, err := m.GetWorkflow(workflow.ID)
	if err != nil {
		t.Fatalf("GetWorkflow failed: %v", err)
	}
	if got.Name != "Test Workflow" {
		t.Errorf("expected name 'Test Workflow', got '%s'", got.Name)
	}
}

func TestGetWorkflowNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetWorkflow("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workflow")
	}
}

func TestListWorkflows(t *testing.T) {
	m := NewManager()

	m.CreateWorkflow(&Workflow{Name: "Workflow 1"})
	m.CreateWorkflow(&Workflow{Name: "Workflow 2"})

	workflows := m.ListWorkflows()
	if len(workflows) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(workflows))
	}
}

func TestSubmitForReview(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Test Article",
		Type:  TypeDocument,
	}
	m.CreateContent(content)

	err := m.SubmitForReview(content.ID)
	if err != nil {
		t.Fatalf("SubmitForReview failed: %v", err)
	}

	got, _ := m.GetContent(content.ID)
	if got.Status != StatusReview {
		t.Errorf("expected status '%s', got '%s'", StatusReview, got.Status)
	}
}

func TestSubmitForReviewNotDraft(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Type:   TypeDocument,
		Status: StatusPublished,
	}
	m.contents["test-id"] = content

	err := m.SubmitForReview("test-id")
	if err == nil {
		t.Error("expected error for non-draft content")
	}
}

func TestApproveContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Type:   TypeDocument,
		Status: StatusReview,
	}
	m.contents["test-id"] = content

	err := m.ApproveContent("test-id", "editor", "Looks good")
	if err != nil {
		t.Fatalf("ApproveContent failed: %v", err)
	}

	got, _ := m.GetContent("test-id")
	if got.Status != StatusApproved {
		t.Errorf("expected status '%s', got '%s'", StatusApproved, got.Status)
	}

	approvals := m.GetApprovalHistory("test-id")
	if len(approvals) != 1 {
		t.Errorf("expected 1 approval, got %d", len(approvals))
	}
}

func TestApproveContentNotInReview(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Status: StatusDraft,
	}
	m.contents["test-id"] = content

	err := m.ApproveContent("test-id", "editor", "comment")
	if err == nil {
		t.Error("expected error for non-review content")
	}
}

func TestPublishContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Status: StatusApproved,
	}
	m.contents["test-id"] = content

	err := m.PublishContent("test-id")
	if err != nil {
		t.Fatalf("PublishContent failed: %v", err)
	}

	got, _ := m.GetContent("test-id")
	if got.Status != StatusPublished {
		t.Errorf("expected status '%s', got '%s'", StatusPublished, got.Status)
	}
	if got.PublishedAt == nil {
		t.Error("published_at not set")
	}
}

func TestPublishContentNotApproved(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Status: StatusDraft,
	}
	m.contents["test-id"] = content

	err := m.PublishContent("test-id")
	if err == nil {
		t.Error("expected error for non-approved content")
	}
}

func TestArchiveContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title:  "Test Article",
		Status: StatusPublished,
	}
	m.contents["test-id"] = content

	err := m.ArchiveContent("test-id")
	if err != nil {
		t.Fatalf("ArchiveContent failed: %v", err)
	}

	got, _ := m.GetContent("test-id")
	if got.Status != StatusArchived {
		t.Errorf("expected status '%s', got '%s'", StatusArchived, got.Status)
	}
}

func TestSaveTemplate(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Template Source",
		Type:  TypeDocument,
		Body:  "Template body",
		Tags:  []string{"template"},
	}
	m.CreateContent(content)

	err := m.SaveTemplate(content.ID, "my-template")
	if err != nil {
		t.Fatalf("SaveTemplate failed: %v", err)
	}

	if _, exists := m.templates["my-template"]; !exists {
		t.Error("template not saved")
	}
}

func TestCreateFromTemplate(t *testing.T) {
	m := NewManager()

	m.templates["my-template"] = &Content{
		Title: "Template",
		Type:  TypeDocument,
		Body:  "Template body",
		Tags:  []string{"template"},
	}

	content, err := m.CreateFromTemplate("my-template", "New Article")
	if err != nil {
		t.Fatalf("CreateFromTemplate failed: %v", err)
	}

	if content.Title != "New Article" {
		t.Errorf("expected title 'New Article', got '%s'", content.Title)
	}
	if content.Body != "Template body" {
		t.Errorf("expected body 'Template body', got '%s'", content.Body)
	}
}

func TestCreateFromTemplateNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.CreateFromTemplate("nonexistent", "title")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestAIReviewContent(t *testing.T) {
	m := NewManager()

	content := &Content{
		Title: "Test Article",
		Body:  "This is a test article for AI review",
	}
	m.CreateContent(content)

	ctx := context.Background()
	result, err := m.AIReviewContent(ctx, content.ID)
	if err != nil {
		t.Fatalf("AIReviewContent failed: %v", err)
	}

	if result["score"] == nil {
		t.Error("score not returned")
	}
	if result["suggestions"] == nil {
		t.Error("suggestions not returned")
	}
}

func TestAIReviewContentNotFound(t *testing.T) {
	m := NewManager()

	ctx := context.Background()
	_, err := m.AIReviewContent(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent content")
	}
}

func TestGetApprovalHistory(t *testing.T) {
	m := NewManager()

	m.approvals = []*Approval{
		{ID: "a1", ContentID: "c1", Approver: "editor1"},
		{ID: "a2", ContentID: "c1", Approver: "editor2"},
		{ID: "a3", ContentID: "c2", Approver: "editor1"},
	}

	approvals := m.GetApprovalHistory("c1")
	if len(approvals) != 2 {
		t.Errorf("expected 2 approvals, got %d", len(approvals))
	}
}

func TestWorkflowLifecycle(t *testing.T) {
	m := NewManager()

	// Create content
	content := &Content{
		Title:  "Test Article",
		Type:   TypeDocument,
		Body:   "Article body",
		Author: "author1",
	}
	m.CreateContent(content)

	// Submit for review
	m.SubmitForReview(content.ID)

	// Approve
	m.ApproveContent(content.ID, "editor", "Good article")

	// Publish
	m.PublishContent(content.ID)

	// Verify final state
	got, _ := m.GetContent(content.ID)
	if got.Status != StatusPublished {
		t.Errorf("expected status '%s', got '%s'", StatusPublished, got.Status)
	}
	if got.PublishedAt == nil {
		t.Error("published_at not set")
	}

	// Check approval history
	approvals := m.GetApprovalHistory(content.ID)
	if len(approvals) != 1 {
		t.Errorf("expected 1 approval, got %d", len(approvals))
	}
}

func TestContentTypes(t *testing.T) {
	types := []ContentType{TypeDocument, TypeImage, TypeVideo, TypeAudio, TypePost}
	expected := []string{"document", "image", "video", "audio", "post"}

	for i, typ := range types {
		if string(typ) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(typ))
		}
	}
}

func TestWorkflowStatuses(t *testing.T) {
	statuses := []WorkflowStatus{StatusDraft, StatusReview, StatusApproved, StatusPublished, StatusArchived}
	expected := []string{"draft", "review", "approved", "published", "archived"}

	for i, s := range statuses {
		if string(s) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(s))
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			content := &Content{
				Title: "Concurrent Article",
				Type:  TypeDocument,
			}
			m.CreateContent(content)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	contents := m.ListContents("", "")
	if len(contents) != 10 {
		t.Errorf("expected 10 contents, got %d", len(contents))
	}
}

func TestTimestamps(t *testing.T) {
	m := NewManager()

	before := time.Now()
	content := &Content{
		Title: "Timestamped Article",
		Type:  TypeDocument,
	}
	m.CreateContent(content)
	after := time.Now()

	if content.CreatedAt.Before(before) || content.CreatedAt.After(after) {
		t.Error("created_at not set correctly")
	}
	if content.UpdatedAt.Before(before) || content.UpdatedAt.After(after) {
		t.Error("updated_at not set correctly")
	}
}

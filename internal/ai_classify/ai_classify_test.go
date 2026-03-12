package ai_classify

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifier_Classify(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "ai_classify_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFiles := map[string]string{
		"test_invoice.pdf":    "invoice content",
		"meeting_notes.txt":   "meeting notes from today",
		"project_report.docx": "project report document",
		"photo_2024.jpg":      "",
		"backup_data.zip":     "",
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		if content != "" {
			_ = os.WriteFile(path, []byte(content), 0644)
		} else {
			_ = os.WriteFile(path, []byte{0, 0, 0}, 0644)
		}
	}

	// 创建分类器
	config := DefaultConfig()
	config.DataDir = tmpDir

	classifier, err := NewClassifier(config)
	if err != nil {
		t.Fatalf("创建分类器失败: %v", err)
	}

	// 测试分类
	ctx := context.Background()

	// 测试发票文件
	invoicePath := filepath.Join(tmpDir, "test_invoice.pdf")
	result, err := classifier.Classify(ctx, invoicePath)
	if err != nil {
		t.Errorf("分类发票文件失败: %v", err)
	}
	t.Logf("发票文件分类: %s -> %s (confidence: %.2f)", invoicePath, result.Category.Name, result.Confidence)

	// 测试会议文件
	meetingPath := filepath.Join(tmpDir, "meeting_notes.txt")
	result, err = classifier.Classify(ctx, meetingPath)
	if err != nil {
		t.Errorf("分类会议文件失败: %v", err)
	}
	t.Logf("会议文件分类: %s -> %s (confidence: %.2f)", meetingPath, result.Category.Name, result.Confidence)

	// 检查标签
	if len(result.Tags) == 0 {
		t.Log("会议文件没有生成标签")
	} else {
		t.Logf("会议文件标签: %v", result.Tags)
	}
}

func TestSimilarityDetector(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "similarity_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建相同内容的文件
	content := []byte("This is test content for similarity detection")
	path1 := filepath.Join(tmpDir, "file1.txt")
	path2 := filepath.Join(tmpDir, "file2.txt")
	path3 := filepath.Join(tmpDir, "project_report_v1.txt")
	path4 := filepath.Join(tmpDir, "project_report_v2.txt")

	_ = os.WriteFile(path1, content, 0644)
	_ = os.WriteFile(path2, content, 0644) // 相同内容
	_ = os.WriteFile(path3, []byte("different content"), 0644)
	_ = os.WriteFile(path4, []byte("different content"), 0644)

	// 创建检测器
	config := DefaultConfig()
	config.DataDir = tmpDir

	detector := NewSimilarityDetector(config)

	// 索引文件
	_ = detector.IndexFile(path1)
	_ = detector.IndexFile(path2)
	_ = detector.IndexFile(path3)
	detector.IndexFile(path4)

	// 检测相似文件
	ctx := context.Background()
	similarities, err := detector.DetectSimilar(ctx, path1)
	if err != nil {
		t.Errorf("检测相似文件失败: %v", err)
	}

	t.Logf("找到 %d 个相似文件", len(similarities))
	for _, sim := range similarities {
		t.Logf("  %s 相似度: %.2f, 类型: %s", sim.FileB, sim.Score, sim.SimType)
	}

	// 查找重复文件
	duplicates, err := detector.FindDuplicates(ctx)
	if err != nil {
		t.Errorf("查找重复文件失败: %v", err)
	}

	t.Logf("找到 %d 组重复文件", len(duplicates))
	for i, group := range duplicates {
		t.Logf("  组 %d: %v", i+1, group)
	}
}

func TestLearner(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "learner_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建分类器和学习器
	config := DefaultConfig()
	config.DataDir = tmpDir

	classifier, _ := NewClassifier(config)
	tagger := NewTagger(config)
	learner := NewLearner(config, classifier, tagger)

	// 模拟用户反馈
	ctx := context.Background()
	feedback := UserFeedback{
		FilePath:           "/test/invoice_2024.pdf",
		OriginalCategoryID: "documents",
		CorrectCategoryID:  "invoices",
		Action:             "correct",
		Comment:            "这是发票，不是普通文档",
	}

	err = learner.LearnFromFeedback(ctx, feedback)
	if err != nil {
		t.Errorf("学习反馈失败: %v", err)
	}

	// 获取学习统计
	stats := learner.GetLearningStats()
	t.Logf("学习统计: %v", stats)

	// 获取规则建议
	suggestions, err := learner.SuggestRules(ctx)
	if err != nil {
		t.Errorf("获取规则建议失败: %v", err)
	}

	t.Logf("规则建议数量: %d", len(suggestions))
}

func TestBatchClassification(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "batch_test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建多个测试文件
	var paths []string
	for i := 0; i < 10; i++ {
		name := filepath.Join(tmpDir, "file_"+string(rune('a'+i))+".txt")
		os.WriteFile(name, []byte("test content"), 0644)
		paths = append(paths, name)
	}

	// 创建分类器
	config := DefaultConfig()
	config.DataDir = tmpDir

	classifier, _ := NewClassifier(config)

	// 批量分类
	ctx := context.Background()
	results, err := classifier.ClassifyBatch(ctx, paths, 4)
	if err != nil {
		t.Errorf("批量分类失败: %v", err)
	}

	t.Logf("批量分类完成: %d 个文件", len(results))
	for _, r := range results[:3] {
		t.Logf("  %s -> %s", r.FileName, r.Category.Name)
	}
}

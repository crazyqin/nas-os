package dataclassify

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:               true,
		AutoClassify:          true,
		AutoTag:               true,
		DetectPII:             true,
		ScanInterval:          24,
		MaxFileSize:           100 * 1024 * 1024,
		ConcurrentScans:       4,
		DefaultClassification: "internal",
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestClassifyFile(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	file := &ClassifiedFile{
		ID:             "file-1",
		Path:           "/docs/financial-report.xlsx",
		Name:           "financial-report.xlsx",
		Size:           1024 * 50,
		MimeType:       "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		DataType:       DataTypeFinancial,
		Classification: ClassConfidential,
		Sensitivity:    SensitivityHigh,
		Tags:           []string{"finance", "quarterly", "2026"},
		Keywords:       []string{"revenue", "profit", "expenses"},
		Confidence:     0.95,
	}

	if err := manager.ClassifyFile(file); err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	got, err := manager.GetFile("file-1")
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}

	if got.Classification != ClassConfidential {
		t.Errorf("Expected confidential, got %s", got.Classification)
	}
}

func TestPIIDetection(t *testing.T) {
	config := &Config{Enabled: true, DetectPII: true}
	manager := NewManager(config)

	file := &ClassifiedFile{
		ID:             "pii-file",
		Path:           "/personal/contacts.csv",
		Name:           "contacts.csv",
		DataType:       DataTypePersonal,
		Classification: ClassRestricted,
		Sensitivity:    SensitivityCritical,
		PIIDetected: []PIIDetection{
			{Type: PIIEmail, Value: "j***@example.com", Confidence: 0.99},
			{Type: PIIPhone, Value: "138****1234", Confidence: 0.95},
		},
	}

	manager.ClassifyFile(file)

	piiFiles := manager.SearchPII()
	if len(piiFiles) != 1 {
		t.Errorf("Expected 1 PII file, got %d", len(piiFiles))
	}
}

func TestSearchByClassification(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	manager.ClassifyFile(&ClassifiedFile{
		ID:             "pub-1",
		Classification: ClassPublic,
		DataType:       DataTypeDocument,
		Sensitivity:    SensitivityLow,
	})
	manager.ClassifyFile(&ClassifiedFile{
		ID:             "sec-1",
		Classification: ClassRestricted,
		DataType:       DataTypeDocument,
		Sensitivity:    SensitivityCritical,
	})

	publicFiles := manager.SearchByClassification(ClassPublic)
	if len(publicFiles) != 1 {
		t.Errorf("Expected 1 public file, got %d", len(publicFiles))
	}

	restrictedFiles := manager.SearchByClassification(ClassRestricted)
	if len(restrictedFiles) != 1 {
		t.Errorf("Expected 1 restricted file, got %d", len(restrictedFiles))
	}
}

func TestSearchByTag(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	manager.ClassifyFile(&ClassifiedFile{
		ID:       "tagged-1",
		Tags:     []string{"important", "review"},
		DataType: DataTypeDocument,
	})
	manager.ClassifyFile(&ClassifiedFile{
		ID:       "tagged-2",
		Tags:     []string{"archive"},
		DataType: DataTypeDocument,
	})

	results := manager.SearchByTag("important")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'important', got %d", len(results))
	}
}

func TestClassificationStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	manager.ClassifyFile(&ClassifiedFile{
		ID:             "stat-1",
		Classification: ClassPublic,
		DataType:       DataTypeDocument,
		Sensitivity:    SensitivityLow,
	})
	manager.ClassifyFile(&ClassifiedFile{
		ID:             "stat-2",
		Classification: ClassConfidential,
		DataType:       DataTypeFinancial,
		Sensitivity:    SensitivityHigh,
	})

	stats := manager.GetStats()
	if stats.TotalFiles != 2 {
		t.Errorf("Expected 2 total files, got %d", stats.TotalFiles)
	}
}

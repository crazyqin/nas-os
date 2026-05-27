package batchrename

import (
	"testing"
)

func TestPreviewRegex(t *testing.T) {
	mgr := NewManager()

	files := []string{
		"/photos/IMG_001.jpg",
		"/photos/IMG_002.jpg",
		"/photos/IMG_003.jpg",
	}

	previews := mgr.Preview(files, RenameRule{
		Mode:    ModeRegex,
		Pattern: `IMG_(\d+)`,
		Replace: "photo_$1",
	})

	for _, p := range previews {
		if !p.Changed {
			t.Errorf("expected change for %s", p.Original)
		}
	}
}

func TestPreviewSequence(t *testing.T) {
	mgr := NewManager()

	files := []string{
		"/photos/a.jpg",
		"/photos/b.jpg",
		"/photos/c.jpg",
	}

	previews := mgr.Preview(files, RenameRule{
		Mode:     ModeSequence,
		StartNum: 1,
		PadWidth: 3,
		Replace:  "vacation_#",
	})

	if previews[0].Renamed == "" {
		t.Error("expected renamed value")
	}
}

func TestPreviewCase(t *testing.T) {
	mgr := NewManager()

	files := []string{"/docs/MyFile.TXT"}

	previews := mgr.Preview(files, RenameRule{
		Mode:     ModeCase,
		CaseType: CaseLower,
	})

	if !previews[0].Changed {
		t.Error("expected case change")
	}
}

func TestPreviewPrefix(t *testing.T) {
	mgr := NewManager()

	files := []string{"/docs/report.pdf"}

	previews := mgr.Preview(files, RenameRule{
		Mode:   ModePrefix,
		Prefix: "2026_",
	})

	if previews[0].Changed {
		// prefix should change the name
		expected := "/docs/2026_report.pdf"
		if previews[0].Renamed != expected {
			t.Errorf("expected %s, got %s", expected, previews[0].Renamed)
		}
	}
}

func TestPreviewExtension(t *testing.T) {
	mgr := NewManager()

	files := []string{"/docs/file.txt"}

	previews := mgr.Preview(files, RenameRule{
		Mode:      ModeExtension,
		Extension: ".md",
	})

	if !previews[0].Changed {
		t.Error("expected extension change")
	}
}

func TestRename(t *testing.T) {
	mgr := NewManager()

	files := []string{
		"/photos/IMG_001.jpg",
		"/photos/IMG_002.jpg",
	}

	result := mgr.Rename(files, RenameRule{
		Mode:    ModeReplace,
		Pattern: "IMG",
		Replace: "photo",
	})

	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
}

func TestPreviewDate(t *testing.T) {
	mgr := NewManager()

	files := []string{"/docs/report.pdf"}

	previews := mgr.Preview(files, RenameRule{
		Mode:       ModeDate,
		DateFormat: "20060102",
	})

	if !previews[0].Changed {
		t.Error("expected date prefix")
	}
}

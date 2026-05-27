package photoeditor

import (
	"testing"
)

func TestApplyEditBasic(t *testing.T) {
	mgr := NewManager()

	result, err := mgr.ApplyEdit(EditRequest{
		ImagePath: "/photos/test.jpg",
		Filter:    filterPtr(FilterVintage),
	})
	if err != nil {
		t.Fatalf("ApplyEdit failed: %v", err)
	}

	if result.OriginalPath != "/photos/test.jpg" {
		t.Errorf("expected /photos/test.jpg, got %s", result.OriginalPath)
	}
	if len(result.EditHistory) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(result.EditHistory))
	}
}

func TestApplyEditUnsupportedFormat(t *testing.T) {
	mgr := NewManager()

	_, err := mgr.ApplyEdit(EditRequest{
		ImagePath: "/photos/test.gif",
	})
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestGetPresets(t *testing.T) {
	mgr := NewManager()

	presets := mgr.GetPresets()
	if len(presets) < 8 {
		t.Errorf("expected at least 8 presets, got %d", len(presets))
	}
}

func TestCreateAndDeletePreset(t *testing.T) {
	mgr := NewManager()

	created, err := mgr.CreatePreset(Preset{
		ID:   "mypreset",
		Name: "My Preset",
		Adjust: AdjustParams{
			Brightness: 20,
			Contrast:   10,
		},
	})
	if err != nil {
		t.Fatalf("CreatePreset failed: %v", err)
	}
	if !created.IsCustom {
		t.Error("expected custom preset")
	}

	// 删除
	if err := mgr.DeletePreset("mypreset"); err != nil {
		t.Fatalf("DeletePreset failed: %v", err)
	}

	// 不能再删除
	if err := mgr.DeletePreset("mypreset"); err == nil {
		t.Error("expected error deleting non-existent preset")
	}
}

func TestDeleteBuiltinPreset(t *testing.T) {
	mgr := NewManager()

	err := mgr.DeletePreset("vintage")
	if err == nil {
		t.Error("expected error deleting built-in preset")
	}
}

func filterPtr(f FilterType) *FilterType {
	return &f
}

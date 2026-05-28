package fancurve

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	if len(m.ListProfiles()) < 4 {
		t.Errorf("expected at least 4 default profiles, got %d", len(m.ListProfiles()))
	}
}

func TestListProfiles(t *testing.T) {
	m := NewManager()

	profiles := m.ListProfiles()
	if len(profiles) < 4 {
		t.Errorf("expected at least 4 profiles, got %d", len(profiles))
	}

	// 验证预设方案
	found := map[string]bool{}
	for _, p := range profiles {
		found[p.ID] = true
	}
	for _, id := range []string{"silent", "balanced", "performance", "fullspeed"} {
		if !found[id] {
			t.Errorf("expected preset profile '%s'", id)
		}
	}
}

func TestGetProfile(t *testing.T) {
	m := NewManager()

	profile := m.GetProfile("balanced")
	if profile == nil {
		t.Fatal("expected profile")
	}
	if profile.Name != "平衡" {
		t.Errorf("expected '平衡', got '%s'", profile.Name)
	}
	if !profile.IsDefault {
		t.Error("expected balanced to be default")
	}

	nilProfile := m.GetProfile("nonexistent")
	if nilProfile != nil {
		t.Error("expected nil for nonexistent profile")
	}
}

func TestCreateProfile(t *testing.T) {
	m := NewManager()

	profile := &CurveProfile{
		ID:          "custom-1",
		Name:        "自定义方案",
		Description: "测试方案",
		Points: []CurvePoint{
			{Temp: 30, Duty: 10},
			{Temp: 50, Duty: 50},
			{Temp: 70, Duty: 90},
		},
	}

	err := m.CreateProfile(profile)
	if err != nil {
		t.Fatalf("create profile failed: %v", err)
	}

	got := m.GetProfile("custom-1")
	if got == nil {
		t.Fatal("expected profile to exist")
	}
	if got.Name != "自定义方案" {
		t.Errorf("expected '自定义方案', got '%s'", got.Name)
	}

	// 重复创建
	err = m.CreateProfile(profile)
	if err == nil {
		t.Error("expected error for duplicate profile")
	}

	// 空 ID
	err = m.CreateProfile(&CurveProfile{ID: ""})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdateProfile(t *testing.T) {
	m := NewManager()

	profile := m.GetProfile("silent")
	if profile == nil {
		t.Fatal("expected profile")
	}

	profile.Description = "更新后的描述"
	err := m.UpdateProfile(profile)
	if err != nil {
		t.Fatalf("update profile failed: %v", err)
	}

	got := m.GetProfile("silent")
	if got.Description != "更新后的描述" {
		t.Errorf("expected '更新后的描述', got '%s'", got.Description)
	}

	// 不存在的方案
	err = m.UpdateProfile(&CurveProfile{ID: "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestDeleteProfile(t *testing.T) {
	m := NewManager()

	// 创建一个可删除的方案
	m.CreateProfile(&CurveProfile{
		ID:   "to-delete",
		Name: "待删除",
		Points: []CurvePoint{
			{Temp: 30, Duty: 10},
		},
	})

	err := m.DeleteProfile("to-delete")
	if err != nil {
		t.Fatalf("delete profile failed: %v", err)
	}

	if m.GetProfile("to-delete") != nil {
		t.Error("expected profile to be deleted")
	}

	// 不能删除默认方案
	err = m.DeleteProfile("balanced")
	if err == nil {
		t.Error("expected error when deleting default profile")
	}

	// 不能删除正在使用的方案（balanced 被所有通道使用）
	err = m.DeleteProfile("balanced")
	if err == nil {
		t.Error("expected error when deleting in-use profile")
	}
}

func TestListChannels(t *testing.T) {
	m := NewManager()

	channels := m.ListChannels()
	if len(channels) < 3 {
		t.Errorf("expected at least 3 channels, got %d", len(channels))
	}
}

func TestListTempSources(t *testing.T) {
	m := NewManager()

	sources := m.ListTempSources()
	if len(sources) < 4 {
		t.Errorf("expected at least 4 temp sources, got %d", len(sources))
	}
}

func TestApplyProfile(t *testing.T) {
	m := NewManager()

	err := m.ApplyProfile("cpu-fan", "performance")
	if err != nil {
		t.Fatalf("apply profile failed: %v", err)
	}

	profile := m.GetActiveProfile("cpu-fan")
	if profile == nil {
		t.Fatal("expected active profile")
	}
	if profile.ID != "performance" {
		t.Errorf("expected 'performance', got '%s'", profile.ID)
	}

	// 不存在的通道
	err = m.ApplyProfile("nonexistent", "balanced")
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}

	// 不存在的方案
	err = m.ApplyProfile("cpu-fan", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

func TestGetActiveProfile(t *testing.T) {
	m := NewManager()

	profile := m.GetActiveProfile("cpu-fan")
	if profile == nil {
		t.Fatal("expected active profile")
	}
	if profile.ID != "balanced" {
		t.Errorf("expected 'balanced', got '%s'", profile.ID)
	}

	nilProfile := m.GetActiveProfile("nonexistent")
	if nilProfile != nil {
		t.Error("expected nil for nonexistent channel")
	}
}

func TestSetHysteresis(t *testing.T) {
	m := NewManager()

	config := &HysteresisConfig{
		TempDelta:     3.0,
		ResponseDelay: 10.0,
	}

	err := m.SetHysteresis(config)
	if err != nil {
		t.Fatalf("set hysteresis failed: %v", err)
	}

	// 验证配置已更新
	if m.hysteresis.TempDelta != 3.0 {
		t.Errorf("expected 3.0, got %.1f", m.hysteresis.TempDelta)
	}

	// 无效配置
	err = m.SetHysteresis(&HysteresisConfig{TempDelta: -1})
	if err == nil {
		t.Error("expected error for negative delta")
	}

	err = m.SetHysteresis(&HysteresisConfig{ResponseDelay: -1})
	if err == nil {
		t.Error("expected error for negative delay")
	}
}

func TestSetSmoothing(t *testing.T) {
	m := NewManager()

	config := &SmoothingConfig{
		WindowSize: 10,
		Algorithm:  "exponential",
	}

	err := m.SetSmoothing(config)
	if err != nil {
		t.Fatalf("set smoothing failed: %v", err)
	}

	if m.smoothing.WindowSize != 10 {
		t.Errorf("expected 10, got %d", m.smoothing.WindowSize)
	}
	if m.smoothing.Algorithm != "exponential" {
		t.Errorf("expected 'exponential', got '%s'", m.smoothing.Algorithm)
	}

	// 无效配置
	err = m.SetSmoothing(&SmoothingConfig{WindowSize: 0})
	if err == nil {
		t.Error("expected error for zero window size")
	}

	err = m.SetSmoothing(&SmoothingConfig{WindowSize: 5, Algorithm: "invalid"})
	if err == nil {
		t.Error("expected error for invalid algorithm")
	}
}

func TestSetWeightedSensor(t *testing.T) {
	m := NewManager()

	sensors := []WeightedSensor{
		{SensorID: "cpu", Weight: 0.7},
		{SensorID: "mb", Weight: 0.3},
	}

	err := m.SetWeightedSensor("cpu-fan", sensors)
	if err != nil {
		t.Fatalf("set weighted sensor failed: %v", err)
	}

	// 不存在的通道
	err = m.SetWeightedSensor("nonexistent", sensors)
	if err == nil {
		t.Error("expected error for nonexistent channel")
	}

	// 不存在的传感器
	err = m.SetWeightedSensor("cpu-fan", []WeightedSensor{{SensorID: "nonexistent", Weight: 1}})
	if err == nil {
		t.Error("expected error for nonexistent sensor")
	}
}

func TestUpdateTemperature(t *testing.T) {
	m := NewManager()

	err := m.UpdateTemperature("cpu", 60.0)
	if err != nil {
		t.Fatalf("update temperature failed: %v", err)
	}

	sources := m.ListTempSources()
	for _, s := range sources {
		if s.ID == "cpu" && s.CurrentTemp != 60.0 {
			t.Errorf("expected 60.0, got %.1f", s.CurrentTemp)
		}
	}

	// 不存在的传感器
	err = m.UpdateTemperature("nonexistent", 50.0)
	if err == nil {
		t.Error("expected error for nonexistent sensor")
	}
}

func TestGetHistory(t *testing.T) {
	m := NewManager()

	// 初始历史为空
	history := m.GetHistory("cpu-fan", time.Hour)
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}

	// 更新温度后应有历史
	m.UpdateTemperature("cpu", 50.0)
	m.UpdateTemperature("cpu", 60.0)
	m.UpdateTemperature("cpu", 70.0)

	history = m.GetHistory("cpu-fan", time.Hour)
	if len(history) == 0 {
		t.Error("expected history after temperature update")
	}
}

func TestExportImportProfile(t *testing.T) {
	m := NewManager()

	// 导出
	data, err := m.ExportProfile("silent")
	if err != nil {
		t.Fatalf("export profile failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty export data")
	}

	// 验证 JSON 格式
	var profile CurveProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// 导入（需要新 ID）
	var imported CurveProfile
	json.Unmarshal(data, &imported)
	imported.ID = "imported-silent"
	imported.Name = "导入的静音"
	data2, _ := json.Marshal(imported)

	result, err := m.ImportProfile(data2)
	if err != nil {
		t.Fatalf("import profile failed: %v", err)
	}

	if result.ID != "imported-silent" {
		t.Errorf("expected 'imported-silent', got '%s'", result.ID)
	}

	// 导入重复 ID
	_, err = m.ImportProfile(data2)
	if err == nil {
		t.Error("expected error for duplicate import")
	}

	// 无效数据
	_, err = m.ImportProfile([]byte("invalid"))
	if err == nil {
		t.Error("expected error for invalid data")
	}

	// 空 ID
	_, err = m.ImportProfile([]byte(`{"name":"test"}`))
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestGetCurvePreviewData(t *testing.T) {
	m := NewManager()

	preview, err := m.GetCurvePreviewData("balanced")
	if err != nil {
		t.Fatalf("get preview failed: %v", err)
	}

	if len(preview) == 0 {
		t.Fatal("expected preview data")
	}

	// 验证温度范围
	if preview[0].Temp != 20 {
		t.Errorf("expected start at 20°C, got %.0f", preview[0].Temp)
	}

	// 不存在的方案
	_, err = m.GetCurvePreviewData("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent profile")
	}
}

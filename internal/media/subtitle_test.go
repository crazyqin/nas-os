package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSubtitleManager(t *testing.T) {
	sm := NewSubtitleManager()
	if sm == nil {
		t.Fatal("NewSubtitleManager returned nil")
	}
}

func TestSubtitleFormat_Constants(t *testing.T) {
	if SubtitleFormatSRT != "srt" {
		t.Errorf("SubtitleFormatSRT = %s, want srt", SubtitleFormatSRT)
	}
	if SubtitleFormatVTT != "vtt" {
		t.Errorf("SubtitleFormatVTT = %s, want vtt", SubtitleFormatVTT)
	}
	if SubtitleFormatASS != "ass" {
		t.Errorf("SubtitleFormatASS = %s, want ass", SubtitleFormatASS)
	}
}

func TestSubtitleItem_Fields(t *testing.T) {
	item := SubtitleItem{
		Index:     1,
		StartTime: 5 * time.Second,
		EndTime:   10 * time.Second,
		Text:      "Hello, world!",
		Style:     "Default",
		Name:      "Speaker1",
	}

	if item.Index != 1 {
		t.Errorf("Index = %d, want 1", item.Index)
	}

	if item.Text != "Hello, world!" {
		t.Errorf("Text = %s, want Hello, world!", item.Text)
	}
}

func TestSubtitle_Fields(t *testing.T) {
	sub := Subtitle{
		Format:   SubtitleFormatSRT,
		Language: "en",
		Title:    "Test Subtitle",
		Items: []SubtitleItem{
			{Index: 1, Text: "First"},
			{Index: 2, Text: "Second"},
		},
	}

	if sub.Format != SubtitleFormatSRT {
		t.Errorf("Format = %s, want srt", sub.Format)
	}

	if len(sub.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(sub.Items))
	}
}

func TestSubtitleManager_ParseSRT(t *testing.T) {
	sm := NewSubtitleManager()

	srtContent := `1
00:00:01,000 --> 00:00:04,000
Hello, world!

2
00:00:05,000 --> 00:00:08,000
This is a test.

`

	reader := strings.NewReader(srtContent)
	subtitle, err := sm.ParseSRT(reader)
	if err != nil {
		t.Fatalf("ParseSRT returned error: %v", err)
	}

	if subtitle.Format != SubtitleFormatSRT {
		t.Errorf("Format = %s, want srt", subtitle.Format)
	}

	if len(subtitle.Items) != 2 {
		t.Fatalf("Items count = %d, want 2", len(subtitle.Items))
	}

	if subtitle.Items[0].Text != "Hello, world!" {
		t.Errorf("Item 0 Text = %s, want Hello, world!", subtitle.Items[0].Text)
	}

	if subtitle.Items[1].Text != "This is a test." {
		t.Errorf("Item 1 Text = %s, want This is a test.", subtitle.Items[1].Text)
	}
}

func TestSubtitleManager_ParseSRT_TimeParsing(t *testing.T) {
	sm := NewSubtitleManager()

	srtContent := `1
00:01:30,500 --> 00:02:45,750
Test timing

`

	reader := strings.NewReader(srtContent)
	subtitle, err := sm.ParseSRT(reader)
	if err != nil {
		t.Fatalf("ParseSRT returned error: %v", err)
	}

	expectedStart := 90*time.Second + 500*time.Millisecond
	expectedEnd := 165*time.Second + 750*time.Millisecond

	if subtitle.Items[0].StartTime != expectedStart {
		t.Errorf("StartTime = %v, want %v", subtitle.Items[0].StartTime, expectedStart)
	}

	if subtitle.Items[0].EndTime != expectedEnd {
		t.Errorf("EndTime = %v, want %v", subtitle.Items[0].EndTime, expectedEnd)
	}
}

func TestSubtitleManager_ParseVTT(t *testing.T) {
	sm := NewSubtitleManager()

	vttContent := `WEBVTT

00:00:01.000 --> 00:00:04.000
Hello, world!

00:00:05.000 --> 00:00:08.000
This is a test.

`

	reader := strings.NewReader(vttContent)
	subtitle, err := sm.ParseVTT(reader)
	if err != nil {
		t.Fatalf("ParseVTT returned error: %v", err)
	}

	if subtitle.Format != SubtitleFormatVTT {
		t.Errorf("Format = %s, want vtt", subtitle.Format)
	}

	if len(subtitle.Items) != 2 {
		t.Fatalf("Items count = %d, want 2", len(subtitle.Items))
	}
}

func TestSubtitleManager_ParseVTT_ShortFormat(t *testing.T) {
	sm := NewSubtitleManager()

	// VTT supports short time format MM:SS.mmm
	vttContent := `WEBVTT

00:30.000 --> 01:00.000
Short format test

`

	reader := strings.NewReader(vttContent)
	subtitle, err := sm.ParseVTT(reader)
	if err != nil {
		t.Fatalf("ParseVTT returned error: %v", err)
	}

	if len(subtitle.Items) != 1 {
		t.Fatalf("Items count = %d, want 1", len(subtitle.Items))
	}

	expectedStart := 30 * time.Second
	if subtitle.Items[0].StartTime != expectedStart {
		t.Errorf("StartTime = %v, want %v", subtitle.Items[0].StartTime, expectedStart)
	}
}

func TestSubtitleManager_ParseASS(t *testing.T) {
	sm := NewSubtitleManager()

	assContent := `[Script Info]
Title: Test ASS
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic
Style: Default, Arial, 20, &H00FFFFFF, &H000000FF, &H00000000, &H00000000, 0, 0

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello, world!
Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,This is a test.
`

	reader := strings.NewReader(assContent)
	subtitle, err := sm.ParseASS(reader)
	if err != nil {
		t.Fatalf("ParseASS returned error: %v", err)
	}

	if subtitle.Format != SubtitleFormatASS {
		t.Errorf("Format = %s, want ass", subtitle.Format)
	}

	if len(subtitle.Items) != 2 {
		t.Fatalf("Items count = %d, want 2", len(subtitle.Items))
	}

	if subtitle.Title != "Test ASS" {
		t.Errorf("Title = %s, want Test ASS", subtitle.Title)
	}
}

func TestSubtitleManager_GenerateSRT(t *testing.T) {
	sm := NewSubtitleManager()

	subtitle := &Subtitle{
		Format: SubtitleFormatSRT,
		Items: []SubtitleItem{
			{
				Index:     1,
				StartTime: time.Second,
				EndTime:   4 * time.Second,
				Text:      "Hello, world!",
			},
			{
				Index:     2,
				StartTime: 5 * time.Second,
				EndTime:   8 * time.Second,
				Text:      "This is a test.",
			},
		},
	}

	output := sm.GenerateSRT(subtitle)

	if !strings.Contains(output, "1") {
		t.Error("Generated SRT should contain index 1")
	}

	if !strings.Contains(output, "00:00:01,000 --> 00:00:04,000") {
		t.Error("Generated SRT should contain time line")
	}

	if !strings.Contains(output, "Hello, world!") {
		t.Error("Generated SRT should contain subtitle text")
	}
}

func TestSubtitleManager_GenerateVTT(t *testing.T) {
	sm := NewSubtitleManager()

	subtitle := &Subtitle{
		Format: SubtitleFormatVTT,
		Items: []SubtitleItem{
			{
				Index:     1,
				StartTime: time.Second,
				EndTime:   4 * time.Second,
				Text:      "Hello, world!",
			},
		},
	}

	output := sm.GenerateVTT(subtitle)

	if !strings.HasPrefix(output, "WEBVTT") {
		t.Error("Generated VTT should start with WEBVTT")
	}

	// VTT uses short format for times < 1 hour
	if !strings.Contains(output, "00:01.000 --> 00:04.000") && !strings.Contains(output, "00:00:01.000 --> 00:00:04.000") {
		t.Error("Generated VTT should contain time line")
	}
}

func TestSubtitleManager_SaveAndParse(t *testing.T) {
	sm := NewSubtitleManager()

	tmpDir := t.TempDir()
	srtPath := filepath.Join(tmpDir, "test.srt")

	subtitle := &Subtitle{
		Format: SubtitleFormatSRT,
		Items: []SubtitleItem{
			{
				Index:     1,
				StartTime: time.Second,
				EndTime:   4 * time.Second,
				Text:      "Test subtitle",
			},
		},
	}

	// Save
	err := sm.SaveSubtitle(subtitle, srtPath)
	if err != nil {
		t.Fatalf("SaveSubtitle returned error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(srtPath); os.IsNotExist(err) {
		t.Error("Subtitle file was not created")
	}

	// Parse it back
	parsed, err := sm.ParseSubtitle(srtPath)
	if err != nil {
		t.Fatalf("ParseSubtitle returned error: %v", err)
	}

	if len(parsed.Items) != 1 {
		t.Errorf("Parsed items count = %d, want 1", len(parsed.Items))
	}

	if parsed.Items[0].Text != "Test subtitle" {
		t.Errorf("Parsed text = %s, want Test subtitle", parsed.Items[0].Text)
	}
}

func TestSubtitleManager_ConvertSubtitle(t *testing.T) {
	sm := NewSubtitleManager()

	tmpDir := t.TempDir()
	srtPath := filepath.Join(tmpDir, "test.srt")
	vttPath := filepath.Join(tmpDir, "test.vtt")

	// Create SRT file
	srtContent := `1
00:00:01,000 --> 00:00:04,000
Test subtitle
`
	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		t.Fatalf("Failed to create SRT file: %v", err)
	}

	// Convert to VTT
	err := sm.ConvertSubtitle(srtPath, vttPath)
	if err != nil {
		t.Fatalf("ConvertSubtitle returned error: %v", err)
	}

	// Verify VTT file exists
	if _, err := os.Stat(vttPath); os.IsNotExist(err) {
		t.Error("VTT file was not created")
	}

	// Verify content
	content, err := os.ReadFile(vttPath)
	if err != nil {
		t.Fatalf("Failed to read VTT file: %v", err)
	}

	if !strings.HasPrefix(string(content), "WEBVTT") {
		t.Error("Converted file should start with WEBVTT")
	}
}

func TestSubtitleManager_ShiftTime(t *testing.T) {
	sm := NewSubtitleManager()

	subtitle := &Subtitle{
		Items: []SubtitleItem{
			{
				StartTime: 10 * time.Second,
				EndTime:   15 * time.Second,
			},
		},
	}

	// Shift forward by 5 seconds
	sm.ShiftTime(subtitle, 5*time.Second)

	if subtitle.Items[0].StartTime != 15*time.Second {
		t.Errorf("StartTime = %v, want 15s", subtitle.Items[0].StartTime)
	}

	if subtitle.Items[0].EndTime != 20*time.Second {
		t.Errorf("EndTime = %v, want 20s", subtitle.Items[0].EndTime)
	}
}

func TestSubtitleManager_ShiftTime_Negative(t *testing.T) {
	sm := NewSubtitleManager()

	subtitle := &Subtitle{
		Items: []SubtitleItem{
			{
				StartTime: 10 * time.Second,
				EndTime:   15 * time.Second,
			},
		},
	}

	// Shift backward by 5 seconds
	sm.ShiftTime(subtitle, -5*time.Second)

	if subtitle.Items[0].StartTime != 5*time.Second {
		t.Errorf("StartTime = %v, want 5s", subtitle.Items[0].StartTime)
	}

	if subtitle.Items[0].EndTime != 10*time.Second {
		t.Errorf("EndTime = %v, want 10s", subtitle.Items[0].EndTime)
	}
}

func TestSubtitleManager_ShiftTime_NoNegative(t *testing.T) {
	sm := NewSubtitleManager()

	subtitle := &Subtitle{
		Items: []SubtitleItem{
			{
				StartTime: 2 * time.Second,
				EndTime:   5 * time.Second,
			},
		},
	}

	// Shift backward by 10 seconds - should not go negative
	sm.ShiftTime(subtitle, -10*time.Second)

	if subtitle.Items[0].StartTime < 0 {
		t.Errorf("StartTime should not be negative, got %v", subtitle.Items[0].StartTime)
	}

	if subtitle.Items[0].EndTime < 0 {
		t.Errorf("EndTime should not be negative, got %v", subtitle.Items[0].EndTime)
	}
}

func TestParseSRTTime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"00:00:01,000", time.Second},
		{"00:01:30,500", 90*time.Second + 500*time.Millisecond},
		{"01:00:00,000", time.Hour},
		{"00:00:00,001", time.Millisecond},
	}

	for _, tt := range tests {
		result := parseSRTTime(tt.input)
		if result != tt.expected {
			t.Errorf("parseSRTTime(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseVTTTime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"00:00:01.000", time.Second},
		{"00:01:30.500", 90*time.Second + 500*time.Millisecond},
		{"01:00:00.000", time.Hour},
		{"00:30.000", 30 * time.Second}, // Short format
		{"00:00.001", time.Millisecond},
	}

	for _, tt := range tests {
		result := parseVTTTime(tt.input)
		if result != tt.expected {
			t.Errorf("parseVTTTime(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseASSTime(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"0:00:01.00", time.Second},
		{"0:01:30.50", 90*time.Second + 500*time.Millisecond},
		{"1:00:00.00", time.Hour},
		{"0:00:00.01", 10 * time.Millisecond},
	}

	for _, tt := range tests {
		result := parseASSTime(tt.input)
		if result != tt.expected {
			t.Errorf("parseASSTime(%s) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestFormatSRTTime(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Second, "00:00:01,000"},
		{time.Hour, "01:00:00,000"},
		{90*time.Second + 500*time.Millisecond, "00:01:30,500"},
	}

	for _, tt := range tests {
		result := formatSRTTime(tt.input)
		if result != tt.expected {
			t.Errorf("formatSRTTime(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestFormatVTTTime(t *testing.T) {
	// VTT format uses short format for times < 1 hour
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{time.Second, "00:01.000"},                        // Short format for < 1 hour
		{time.Hour, "01:00:00.000"},                       // Full format for >= 1 hour
		{30 * time.Second, "00:30.000"},                   // Short format
		{90 * time.Second, "01:30.000"},                   // Short format (1.5 minutes)
		{90*time.Minute + 30*time.Second, "01:30:30.000"}, // Full format (over 1 hour)
	}

	for _, tt := range tests {
		result := formatVTTTime(tt.input)
		if result != tt.expected {
			t.Errorf("formatVTTTime(%v) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestCleanASSText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Plain text", "Plain text"},
		{"{\an8}Formatted text", "Formatted text"},
		{"Line1\\NLine2", "Line1\nLine2"},
		{"{\\b1}Bold{\\b0}", "Bold"},
	}

	for _, tt := range tests {
		result := cleanASSText(tt.input)
		if result != tt.expected {
			t.Errorf("cleanASSText(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestSubtitleManager_ParseSubtitle_UnsupportedFormat(t *testing.T) {
	sm := NewSubtitleManager()

	tmpDir := t.TempDir()
	unsupportedPath := filepath.Join(tmpDir, "test.xyz")

	if err := os.WriteFile(unsupportedPath, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	_, err := sm.ParseSubtitle(unsupportedPath)
	if err == nil {
		t.Error("ParseSubtitle should return error for unsupported format")
	}
}

func TestASSStyle_Fields(t *testing.T) {
	style := ASSStyle{
		Name:         "Default",
		FontName:     "Arial",
		FontSize:     20,
		PrimaryColor: "&H00FFFFFF",
		Bold:         true,
		Italic:       false,
		Alignment:    2,
	}

	if style.Name != "Default" {
		t.Errorf("Name = %s, want Default", style.Name)
	}

	if !style.Bold {
		t.Error("Bold should be true")
	}
}

func TestVTTRegion_Fields(t *testing.T) {
	region := VTTRegion{
		ID:        "region1",
		Width:     50,
		Height:    10,
		AnchorX:   0,
		AnchorY:   100,
		ViewportX: 0,
		ViewportY: 100,
		Scroll:    "up",
	}

	if region.ID != "region1" {
		t.Errorf("ID = %s, want region1", region.ID)
	}

	if region.Scroll != "up" {
		t.Errorf("Scroll = %s, want up", region.Scroll)
	}
}

func TestSubtitleManager_MergeSubtitles(t *testing.T) {
	sm := NewSubtitleManager()

	tmpDir := t.TempDir()

	// Create two SRT files
	srt1 := filepath.Join(tmpDir, "sub1.srt")
	srt2 := filepath.Join(tmpDir, "sub2.srt")

	content1 := `1
00:00:01,000 --> 00:00:03,000
First subtitle
`
	content2 := `1
00:00:04,000 --> 00:00:06,000
Second subtitle
`

	if err := os.WriteFile(srt1, []byte(content1), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	if err := os.WriteFile(srt2, []byte(content2), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Merge
	mergedPath := filepath.Join(tmpDir, "merged.srt")
	err := sm.MergeSubtitles([]string{srt1, srt2}, mergedPath)
	if err != nil {
		t.Fatalf("MergeSubtitles returned error: %v", err)
	}

	// Verify
	parsed, err := sm.ParseSubtitle(mergedPath)
	if err != nil {
		t.Fatalf("ParseSubtitle returned error: %v", err)
	}

	if len(parsed.Items) != 2 {
		t.Errorf("Merged items count = %d, want 2", len(parsed.Items))
	}
}

package locale

import (
	"testing"
	"time"
)

func newTestManager() *LocaleManager {
	m := NewManager("en")

	// Register built-in languages
	m.AddLanguage(Language{Code: "en", Name: "English", NativeName: "English", Direction: DirectionLTR})
	m.AddLanguage(Language{Code: "zh", Name: "Chinese", NativeName: "中文", Direction: DirectionLTR})
	m.AddLanguage(Language{Code: "ja", Name: "Japanese", NativeName: "日本語", Direction: DirectionLTR})
	m.AddLanguage(Language{Code: "ar", Name: "Arabic", NativeName: "العربية", Direction: DirectionRTL})

	// Load English translations
	m.LoadTranslations("en", map[string]string{
		"greeting":        "Hello, {0}!",
		"menu.home":       "Home",
		"menu.settings":   "Settings",
		"items.count":     "You have {0} items",
		"welcome":         "Welcome",
		"goodbye":         "Goodbye",
		"error.not_found": "Not found",
	})

	// Load Chinese translations
	m.LoadTranslations("zh", map[string]string{
		"greeting":        "你好，{0}！",
		"menu.home":       "首页",
		"menu.settings":   "设置",
		"items.count":     "你有{0}个项目",
		"welcome":         "欢迎",
		"goodbye":         "再见",
		"error.not_found": "未找到",
	})

	// Load Japanese translations
	m.LoadTranslations("ja", map[string]string{
		"greeting":   "こんにちは、{0}！",
		"menu.home":  "ホーム",
		"welcome":    "ようこそ",
	})

	// Load Arabic translations
	m.LoadTranslations("ar", map[string]string{
		"greeting":   "مرحبا {0}",
		"menu.home":  "الصفحة الرئيسية",
		"welcome":    "مرحبا بكم",
	})

	return m
}

func TestNewManager(t *testing.T) {
	m := NewManager("en")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.GetLanguage() != "en" {
		t.Errorf("default language = %q, want %q", m.GetLanguage(), "en")
	}
}

func TestAddLanguage(t *testing.T) {
	m := newTestManager()

	langs := m.GetAvailableLanguages()
	if len(langs) != 4 {
		t.Errorf("expected 4 languages, got %d", len(langs))
	}

	// Should be sorted by code
	expectedCodes := []string{"ar", "en", "ja", "zh"}
	for i, l := range langs {
		if l.Code != expectedCodes[i] {
			t.Errorf("lang[%d].Code = %q, want %q", i, l.Code, expectedCodes[i])
		}
	}
}

func TestSetLanguage(t *testing.T) {
	m := newTestManager()

	if err := m.SetLanguage("zh"); err != nil {
		t.Fatalf("SetLanguage failed: %v", err)
	}
	if m.GetLanguage() != "zh" {
		t.Errorf("language = %q, want %q", m.GetLanguage(), "zh")
	}
}

func TestTranslateBasic(t *testing.T) {
	m := newTestManager()

	tests := []struct {
		lang string
		key  string
		want string
	}{
		{"en", "welcome", "Welcome"},
		{"zh", "welcome", "欢迎"},
		{"ja", "welcome", "ようこそ"},
		{"ar", "welcome", "مرحبا بكم"},
	}

	for _, tt := range tests {
		m.SetLanguage(tt.lang)
		got := m.Translate(tt.key)
		if got != tt.want {
			t.Errorf("[%s] Translate(%q) = %q, want %q", tt.lang, tt.key, got, tt.want)
		}
	}
}

func TestTranslateWithArgs(t *testing.T) {
	m := newTestManager()

	m.SetLanguage("en")
	got := m.Translate("greeting", "World")
	if got != "Hello, World!" {
		t.Errorf("Translate greeting = %q, want %q", got, "Hello, World!")
	}

	m.SetLanguage("zh")
	got = m.Translate("greeting", "世界")
	if got != "你好，世界！" {
		t.Errorf("Translate greeting = %q, want %q", got, "你好，世界！")
	}
}

func TestTranslateMultipleArgs(t *testing.T) {
	m := NewManager("en")
	m.LoadTranslations("en", map[string]string{
		"msg": "{0} bought {1} items on {2}",
	})

	got := m.T("msg", "Alice", 5, "Monday")
	if got != "Alice bought 5 items on Monday" {
		t.Errorf("T(msg) = %q, want %q", got, "Alice bought 5 items on Monday")
	}
}

func TestTranslateMissingKey(t *testing.T) {
	m := newTestManager()
	m.SetLanguage("en")

	got := m.Translate("nonexistent.key")
	if got != "nonexistent.key" {
		t.Errorf("missing key should return key itself, got %q", got)
	}
}

func TestTranslateFallbackToDefault(t *testing.T) {
	m := newTestManager()

	// ja doesn't have "goodbye", should fall back to en
	m.SetLanguage("ja")
	got := m.T("goodbye")
	if got != "Goodbye" {
		t.Errorf("fallback T(goodbye) = %q, want %q", got, "Goodbye")
	}
}

func TestTranslateShorthand(t *testing.T) {
	m := newTestManager()
	m.SetLanguage("en")

	if m.T("welcome") != m.Translate("welcome") {
		t.Error("T() and Translate() should return same result")
	}
}

func TestTranslatePlural(t *testing.T) {
	m := NewManager("en")

	m.LoadTranslationsWithPlural("en", map[string]*TranslationEntry{
		"item": {
			Key:   "item",
			Value: "",
			Plural: &PluralForm{
				One:   "{0} item",
				Other: "{0} items",
			},
		},
		"cat": {
			Key:   "cat",
			Value: "",
			Plural: &PluralForm{
				One:   "{0} cat",
				Other: "{0} cats",
			},
		},
	})

	tests := []struct {
		count int
		want  string
	}{
		{0, "0 items"},
		{1, "1 item"},
		{5, "5 items"},
	}

	for _, tt := range tests {
		got := m.TranslatePlural("item", tt.count, tt.count)
		if got != tt.want {
			t.Errorf("TranslatePlural(item, %d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestTranslatePluralArabic(t *testing.T) {
	m := NewManager("ar")
	m.AddLanguage(Language{Code: "ar", Name: "Arabic", NativeName: "العربية", Direction: DirectionRTL})

	m.LoadTranslationsWithPlural("ar", map[string]*TranslationEntry{
		"file": {
			Key:   "file",
			Value: "",
			Plural: &PluralForm{
				Zero:  "لا ملفات",
				One:   "ملف واحد",
				Two:   "ملفان",
				Few:   "{0} ملفات",
				Many:  "{0} ملفاً",
				Other: "{0} ملف",
			},
		},
	})

	tests := []struct {
		count int
		want  string
	}{
		{0, "لا ملفات"},
		{1, "ملف واحد"},
		{2, "ملفان"},
		{5, "5 ملفات"},
		{15, "15 ملفاً"},
		{100, "100 ملف"},
	}

	for _, tt := range tests {
		got := m.TranslatePlural("file", tt.count, tt.count)
		if got != tt.want {
			t.Errorf("TranslatePlural(file, %d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestTranslatePluralChinese(t *testing.T) {
	m := NewManager("zh")
	m.LoadTranslationsWithPlural("zh", map[string]*TranslationEntry{
		"apple": {
			Key:   "apple",
			Value: "",
			Plural: &PluralForm{
				Other: "{0}个苹果",
			},
		},
	})

	got := m.TranslatePlural("apple", 3, 3)
	if got != "3个苹果" {
		t.Errorf("TranslatePlural(apple, 3) = %q, want %q", got, "3个苹果")
	}
}

func TestTranslatePluralFallback(t *testing.T) {
	m := NewManager("en")
	// No plural entry, should fallback to Translate
	m.LoadTranslations("en", map[string]string{
		"simple": "just text",
	})

	got := m.TranslatePlural("simple", 5)
	if got != "just text" {
		t.Errorf("TranslatePlural fallback = %q, want %q", got, "just text")
	}
}

func TestFormatDate(t *testing.T) {
	m := newTestManager()
	date := time.Date(2026, 5, 29, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		lang string
		want string
	}{
		{"en", "May 29, 2026"},
		{"zh", "2026年05月29日"},
		{"ja", "2026年05月29日"},
		{"ar", "29/05/2026"},
	}

	for _, tt := range tests {
		m.SetLanguage(tt.lang)
		got := m.FormatDate(date)
		if got != tt.want {
			t.Errorf("[%s] FormatDate = %q, want %q", tt.lang, got, tt.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	m := newTestManager()

	tests := []struct {
		lang   string
		number float64
		want   string
	}{
		{"en", 1234.56, "1,234.56"},
		{"zh", 1234.56, "1,234.56"},
		{"en", 1000000.00, "1,000,000"},
		{"en", 42, "42"},
		{"en", 0, "0"},
		{"en", -1234.56, "-1,234.56"},
	}

	for _, tt := range tests {
		m.SetLanguage(tt.lang)
		got := m.FormatNumber(tt.number)
		if got != tt.want {
			t.Errorf("[%s] FormatNumber(%v) = %q, want %q", tt.lang, tt.number, got, tt.want)
		}
	}
}

func TestIsRTL(t *testing.T) {
	m := newTestManager()

	m.SetLanguage("en")
	if m.IsRTL() {
		t.Error("en should not be RTL")
	}

	m.SetLanguage("ar")
	if !m.IsRTL() {
		t.Error("ar should be RTL")
	}

	m.SetLanguage("zh")
	if m.IsRTL() {
		t.Error("zh should not be RTL")
	}
}

func TestIsRTLUnknownLang(t *testing.T) {
	m := NewManager("xx")
	if m.IsRTL() {
		t.Error("unknown language should not be RTL")
	}
}

func TestExportTranslations(t *testing.T) {
	m := newTestManager()

	result, err := m.ExportTranslations("en")
	if err != nil {
		t.Fatalf("ExportTranslations failed: %v", err)
	}

	if result["greeting"] != "Hello, {0}!" {
		t.Errorf("exported greeting = %q", result["greeting"])
	}
	if result["menu.home"] != "Home" {
		t.Errorf("exported menu.home = %q", result["menu.home"])
	}
	if len(result) != 7 {
		t.Errorf("expected 7 entries, got %d", len(result))
	}
}

func TestExportTranslationsNotFound(t *testing.T) {
	m := newTestManager()

	_, err := m.ExportTranslations("fr")
	if err == nil {
		t.Error("expected error for unregistered language")
	}
}

func TestGetAvailableLanguages(t *testing.T) {
	m := newTestManager()

	langs := m.GetAvailableLanguages()
	if len(langs) == 0 {
		t.Error("should have available languages")
	}

	found := false
	for _, l := range langs {
		if l.Code == "zh" && l.NativeName == "中文" {
			found = true
		}
	}
	if !found {
		t.Error("Chinese language not found in available languages")
	}
}

func TestLoadTranslationsOverwrite(t *testing.T) {
	m := NewManager("en")

	m.LoadTranslations("en", map[string]string{
		"key1": "original",
	})

	m.LoadTranslations("en", map[string]string{
		"key1": "updated",
	})

	got := m.T("key1")
	if got != "updated" {
		t.Errorf("overwrite T(key1) = %q, want %q", got, "updated")
	}
}

func TestLoadTranslationsWithPlural(t *testing.T) {
	m := NewManager("en")

	entry := &TranslationEntry{
		Key:   "msg",
		Value: "message",
		Plural: &PluralForm{
			One:   "1 message",
			Other: "{0} messages",
		},
	}

	err := m.LoadTranslationsWithPlural("en", map[string]*TranslationEntry{
		"msg": entry,
	})
	if err != nil {
		t.Fatalf("LoadTranslationsWithPlural failed: %v", err)
	}

	got := m.TranslatePlural("msg", 3, 3)
	if got != "3 messages" {
		t.Errorf("TranslatePlural(msg, 3) = %q, want %q", got, "3 messages")
	}
}

func TestFormatNumberThousands(t *testing.T) {
	m := NewManager("en")

	got := m.FormatNumber(1234567.89)
	if got != "1,234,567.89" {
		t.Errorf("FormatNumber(1234567.89) = %q, want %q", got, "1,234,567.89")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := newTestManager()

	done := make(chan bool, 20)

	// Concurrent readers
	for i := 0; i < 10; i++ {
		go func() {
			m.T("welcome")
			m.GetLanguage()
			m.IsRTL()
			done <- true
		}()
	}

	// Concurrent writers
	for i := 0; i < 10; i++ {
		go func() {
			m.SetLanguage("zh")
			m.LoadTranslations("en", map[string]string{"new.key": "value"})
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestRegisterPluralFunc(t *testing.T) {
	m := NewManager("fr")
	m.AddLanguage(Language{Code: "fr", Name: "French", NativeName: "Français", Direction: DirectionLTR})

	m.RegisterPluralFunc("fr", func(n int) string {
		if n <= 1 {
			return "one"
		}
		return "other"
	})

	m.LoadTranslationsWithPlural("fr", map[string]*TranslationEntry{
		"book": {
			Key: "book",
			Plural: &PluralForm{
				One:   "{0} livre",
				Other: "{0} livres",
			},
		},
	})

	m.SetLanguage("fr")

	got := m.TranslatePlural("book", 1, 1)
	if got != "1 livre" {
		t.Errorf("TranslatePlural(book, 1) = %q, want %q", got, "1 livre")
	}

	got = m.TranslatePlural("book", 3, 3)
	if got != "3 livres" {
		t.Errorf("TranslatePlural(book, 3) = %q, want %q", got, "3 livres")
	}
}

func TestFormatMessageNoArgs(t *testing.T) {
	got := formatMessage("hello world")
	if got != "hello world" {
		t.Errorf("formatMessage(no args) = %q, want %q", got, "hello world")
	}
}

func TestFormatMessageInvalidIndex(t *testing.T) {
	got := formatMessage("{99} test", "a")
	if got != "{99} test" {
		t.Errorf("formatMessage(invalid idx) = %q, want %q", got, "{99} test")
	}
}

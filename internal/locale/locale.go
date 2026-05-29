package locale

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Direction represents text direction
type Direction string

const (
	DirectionLTR Direction = "ltr"
	DirectionRTL Direction = "rtl"
)

// Language represents a supported language
type Language struct {
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	NativeName string    `json:"native_name"`
	Direction  Direction `json:"direction"`
}

// PluralForm represents a plural form for a key
type PluralForm struct {
	Zero  string
	One   string
	Two   string
	Few   string
	Many  string
	Other string
}

// TranslationEntry represents a single translation entry
type TranslationEntry struct {
	Key    string      `json:"key"`
	Value  string      `json:"value"`
	Plural *PluralForm `json:"plural,omitempty"`
}

// LocaleManager manages translations and locale settings
type LocaleManager struct {
	mu              sync.RWMutex
	defaultLang     string
	currentLang     string
	languages       map[string]Language
	translations    map[string]map[string]*TranslationEntry // lang -> key -> entry
	pluralFuncs     map[string]func(int) string             // lang -> plural rule function
}

// NewManager creates a new LocaleManager with a default language
func NewManager(defaultLang string) *LocaleManager {
	m := &LocaleManager{
		defaultLang:  defaultLang,
		currentLang:  defaultLang,
		languages:    make(map[string]Language),
		translations: make(map[string]map[string]*TranslationEntry),
		pluralFuncs:  make(map[string]func(int) string),
	}
	m.registerDefaultPluralFuncs()
	return m
}

// registerDefaultPluralFuncs sets up built-in plural rules
func (m *LocaleManager) registerDefaultPluralFuncs() {
	// English: 1 = one, else other
	m.pluralFuncs["en"] = func(n int) string {
		if n == 1 {
			return "one"
		}
		return "other"
	}
	// Chinese/Japanese: no plural distinction
	m.pluralFuncs["zh"] = func(n int) string { return "other" }
	m.pluralFuncs["ja"] = func(n int) string { return "other" }
	// Arabic: complex plural rules
	m.pluralFuncs["ar"] = func(n int) string {
		switch {
		case n == 0:
			return "zero"
		case n == 1:
			return "one"
		case n == 2:
			return "two"
		case n%100 >= 3 && n%100 <= 10:
			return "few"
		case n%100 >= 11 && n%100 <= 99:
			return "many"
		default:
			return "other"
		}
	}
}

// AddLanguage adds a language to the manager
func (m *LocaleManager) AddLanguage(lang Language) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.languages[lang.Code] = lang
}

// GetAvailableLanguages returns all registered languages sorted by code
func (m *LocaleManager) GetAvailableLanguages() []Language {
	m.mu.RLock()
	defer m.mu.RUnlock()
	langs := make([]Language, 0, len(m.languages))
	for _, l := range m.languages {
		langs = append(langs, l)
	}
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].Code < langs[j].Code
	})
	return langs
}

// LoadTranslations loads translations for a given language
func (m *LocaleManager) LoadTranslations(lang string, entries map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.translations[lang] == nil {
		m.translations[lang] = make(map[string]*TranslationEntry)
	}
	for key, value := range entries {
		m.translations[lang][key] = &TranslationEntry{
			Key:   key,
			Value: value,
		}
	}
	return nil
}

// LoadTranslationsWithPlural loads translations including plural forms
func (m *LocaleManager) LoadTranslationsWithPlural(lang string, entries map[string]*TranslationEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.translations[lang] == nil {
		m.translations[lang] = make(map[string]*TranslationEntry)
	}
	for _, entry := range entries {
		m.translations[lang][entry.Key] = entry
	}
	return nil
}

// SetLanguage sets the current language
func (m *LocaleManager) SetLanguage(lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentLang = lang
	return nil
}

// GetLanguage returns the current language code
func (m *LocaleManager) GetLanguage() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLang
}

// resolveKey looks up a translation by key, falling back to default language
func (m *LocaleManager) resolveKey(lang, key string) (*TranslationEntry, bool) {
	// Try current language
	if entries, ok := m.translations[lang]; ok {
		if entry, found := entries[key]; found {
			return entry, true
		}
	}
	// Fallback to default language
	if lang != m.defaultLang {
		if entries, ok := m.translations[m.defaultLang]; ok {
			if entry, found := entries[key]; found {
				return entry, true
			}
		}
	}
	return nil, false
}

var placeholderRe = regexp.MustCompile(`\{(\d+)\}`)

// formatMessage replaces {0}, {1}, etc. with positional args
func formatMessage(template string, args ...interface{}) string {
	if len(args) == 0 {
		return template
	}
	return placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		idxStr := placeholderRe.FindStringSubmatch(match)[1]
		var idx int
		fmt.Sscanf(idxStr, "%d", &idx)
		if idx >= 0 && idx < len(args) {
			return fmt.Sprintf("%v", args[idx])
		}
		return match
	})
}

// Translate translates a key with optional positional arguments
func (m *LocaleManager) Translate(key string, args ...interface{}) string {
	m.mu.RLock()
	lang := m.currentLang
	m.mu.RUnlock()

	m.mu.RLock()
	entry, found := m.resolveKey(lang, key)
	m.mu.RUnlock()

	if !found {
		return key
	}
	return formatMessage(entry.Value, args...)
}

// T is a shorthand for Translate
func (m *LocaleManager) T(key string, args ...interface{}) string {
	return m.Translate(key, args...)
}

// TranslatePlural translates a key with plural form selection
func (m *LocaleManager) TranslatePlural(key string, count int, args ...interface{}) string {
	m.mu.RLock()
	lang := m.currentLang
	m.mu.RUnlock()

	m.mu.RLock()
	entry, found := m.resolveKey(lang, key)
	m.mu.RUnlock()

	if !found || entry.Plural == nil {
		// Fallback to simple translate
		return m.Translate(key, args...)
	}

	// Get plural category
	m.mu.RLock()
	pluralFn, ok := m.pluralFuncs[lang]
	if !ok {
		pluralFn = m.pluralFuncs[m.defaultLang]
	}
	m.mu.RUnlock()

	category := "other"
	if pluralFn != nil {
		category = pluralFn(count)
	}

	// Select plural form
	var template string
	switch category {
	case "zero":
		template = entry.Plural.Zero
	case "one":
		template = entry.Plural.One
	case "two":
		template = entry.Plural.Two
	case "few":
		template = entry.Plural.Few
	case "many":
		template = entry.Plural.Many
	default:
		template = entry.Plural.Other
	}

	if template == "" {
		template = entry.Plural.Other
	}
	if template == "" {
		return key
	}

	return formatMessage(template, args...)
}

// FormatDate formats a time.Time according to the current locale's conventions
func (m *LocaleManager) FormatDate(t time.Time) string {
	m.mu.RLock()
	lang := m.currentLang
	m.mu.RUnlock()

	switch lang {
	case "zh":
		return fmt.Sprintf("%d年%02d月%02d日", t.Year(), t.Month(), t.Day())
	case "ja":
		return fmt.Sprintf("%d年%02d月%02d日", t.Year(), t.Month(), t.Day())
	case "ar":
		return fmt.Sprintf("%02d/%02d/%d", t.Day(), t.Month(), t.Year())
	default: // en and others
		return t.Format("January 2, 2006")
	}
}

// FormatNumber formats a float64 according to locale conventions
func (m *LocaleManager) FormatNumber(n float64) string {
	m.mu.RLock()
	lang := m.currentLang
	m.mu.RUnlock()

	switch lang {
	case "zh", "ja":
		// Use comma as thousands separator, dot as decimal
		return formatWithSeparator(n, ',', '.')
	case "ar":
		// Arabic uses Arabic comma as thousands separator
		return formatWithSeparator(n, '٬', '٫')
	default: // en
		return formatWithSeparator(n, ',', '.')
	}
}

// formatWithSeparator formats a number with given separators
func formatWithSeparator(n float64, thousandsSep, decimalSep rune) string {
	// Handle negative
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}

	// Split integer and decimal parts
	intPart := int64(n)
	decPart := n - float64(intPart)

	// Format integer part with thousands separator
	intStr := fmt.Sprintf("%d", intPart)
	if len(intStr) > 3 {
		var result strings.Builder
		for i, c := range intStr {
			if i > 0 && (len(intStr)-i)%3 == 0 {
				result.WriteRune(thousandsSep)
			}
			result.WriteRune(c)
		}
		intStr = result.String()
	}

	// Format decimal part
	if decPart > 0 {
		decStr := fmt.Sprintf("%.2f", decPart)[1:] // remove leading "0"
		if decimalSep != '.' {
			decStr = strings.Replace(decStr, ".", string(decimalSep), 1)
		}
		return sign + intStr + decStr
	}
	return sign + intStr
}

// IsRTL returns true if the current language is right-to-left
func (m *LocaleManager) IsRTL() bool {
	m.mu.RLock()
	lang := m.currentLang
	m.mu.RUnlock()

	m.mu.RLock()
	l, ok := m.languages[lang]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	return l.Direction == DirectionRTL
}

// ExportTranslations returns all translations for a language as a flat map
func (m *LocaleManager) ExportTranslations(lang string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, ok := m.translations[lang]
	if !ok {
		return nil, fmt.Errorf("language %q not loaded", lang)
	}

	result := make(map[string]string, len(entries))
	for key, entry := range entries {
		result[key] = entry.Value
	}
	return result, nil
}

// RegisterPluralFunc registers a custom plural rule function for a language
func (m *LocaleManager) RegisterPluralFunc(lang string, fn func(int) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pluralFuncs[lang] = fn
}

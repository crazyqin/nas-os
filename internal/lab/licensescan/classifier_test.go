package licensescan

import (
	"testing"
)

func TestClassifyLicense(t *testing.T) {
	tests := []struct {
		name     string
		license  string
		expected Category
	}{
		{"MIT", "MIT", CategoryPermissive},
		{"Apache-2.0", "Apache-2.0", CategoryPermissive},
		{"BSD-2-Clause", "BSD-2-Clause", CategoryPermissive},
		{"BSD-3-Clause", "BSD-3-Clause", CategoryPermissive},
		{"ISC", "ISC", CategoryPermissive},
		{"Unlicense", "Unlicense", CategoryPermissive},
		{"LGPL-2.1", "LGPL-2.1", CategoryWeakCopyleft},
		{"LGPL-3.0", "LGPL-3.0", CategoryWeakCopyleft},
		{"MPL-2.0", "MPL-2.0", CategoryWeakCopyleft},
		{"EPL-2.0", "EPL-2.0", CategoryWeakCopyleft},
		{"GPL-2.0", "GPL-2.0", CategoryStrongCopyleft},
		{"GPL-3.0", "GPL-3.0", CategoryStrongCopyleft},
		{"AGPL-3.0", "AGPL-3.0", CategoryStrongCopyleft},
		{"AGPL-3.0-only", "AGPL-3.0-only", CategoryStrongCopyleft},
		{"Empty", "", CategoryUnknown},
		{"Unknown", "SomeCustomLicense", CategoryCustom},
		// 模糊匹配测试
		{"Fuzzy MIT", "MIT License", CategoryPermissive},
		{"Fuzzy GPL", "GNU GPL v3", CategoryStrongCopyleft},
		{"Fuzzy LGPL", "LGPL v2.1", CategoryWeakCopyleft},
		{"Fuzzy BSD", "BSD License", CategoryPermissive},
		{"Fuzzy Apache", "Apache License 2.0", CategoryPermissive},
		{"Fuzzy MPL", "Mozilla Public License", CategoryWeakCopyleft},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyLicense(tt.license)
			if result != tt.expected {
				t.Errorf("ClassifyLicense(%q) = %q, want %q", tt.license, result, tt.expected)
			}
		})
	}
}

func TestGetComplianceStatus(t *testing.T) {
	policy := &Policy{
		ID:          "test",
		Whitelist:   []string{"MIT", "Apache-2.0", "BSD-3-Clause"},
		Blacklist:   []string{"AGPL-3.0", "SSPL-1.0"},
		Graylist:    []string{"GPL-3.0", "LGPL-2.1"},
		DefaultList: ListGraylist,
	}

	tests := []struct {
		name     string
		license  string
		policy   *Policy
		expected Compliance
	}{
		{"Whitelist hit", "MIT", policy, ComplianceAllowed},
		{"Whitelist hit Apache", "Apache-2.0", policy, ComplianceAllowed},
		{"Blacklist hit", "AGPL-3.0", policy, ComplianceDenied},
		{"Blacklist hit SSPL", "SSPL-1.0", policy, ComplianceDenied},
		{"Graylist hit", "GPL-3.0", policy, ComplianceReview},
		{"Graylist hit LGPL", "LGPL-2.1", policy, ComplianceReview},
		{"Default graylist", "SomeUnknownLicense", policy, ComplianceReview},
		{"Nil policy MIT", "MIT", nil, ComplianceAllowed},
		{"Nil policy AGPL", "AGPL-3.0", nil, ComplianceDenied},
		{"Nil policy LGPL", "LGPL-2.1", nil, ComplianceReview},
		{"Nil policy unknown", "CustomLicense", nil, ComplianceUnknown},
		// 黑名单优先级测试
		{"Blacklist overrides whitelist", "MIT", &Policy{
			Whitelist: []string{"MIT"},
			Blacklist: []string{"MIT"},
		}, ComplianceDenied},
		// DefaultList whitelist测试
		{"Default whitelist", "UnknownLicense", &Policy{
			DefaultList: ListWhitelist,
		}, ComplianceAllowed},
		// DefaultList blacklist测试
		{"Default blacklist", "UnknownLicense", &Policy{
			DefaultList: ListBlacklist,
		}, ComplianceDenied},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetComplianceStatus(tt.license, tt.policy)
			if result != tt.expected {
				t.Errorf("GetComplianceStatus(%q, policy) = %q, want %q", tt.license, result, tt.expected)
			}
		})
	}
}

func TestMatchLicense(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"Exact match", "MIT", "MIT", true},
		{"Case insensitive", "mit", "MIT", true},
		{"Trim spaces", " MIT ", "MIT", true},
		{"Different", "MIT", "Apache-2.0", false},
		{"Empty both", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchLicense(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("matchLicense(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestSplitLicenses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Single", "MIT", 1},
		{"Comma separated", "MIT, BSD-3-Clause", 2},
		{"AND separated", "MIT AND Apache-2.0", 2},
		{"OR separated", "MIT OR Apache-2.0", 2},
		{"Semicolon", "MIT; Apache-2.0", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLicenses(tt.input)
			if len(result) != tt.expected {
				t.Errorf("splitLicenses(%q) returned %d items, want %d", tt.input, len(result), tt.expected)
			}
		})
	}
}

func TestExtractLabel(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		key      string
		expected string
	}{
		{"Simple", `{"license":"MIT"}`, "license", "MIT"},
		{"With spaces", `{"license": "MIT"}`, "license", "MIT"},
		{"Not found", `{"license":"MIT"}`, "version", ""},
		{"OCI label", `{"org.opencontainers.image.licenses":"Apache-2.0"}`, "org.opencontainers.image.licenses", "Apache-2.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLabel(tt.json, tt.key)
			if result != tt.expected {
				t.Errorf("extractLabel(%q, %q) = %q, want %q", tt.json, tt.key, result, tt.expected)
			}
		})
	}
}

func TestParseRequireLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantPath string
		wantVer  string
		wantNil  bool
	}{
		{"Standard", "require github.com/gin-gonic/gin v1.9.0", "github.com/gin-gonic/gin", "v1.9.0", false},
		{"With indirect", "require github.com/pkg/errors v0.9.0 // indirect", "github.com/pkg/errors", "v0.9.0", false},
		{"No require prefix", "github.com/google/uuid v1.3.0", "github.com/google/uuid", "v1.3.0", false},
		{"Too few fields", "require", "", "", true},
		{"Empty", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRequireLine(tt.line)
			if tt.wantNil {
				if result != nil {
					t.Errorf("parseRequireLine(%q) expected nil, got %v", tt.line, result)
				}
				return
			}
			if result == nil {
				t.Fatalf("parseRequireLine(%q) returned nil, want non-nil", tt.line)
			}
			if result.Path != tt.wantPath {
				t.Errorf("parseRequireLine(%q).Path = %q, want %q", tt.line, result.Path, tt.wantPath)
			}
			if result.Version != tt.wantVer {
				t.Errorf("parseRequireLine(%q).Version = %q, want %q", tt.line, result.Version, tt.wantVer)
			}
		})
	}
}

func TestLookupGoLicense(t *testing.T) {
	tests := []struct {
		name     string
		module   string
		version  string
		expected string
	}{
		{"Known gin", "github.com/gin-gonic/gin", "v1.9.0", "MIT"},
		{"Known gorilla/mux", "github.com/gorilla/mux", "v1.8.0", "BSD-3-Clause"},
		{"Known uuid", "github.com/google/uuid", "v1.3.0", "BSD-3-Clause"},
		{"golang.org/x", "golang.org/x/text", "v0.14.0", "BSD-3-Clause"},
		{"google.golang.org", "google.golang.org/grpc", "v1.60.0", "Apache-2.0"},
		{"k8s.io", "k8s.io/api", "v0.29.0", "Apache-2.0"},
		{"Unknown", "github.com/some/random/pkg", "v1.0.0", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lookupGoLicense(tt.module, tt.version, nil)
			if result != tt.expected {
				t.Errorf("lookupGoLicense(%q, %q) = %q, want %q", tt.module, tt.version, result, tt.expected)
			}
		})
	}
}

func TestBuildSummary(t *testing.T) {
	licenses := []License{
		{Name: "MIT", Compliance: ComplianceAllowed},
		{Name: "Apache-2.0", Compliance: ComplianceAllowed},
		{Name: "GPL-3.0", Compliance: ComplianceDenied},
		{Name: "LGPL-2.1", Compliance: ComplianceReview},
		{Name: "CustomLicense", Compliance: ComplianceUnknown},
	}

	summary := buildSummary(licenses)

	if summary.TotalLicenses != 5 {
		t.Errorf("TotalLicenses = %d, want 5", summary.TotalLicenses)
	}
	if summary.Allowed != 2 {
		t.Errorf("Allowed = %d, want 2", summary.Allowed)
	}
	if summary.Denied != 1 {
		t.Errorf("Denied = %d, want 1", summary.Denied)
	}
	if summary.ReviewRequired != 1 {
		t.Errorf("ReviewRequired = %d, want 1", summary.ReviewRequired)
	}
	if summary.Unknown != 1 {
		t.Errorf("Unknown = %d, want 1", summary.Unknown)
	}
}

func TestFindViolations(t *testing.T) {
	licenses := []License{
		{Name: "MIT", Compliance: ComplianceAllowed, Source: "pkg1"},
		{Name: "AGPL-3.0", Compliance: ComplianceDenied, Source: "pkg2"},
		{Name: "LGPL-2.1", Compliance: ComplianceReview, Source: "pkg3"},
		{Name: "Unknown", Compliance: ComplianceUnknown, Source: "pkg4"},
	}

	violations := findViolations(licenses)

	if len(violations) != 3 {
		t.Fatalf("findViolations returned %d violations, want 3", len(violations))
	}

	// 检查denied违规
	found := false
	for _, v := range violations {
		if v.LicenseName == "AGPL-3.0" && v.Severity == SeverityHigh {
			found = true
		}
	}
	if !found {
		t.Error("Expected AGPL-3.0 violation with severity high")
	}

	// 检查review违规
	found = false
	for _, v := range violations {
		if v.LicenseName == "LGPL-2.1" && v.Severity == SeverityMedium {
			found = true
		}
	}
	if !found {
		t.Error("Expected LGPL-2.1 violation with severity medium")
	}
}

func TestIdentifyLicenseFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"Apache", "Apache License Version 2.0, January 2004 http://www.apache.org/licenses/", "Apache-2.0"},
		{"MIT", "MIT License\n\nPermission is hereby granted, free of charge,", "MIT"},
		{"GPL3", "GNU GENERAL PUBLIC LICENSE Version 3, 29 June 2007", "GPL-3.0"},
		{"AGPL3", "GNU AFFERO GENERAL PUBLIC LICENSE Version 3", "AGPL-3.0"},
		{"LGPL3", "GNU LESSER GENERAL PUBLIC LICENSE Version 3", "LGPL-3.0"},
		{"BSD3", "Redistribution and use in source and binary forms, with or without", "BSD-3-Clause"},
		{"ISC", "ISC License\n\nPermission to use, copy, modify, and/or distribute", "ISC"},
		{"MPL2", "Mozilla Public License Version 2.0", "MPL-2.0"},
		{"Empty", "", ""},
		{"Unknown", "Some random text about nothing", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := identifyLicenseFromContent(tt.content)
			if result != tt.expected {
				t.Errorf("identifyLicenseFromContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		severity Severity
		expected int
	}{
		{SeverityLow, 1},
		{SeverityMedium, 2},
		{SeverityHigh, 3},
		{SeverityCritical, 4},
		{"", 0},
	}

	for _, tt := range tests {
		result := severityRank(tt.severity)
		if result != tt.expected {
			t.Errorf("severityRank(%q) = %d, want %d", tt.severity, result, tt.expected)
		}
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/gin-gonic/gin", "github.com"},
		{"golang.org/x/text", "golang.org"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		result := extractDomain(tt.input)
		if result != tt.expected {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultCompliance(t *testing.T) {
	tests := []struct {
		cat      Category
		expected Compliance
	}{
		{CategoryPermissive, ComplianceAllowed},
		{CategoryWeakCopyleft, ComplianceReview},
		{CategoryStrongCopyleft, ComplianceDenied},
		{CategoryCustom, ComplianceUnknown},
		{CategoryUnknown, ComplianceUnknown},
	}

	for _, tt := range tests {
		result := defaultCompliance(tt.cat)
		if result != tt.expected {
			t.Errorf("defaultCompliance(%q) = %q, want %q", tt.cat, result, tt.expected)
		}
	}
}

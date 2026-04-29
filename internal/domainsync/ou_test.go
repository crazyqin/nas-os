package domainsync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainToDN(t *testing.T) {
	tests := []struct {
		domain   string
		expected string
	}{
		{"example.com", "dc=example,dc=com"},
		{"corp.example.com", "dc=corp,dc=example,dc=com"},
		{"test.local", "dc=test,dc=local"},
		{"single", "dc=single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := domainToDN(tt.domain)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractCNFromDN(t *testing.T) {
	tests := []struct {
		dn       string
		expected string
	}{
		{"OU=Engineering,DC=example,DC=com", "Engineering"},
		{"CN=Admin,CN=Users,DC=example,DC=com", "Admin"},
		{"OU=Sales,OU=Corporate,DC=example,DC=com", "Sales"},
		{"invalid dn", "invalid dn"},
		{"DC=example,DC=com", "DC=example,DC=com"}, // 没有 CN 或 OU，返回原始 DN
	}

	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			result := extractCNFromDN(tt.dn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractParentDN(t *testing.T) {
	tests := []struct {
		dn       string
		expected string
	}{
		{"OU=Users,DC=example,DC=com", "DC=example,DC=com"},
		{"CN=Admin,CN=Users,DC=example,DC=com", "CN=Users,DC=example,DC=com"},
		{"DC=com", "com"},
		{"single", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.dn, func(t *testing.T) {
			result := extractParentDN(tt.dn)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateLevel(t *testing.T) {
	tests := []struct {
		name     string
		dn       string
		baseDN   string
		expected int
	}{
		{
			name:     "top level OU",
			dn:       "OU=Users,DC=example,DC=com",
			baseDN:   "DC=example,DC=com",
			expected: 1,
		},
		{
			name:     "nested OU",
			dn:       "OU=Engineering,OU=Users,DC=example,DC=com",
			baseDN:   "DC=example,DC=com",
			expected: 2,
		},
		{
			name:     "same as base",
			dn:       "DC=example,DC=com",
			baseDN:   "DC=example,DC=com",
			expected: 0,
		},
		{
			name:     "empty base",
			dn:       "OU=Test,DC=example,DC=com",
			baseDN:   "",
			expected: 3,
		},
		{
			name:     "deeply nested",
			dn:       "OU=L3,OU=L2,OU=L1,DC=example,DC=com",
			baseDN:   "DC=example,DC=com",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateLevel(tt.dn, tt.baseDN)
			assert.Equal(t, tt.expected, result)
		})
	}
}

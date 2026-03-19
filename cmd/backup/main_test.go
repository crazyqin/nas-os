package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		assert.Equal(t, tt.expected, result, "formatSize(%d)", tt.bytes)
	}
}

func TestBackupResult(t *testing.T) {
	result := BackupResult{
		BackupPath:    "/backup/test",
		IsIncremental: true,
		TotalFiles:    100,
		Duration:      5000000000, // 5 seconds
	}

	assert.Equal(t, "/backup/test", result.BackupPath)
	assert.True(t, result.IsIncremental)
	assert.Equal(t, 100, result.TotalFiles)
}

func TestPrintUsage(t *testing.T) {
	// Just ensure it doesn't panic
	assert.NotPanics(t, func() {
		printUsage()
	})
}
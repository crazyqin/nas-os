package filepreview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== 加密检测测试 ==========

func TestIsEncryptedPDFBytes_NotEncrypted(t *testing.T) {
	// 普通 PDF 头部
	pdfHeader := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	assert.False(t, IsEncryptedPDFBytes(pdfHeader))
}

func TestIsEncryptedPDFBytes_Encrypted(t *testing.T) {
	// 包含 /Encrypt 标记的 PDF
	encryptedPDF := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n2 0 obj\n<< /Encrypt 3 0 R >>\nendobj\n")
	assert.True(t, IsEncryptedPDFBytes(encryptedPDF))
}

func TestIsEncryptedPDFBytes_EmptyData(t *testing.T) {
	assert.False(t, IsEncryptedPDFBytes([]byte{}))
}

func TestEncryptedPDFManager_IsEncryptedPDF_FileNotFound(t *testing.T) {
	m := NewEncryptedPDFManager()
	_, err := m.IsEncryptedPDF(context.Background(), "/nonexistent/file.pdf")
	assert.Error(t, err)
}

func TestEncryptedPDFManager_IsEncryptedPDF_NotPDF(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("not a pdf"), 0644))

	m := NewEncryptedPDFManager()
	encrypted, err := m.IsEncryptedPDF(context.Background(), filePath)
	assert.NoError(t, err)
	assert.False(t, encrypted)
}

func TestEncryptedPDFManager_IsEncryptedPDF_PlainPDF(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plain.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	encrypted, err := m.IsEncryptedPDF(context.Background(), filePath)
	assert.NoError(t, err)
	assert.False(t, encrypted)
}

func TestEncryptedPDFManager_IsEncryptedPDF_EncryptedPDF(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "encrypted.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Encrypt 2 0 R >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	encrypted, err := m.IsEncryptedPDF(context.Background(), filePath)
	assert.NoError(t, err)
	assert.True(t, encrypted)
}

// ========== 密码验证测试 ==========

func TestEncryptedPDFManager_VerifyPassword_NotEncrypted(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plain.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	err := m.VerifyPassword(context.Background(), filePath, "password")
	assert.Error(t, err)
	assert.Equal(t, ErrPDFNotEncrypted, err)
}

func TestEncryptedPDFManager_UnlockPDF_NotEncrypted(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plain.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	_, err := m.UnlockPDF(context.Background(), filePath, "password")
	assert.Error(t, err)
	assert.Equal(t, ErrPDFNotEncrypted, err)
}

// ========== 尝试次数限制测试 ==========

func TestEncryptedPDFManager_AttemptTracking(t *testing.T) {
	m := NewEncryptedPDFManager()
	m.SetMaxAttempts(3, 1*time.Hour) // 1 hour block

	// 模拟失败尝试
	m.recordFailedAttempt("hash1")
	m.recordFailedAttempt("hash2")

	assert.Equal(t, 1, m.attempts["hash1"])
	assert.Equal(t, 1, m.attempts["hash2"])

	// 第三次应该触发封锁
	m.recordFailedAttempt("hash1")
	m.recordFailedAttempt("hash1")
	assert.True(t, m.isBlocked("hash1"))
	assert.False(t, m.isBlocked("hash2"))
}

func TestEncryptedPDFManager_ResetAttempts(t *testing.T) {
	m := NewEncryptedPDFManager()
	m.recordFailedAttempt("hash1")
	m.recordFailedAttempt("hash1")

	m.resetAttempts("hash1")
	assert.Equal(t, 0, m.attempts["hash1"])
	assert.False(t, m.isBlocked("hash1"))
}

func TestEncryptedPDFManager_ClearExpiredBlocks(t *testing.T) {
	m := NewEncryptedPDFManager()
	m.SetMaxAttempts(1, 1*time.Millisecond) // 1ms block

	m.recordFailedAttempt("hash1")
	// 应该被封锁了
	assert.True(t, m.isBlocked("hash1"))

	// 等待封锁过期
	time.Sleep(10 * time.Millisecond)
	m.ClearExpiredBlocks()
	assert.False(t, m.isBlocked("hash1"))
}

func TestEncryptedPDFManager_SetMaxAttempts(t *testing.T) {
	m := NewEncryptedPDFManager()
	m.SetMaxAttempts(10, 60_000_000_000) // 1 minute in nanoseconds

	assert.Equal(t, 10, m.maxAttempts)
}

// ========== 集成测试 ==========

func TestEncryptedPDFManager_GeneratePreview_PlainPDF(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plain.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	_, err := m.GenerateEncryptedPDFPreview(context.Background(), filePath, "", 1)
	assert.Error(t, err)
	assert.Equal(t, ErrPDFNotEncrypted, err)
}

func TestEncryptedPDFManager_GeneratePreview_EncryptedNoPassword(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "encrypted.pdf")
	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Encrypt 2 0 R >>\nendobj\n")
	require.NoError(t, os.WriteFile(filePath, pdfContent, 0644))

	m := NewEncryptedPDFManager()
	_, err := m.GenerateEncryptedPDFPreview(context.Background(), filePath, "", 1)
	// 应该失败，因为这不是真正的加密PDF，qpdf会报错
	assert.Error(t, err)
}

// ========== 预览请求集成测试 ==========

func TestPreviewRequest_PasswordField(t *testing.T) {
	req := &PreviewRequest{
		FilePath: "/tmp/test.pdf",
		Password: "secret123",
	}
	assert.Equal(t, "secret123", req.Password)
}

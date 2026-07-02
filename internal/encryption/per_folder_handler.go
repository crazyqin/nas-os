// Package encryption HTTP handlers for per-folder encryption API.
// Routes: /api/v1/encryption/folders/*
package encryption

import (
	"encoding/base64"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// ========== Per-Folder Encryption Handlers ==========

// PerFolderHandlers provides HTTP handlers for per-folder encryption.
type PerFolderHandlers struct {
	manager *PerFolderManager
}

// NewPerFolderHandlers creates handlers for per-folder encryption.
func NewPerFolderHandlers(mgr *PerFolderManager) *PerFolderHandlers {
	return &PerFolderHandlers{manager: mgr}
}

// RegisterRoutes registers per-folder encryption routes.
func (h *PerFolderHandlers) RegisterRoutes(r *gin.RouterGroup) {
	encGroup := r.Group("/encryption")
	{
		// Master key management
		encGroup.POST("/unlock", h.UnlockMaster)
		encGroup.POST("/lock", h.LockMaster)
		encGroup.GET("/status", h.GetStatus)

		// Folder operations
		encGroup.POST("/folders", h.CreateFolder)
		encGroup.GET("/folders", h.ListFolders)
		encGroup.GET("/folders/:id", h.GetFolder)
		encGroup.DELETE("/folders/:id", h.DeleteFolder)
		encGroup.POST("/folders/:id/unlock", h.UnlockFolder)
		encGroup.POST("/folders/:id/lock", h.LockFolder)
		encGroup.POST("/folders/:id/rotate-key", h.RotateKey)

		// Data operations
		encGroup.POST("/folders/:id/encrypt", h.EncryptData)
		encGroup.POST("/folders/:id/decrypt", h.DecryptData)

		// Stats
		encGroup.GET("/stats", h.GetStats)
	}
}

// ========== Request/Response Types ==========

// UnlockMasterRequest is the request to unlock the master key.
type UnlockMasterRequest struct {
	Password string `json:"password" binding:"required"`
	Salt     string `json:"salt" binding:"required"` // Base64 encoded salt
}

// CreateFolderRequest is the request to create an encrypted folder.
type CreateFolderRequest struct {
	Name string `json:"name" binding:"required,min=1,max=128"`
}

// EncryptDataRequest is the request to encrypt data.
type EncryptDataRequest struct {
	Data string `json:"data" binding:"required"` // Base64 encoded plaintext
}

// DecryptDataRequest is the request to decrypt data.
type DecryptDataRequest struct {
	Data string `json:"data" binding:"required"` // Base64 encoded ciphertext
}

// FolderInfo is the API response for folder information.
type FolderInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	State        string `json:"state"`
	Algorithm    string `json:"algorithm"`
	KeyVersion   int    `json:"keyVersion"`
	CreatedAt    string `json:"createdAt"`
	LastAccessed string `json:"lastAccessed"`
	FileCount    int64  `json:"fileCount"`
	TotalSize    int64  `json:"totalSize"`
	OriginalSize int64  `json:"originalSize"`
}

// ========== Master Key Handlers ==========

// UnlockMaster POST /api/v1/encryption/unlock.
func (h *PerFolderHandlers) UnlockMaster(c *gin.Context) {
	var req UnlockMasterRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	salt, err := decodeBase64(req.Salt)
	if err != nil {
		api.BadRequest(c, "invalid salt encoding: "+err.Error())
		return
	}

	if err := h.manager.UnlockWithPassword(req.Password, salt); err != nil {
		api.InternalError(c, "failed to unlock: "+err.Error())
		return
	}

	api.OK(c, gin.H{
		"message": "master key unlocked",
		"status":  "unlocked",
	})
}

// LockMaster POST /api/v1/encryption/lock.
func (h *PerFolderHandlers) LockMaster(c *gin.Context) {
	h.manager.LockMasterKey()
	api.OK(c, gin.H{
		"message": "master key locked, all folders secured",
		"status":  "locked",
	})
}

// GetStatus GET /api/v1/encryption/status.
func (h *PerFolderHandlers) GetStatus(c *gin.Context) {
	api.OK(c, gin.H{
		"masterKeyUnlocked": h.manager.IsUnlocked(),
		"folderCount":       len(h.manager.ListFolders()),
	})
}

// ========== Folder Handlers ==========

// CreateFolder POST /api/v1/encryption/folders.
func (h *PerFolderHandlers) CreateFolder(c *gin.Context) {
	var req CreateFolderRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	folder, err := h.manager.CreateFolder(req.Name)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, toFolderInfo(folder))
}

// ListFolders GET /api/v1/encryption/folders.
func (h *PerFolderHandlers) ListFolders(c *gin.Context) {
	folders := h.manager.ListFolders()
	infos := make([]FolderInfo, len(folders))
	for i, f := range folders {
		infos[i] = toFolderInfo(f)
	}
	api.OK(c, gin.H{
		"folders": infos,
		"total":   len(infos),
	})
}

// GetFolder GET /api/v1/encryption/folders/:id.
func (h *PerFolderHandlers) GetFolder(c *gin.Context) {
	folderID := c.Param("id")
	folder, err := h.manager.GetFolder(folderID)
	if err != nil {
		api.NotFound(c, "folder not found")
		return
	}
	api.OK(c, toFolderInfo(folder))
}

// DeleteFolder DELETE /api/v1/encryption/folders/:id.
func (h *PerFolderHandlers) DeleteFolder(c *gin.Context) {
	folderID := c.Param("id")
	if err := h.manager.DeleteFolder(folderID); err != nil {
		api.NotFound(c, "folder not found")
		return
	}
	api.OK(c, gin.H{"message": "folder deleted"})
}

// UnlockFolder POST /api/v1/encryption/folders/:id/unlock.
func (h *PerFolderHandlers) UnlockFolder(c *gin.Context) {
	folderID := c.Param("id")
	if err := h.manager.UnlockFolder(folderID); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	api.OK(c, gin.H{"message": "folder unlocked", "folderId": folderID})
}

// LockFolder POST /api/v1/encryption/folders/:id/lock.
func (h *PerFolderHandlers) LockFolder(c *gin.Context) {
	folderID := c.Param("id")
	if err := h.manager.LockFolder(folderID); err != nil {
		api.NotFound(c, "folder not found")
		return
	}
	api.OK(c, gin.H{"message": "folder locked", "folderId": folderID})
}

// RotateKey POST /api/v1/encryption/folders/:id/rotate-key.
func (h *PerFolderHandlers) RotateKey(c *gin.Context) {
	folderID := c.Param("id")
	if err := h.manager.RotateKey(folderID); err != nil {
		api.BadRequest(c, err.Error())
		return
	}
	api.OK(c, gin.H{"message": "key rotated", "folderId": folderID})
}

// ========== Data Handlers ==========

// EncryptData POST /api/v1/encryption/folders/:id/encrypt.
func (h *PerFolderHandlers) EncryptData(c *gin.Context) {
	folderID := c.Param("id")
	var req EncryptDataRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	plaintext, err := decodeBase64(req.Data)
	if err != nil {
		api.BadRequest(c, "invalid data encoding: "+err.Error())
		return
	}

	encrypted, err := h.manager.EncryptData(folderID, plaintext)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"data": encodeBase64(encrypted),
	})
}

// DecryptData POST /api/v1/encryption/folders/:id/decrypt.
func (h *PerFolderHandlers) DecryptData(c *gin.Context) {
	folderID := c.Param("id")
	var req DecryptDataRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	ciphertext, err := decodeBase64(req.Data)
	if err != nil {
		api.BadRequest(c, "invalid data encoding: "+err.Error())
		return
	}

	plaintext, err := h.manager.DecryptData(folderID, ciphertext)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"data": encodeBase64(plaintext),
	})
}

// ========== Stats ==========

// GetStats GET /api/v1/encryption/stats.
func (h *PerFolderHandlers) GetStats(c *gin.Context) {
	stats := h.manager.Stats()
	api.OK(c, stats)
}

// ========== Helpers ==========

func toFolderInfo(f *EncryptedFolder) FolderInfo {
	return FolderInfo{
		ID:           f.ID,
		Name:         f.Name,
		Path:         f.Path,
		State:        string(f.State),
		Algorithm:    string(f.Algorithm),
		KeyVersion:   f.KeyVersion,
		CreatedAt:    f.CreatedAt.Format("2006-01-02T15:04:05Z"),
		LastAccessed: f.LastAccessed.Format("2006-01-02T15:04:05Z"),
		FileCount:    f.FileCount,
		TotalSize:    f.TotalSize,
		OriginalSize: f.OriginalSize,
	}
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

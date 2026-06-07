package writeonce

import (
	"fmt"
	"sync"
	"time"
)

// Manager manages WriteOnce immutable folders
type Manager struct {
	mu      sync.RWMutex
	folders map[string]*WriteOnceFolder
	files   map[string][]*WriteOnceFile // folderID -> files
	audit   []*AuditEntry
	config  WriteOnceConfig
}

// NewManager creates a new WriteOnce manager
func NewManager(config WriteOnceConfig) *Manager {
	return &Manager{
		folders: make(map[string]*WriteOnceFolder),
		files:   make(map[string][]*WriteOnceFile),
		audit:   make([]*AuditEntry, 0),
		config:  config,
	}
}

// CreateFolder creates a new WriteOnce folder
func (m *Manager) CreateFolder(req CreateFolderRequest) (*WriteOnceFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("WriteOnce feature is disabled")
	}

	if req.Name == "" {
		return nil, fmt.Errorf("folder name is required")
	}

	if req.Path == "" {
		return nil, fmt.Errorf("folder path is required")
	}

	// Validate retention mode
	switch req.RetentionMode {
	case RetentionModeFixed:
		if req.RetentionDays <= 0 {
			return nil, fmt.Errorf("retention days must be positive for fixed retention mode")
		}
		if m.config.MaxRetentionDays > 0 && req.RetentionDays > m.config.MaxRetentionDays {
			return nil, fmt.Errorf("retention days %d exceeds maximum %d", req.RetentionDays, m.config.MaxRetentionDays)
		}
	case RetentionModeForever:
		if !m.config.AllowForeverLock {
			return nil, fmt.Errorf("forever lock is not allowed by configuration")
		}
	case RetentionModeUnlocked:
		// OK, folder starts unlocked
	default:
		return nil, fmt.Errorf("invalid retention mode: %s", req.RetentionMode)
	}

	// Validate policy mode
	switch req.PolicyMode {
	case PolicyModeEnterprise, PolicyModeCompliance:
		// OK
	case "":
		req.PolicyMode = PolicyModeEnterprise // default
	default:
		return nil, fmt.Errorf("invalid policy mode: %s", req.PolicyMode)
	}

	// Check for duplicate path
	for _, folder := range m.folders {
		if folder.Path == req.Path {
			return nil, fmt.Errorf("folder already exists at path: %s", req.Path)
		}
	}

	folder := &WriteOnceFolder{
		ID:            fmt.Sprintf("wo-%d", time.Now().UnixNano()),
		Name:          req.Name,
		Path:          req.Path,
		State:         FolderStateOpen,
		RetentionMode: req.RetentionMode,
		RetentionDays: req.RetentionDays,
		PolicyMode:    req.PolicyMode,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CreatedBy:     req.CreatedBy,
		Description:   req.Description,
		Tags:          req.Tags,
	}

	m.folders[folder.ID] = folder
	m.files[folder.ID] = make([]*WriteOnceFile, 0)

	// Audit log
	m.addAuditEntry(folder.ID, "folder_created", fmt.Sprintf("Folder '%s' created", req.Name), req.CreatedBy, true, "")

	return folder, nil
}

// LockFolder locks a folder, making it immutable
func (m *Manager) LockFolder(folderID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[folderID]
	if !ok {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	if folder.State == FolderStateLocked {
		return fmt.Errorf("folder is already locked")
	}

	if folder.State == FolderStateExpired {
		return fmt.Errorf("folder has expired and cannot be locked")
	}

	now := time.Now()
	folder.State = FolderStateLocked
	folder.LockedAt = &now
	folder.UpdatedAt = now

	// Set expiration for fixed retention
	if folder.RetentionMode == RetentionModeFixed && folder.RetentionDays > 0 {
		expiry := now.AddDate(0, 0, folder.RetentionDays)
		folder.ExpiresAt = &expiry
	}

	m.addAuditEntry(folderID, "folder_locked", fmt.Sprintf("Folder locked with %s retention", folder.RetentionMode), userID, true, "")

	return nil
}

// AddFile adds a file to an open folder
func (m *Manager) AddFile(req AddFileRequest) (*WriteOnceFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[req.FolderID]
	if !ok {
		return nil, fmt.Errorf("folder not found: %s", req.FolderID)
	}

	if folder.State != FolderStateOpen {
		return nil, fmt.Errorf("cannot add file to %s folder", folder.State)
	}

	if req.FileName == "" {
		return nil, fmt.Errorf("file name is required")
	}

	// Check for duplicate file name in folder
	for _, f := range m.files[req.FolderID] {
		if f.FileName == req.FileName && !f.IsDeleted {
			return nil, fmt.Errorf("file already exists in folder: %s", req.FileName)
		}
	}

	file := &WriteOnceFile{
		ID:         fmt.Sprintf("wf-%d", time.Now().UnixNano()),
		FolderID:   req.FolderID,
		FilePath:   req.FilePath,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		FileHash:   req.FileHash,
		IsDeleted:  false,
		CreatedAt:  time.Now(),
		UploadedBy: req.UploadedBy,
	}

	m.files[req.FolderID] = append(m.files[req.FolderID], file)
	folder.FileCount++
	folder.TotalSizeBytes += req.FileSize
	folder.UpdatedAt = time.Now()

	m.addAuditEntry(req.FolderID, "file_added", fmt.Sprintf("File '%s' added", req.FileName), req.UploadedBy, true, "")

	return file, nil
}

// PreventDelete prevents deletion of a file in a locked folder (WORM enforcement)
func (m *Manager) PreventDelete(folderID string, fileName string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[folderID]
	if !ok {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Open folders allow deletes
	if folder.State == FolderStateOpen {
		m.addAuditEntry(folderID, "delete_allowed", fmt.Sprintf("File '%s' delete allowed (folder open)", fileName), userID, true, "")
		return nil
	}

	// Locked folders prevent deletes
	if folder.State == FolderStateLocked {
		errMsg := "WORM violation: cannot delete file '" + fileName + "' from locked folder"
		m.addAuditEntry(folderID, "delete_blocked", errMsg, userID, false, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Expired folders allow deletes
	m.addAuditEntry(folderID, "delete_allowed", fmt.Sprintf("File '%s' delete allowed (folder expired)", fileName), userID, true, "")
	return nil
}

// PreventModify prevents modification of a file in a locked folder (WORM enforcement)
func (m *Manager) PreventModify(folderID string, fileName string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, ok := m.folders[folderID]
	if !ok {
		return fmt.Errorf("folder not found: %s", folderID)
	}

	// Open folders allow modifications
	if folder.State == FolderStateOpen {
		m.addAuditEntry(folderID, "modify_allowed", fmt.Sprintf("File '%s' modify allowed (folder open)", fileName), userID, true, "")
		return nil
	}

	// Locked folders prevent modifications
	if folder.State == FolderStateLocked {
		errMsg := "WORM violation: cannot modify file '" + fileName + "' in locked folder"
		m.addAuditEntry(folderID, "modify_blocked", errMsg, userID, false, errMsg)
		return fmt.Errorf("%s", errMsg)
	}

	// Expired folders allow modifications
	m.addAuditEntry(folderID, "modify_allowed", fmt.Sprintf("File '%s' modify allowed (folder expired)", fileName), userID, true, "")
	return nil
}

// GetFolder returns a folder by ID
func (m *Manager) GetFolder(folderID string) (*WriteOnceFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folder, ok := m.folders[folderID]
	if !ok {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}

	return folder, nil
}

// ListFolders lists all WriteOnce folders
func (m *Manager) ListFolders() []*WriteOnceFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folders := make([]*WriteOnceFolder, 0, len(m.folders))
	for _, f := range m.folders {
		folders = append(folders, f)
	}

	return folders
}

// GetFiles returns all files in a folder
func (m *Manager) GetFiles(folderID string) ([]*WriteOnceFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.folders[folderID]; !ok {
		return nil, fmt.Errorf("folder not found: %s", folderID)
	}

	files := m.files[folderID]
	if files == nil {
		return []*WriteOnceFile{}, nil
	}

	return files, nil
}

// GetAuditLog returns audit log entries for a folder
func (m *Manager) GetAuditLog(folderID string) []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]*AuditEntry, 0)
	for _, entry := range m.audit {
		if entry.FolderID == folderID {
			entries = append(entries, entry)
		}
	}

	return entries
}

// GetAllAuditLog returns all audit log entries
func (m *Manager) GetAllAuditLog() []*AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.audit
}

// CheckExpiry checks and updates expired folders
func (m *Manager) CheckExpiry() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	expired := 0
	now := time.Now()

	for _, folder := range m.folders {
		if folder.State == FolderStateLocked && folder.ExpiresAt != nil && folder.ExpiresAt.Before(now) {
			folder.State = FolderStateExpired
			folder.UpdatedAt = now
			expired++

			m.addAuditEntry(folder.ID, "folder_expired", "Retention period expired", "system", true, "")
		}
	}

	return expired
}

// UpdateConfig updates the WriteOnce configuration
func (m *Manager) UpdateConfig(config WriteOnceConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() WriteOnceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// GetStats returns statistics about WriteOnce folders
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalFolders := len(m.folders)
	lockedFolders := 0
	openFolders := 0
	expiredFolders := 0
	totalFiles := int64(0)
	totalSize := int64(0)

	for _, folder := range m.folders {
		switch folder.State {
		case FolderStateLocked:
			lockedFolders++
		case FolderStateOpen:
			openFolders++
		case FolderStateExpired:
			expiredFolders++
		}
		totalFiles += folder.FileCount
		totalSize += folder.TotalSizeBytes
	}

	return map[string]interface{}{
		"total_folders":    totalFolders,
		"locked_folders":   lockedFolders,
		"open_folders":     openFolders,
		"expired_folders":  expiredFolders,
		"total_files":      totalFiles,
		"total_size_bytes": totalSize,
		"audit_entries":    len(m.audit),
	}
}

// addAuditEntry adds an entry to the audit log (caller must hold lock)
func (m *Manager) addAuditEntry(folderID, action, details, userID string, success bool, errorMsg string) {
	if !m.config.AuditLogEnabled {
		return
	}

	entry := &AuditEntry{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		FolderID:  folderID,
		Action:    action,
		Details:   details,
		UserID:    userID,
		Timestamp: time.Now(),
		Success:   success,
		ErrorMsg:  errorMsg,
	}

	m.audit = append(m.audit, entry)
}

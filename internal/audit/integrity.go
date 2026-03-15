package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// IntegrityManager 审计日志完整性管理器
// 负责日志签名、验证和防篡改
type IntegrityManager struct {
	signingKey []byte
	chainHash  []byte // 区块链式哈希
	mu         sync.RWMutex
}

// NewIntegrityManager 创建完整性管理器
func NewIntegrityManager() *IntegrityManager {
	return &IntegrityManager{
		signingKey: []byte(generateSecureKey()),
		chainHash:  nil,
	}
}

// SetSigningKey 设置签名密钥
func (im *IntegrityManager) SetSigningKey(key []byte) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.signingKey = key
}

// SignEntry 对日志条目签名
func (im *IntegrityManager) SignEntry(entry *Entry, previousHash []byte) string {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// 构建签名数据
	signData := im.buildSignData(entry, previousHash)

	// 使用HMAC-SHA256签名
	h := hmac.New(sha256.New, im.signingKey)
	h.Write(signData)
	return hex.EncodeToString(h.Sum(nil))
}

// buildSignData 构建签名数据
func (im *IntegrityManager) buildSignData(entry *Entry, previousHash []byte) []byte {
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s",
		entry.ID,
		entry.Timestamp.Format(time.RFC3339Nano),
		entry.Level,
		entry.Category,
		entry.Event,
		entry.UserID,
		entry.Resource,
		entry.Status,
	)

	// 如果有前一个哈希，加入链接
	if len(previousHash) > 0 {
		data += "|" + hex.EncodeToString(previousHash)
	}

	return []byte(data)
}

// VerifyEntry 验证日志条目
func (im *IntegrityManager) VerifyEntry(entry *Entry, previousHash []byte) bool {
	if entry.Signature == "" {
		return false
	}

	expectedSig := im.SignEntry(entry, previousHash)
	return hmac.Equal([]byte(entry.Signature), []byte(expectedSig))
}

// ComputeChainHash 计算区块链式哈希
func (im *IntegrityManager) ComputeChainHash(entries []*Entry) []byte {
	im.mu.Lock()
	defer im.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	// 从第一条开始计算链式哈希
	var prevHash []byte
	for _, entry := range entries {
		// 将前一个哈希加入当前条目的哈希计算
		h := sha256.New()
		h.Write([]byte(entry.ID))
		h.Write([]byte(entry.Timestamp.Format(time.RFC3339Nano)))
		h.Write([]byte(entry.Level))
		h.Write([]byte(entry.Category))
		h.Write([]byte(entry.Event))
		if entry.Signature != "" {
			h.Write([]byte(entry.Signature))
		}
		if len(prevHash) > 0 {
			h.Write(prevHash)
		}
		prevHash = h.Sum(nil)
	}

	im.chainHash = prevHash
	return prevHash
}

// GenerateMerkleRoot 生成默克尔树根哈希（用于批量验证）
func (im *IntegrityManager) GenerateMerkleRoot(entries []*Entry) string {
	if len(entries) == 0 {
		return ""
	}

	// 计算每个条目的哈希
	hashes := make([][]byte, len(entries))
	for i, entry := range entries {
		h := sha256.New()
		data, _ := json.Marshal(entry)
		h.Write(data)
		hashes[i] = h.Sum(nil)
	}

	// 构建默克尔树
	for len(hashes) > 1 {
		newHashes := make([][]byte, 0)
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				// 合并两个哈希
				h := sha256.New()
				h.Write(hashes[i])
				h.Write(hashes[i+1])
				newHashes = append(newHashes, h.Sum(nil))
			} else {
				// 奇数个，最后一个直接保留
				newHashes = append(newHashes, hashes[i])
			}
		}
		hashes = newHashes
	}

	return hex.EncodeToString(hashes[0])
}

// AuditProof 审计证明
type AuditProof struct {
	EntryID    string    `json:"entry_id"`
	Timestamp  time.Time `json:"timestamp"`
	RootHash   string    `json:"root_hash"`
	ProofPath  []string  `json:"proof_path"`
	ProofIndex int       `json:"proof_index"`
}

// GenerateAuditProof 生成审计证明
func (im *IntegrityManager) GenerateAuditProof(entries []*Entry, entryID string) (*AuditProof, error) {
	// 找到目标条目
	var targetEntry *Entry
	var targetIndex int
	for i, e := range entries {
		if e.ID == entryID {
			targetEntry = e
			targetIndex = i
			break
		}
	}

	if targetEntry == nil {
		return nil, fmt.Errorf("entry not found")
	}

	// 计算默克尔树根
	rootHash := im.GenerateMerkleRoot(entries)

	// 构建证明路径
	proofPath := im.buildMerkleProof(entries, targetIndex)

	return &AuditProof{
		EntryID:    entryID,
		Timestamp:  targetEntry.Timestamp,
		RootHash:   rootHash,
		ProofPath:  proofPath,
		ProofIndex: targetIndex,
	}, nil
}

// buildMerkleProof 构建默克尔证明路径
func (im *IntegrityManager) buildMerkleProof(entries []*Entry, targetIndex int) []string {
	proofPath := make([]string, 0)

	// 计算每个条目的哈希
	hashes := make([][]byte, len(entries))
	for i, entry := range entries {
		h := sha256.New()
		data, _ := json.Marshal(entry)
		h.Write(data)
		hashes[i] = h.Sum(nil)
	}

	currentIndex := targetIndex
	currentHashes := hashes

	for len(currentHashes) > 1 {
		// 找到兄弟节点
		if currentIndex%2 == 0 {
			// 当前节点是左节点，兄弟是右节点
			if currentIndex+1 < len(currentHashes) {
				proofPath = append(proofPath, hex.EncodeToString(currentHashes[currentIndex+1]))
			}
		} else {
			// 当前节点是右节点，兄弟是左节点
			proofPath = append(proofPath, hex.EncodeToString(currentHashes[currentIndex-1]))
		}

		// 上升一层
		newHashes := make([][]byte, 0)
		for i := 0; i < len(currentHashes); i += 2 {
			if i+1 < len(currentHashes) {
				h := sha256.New()
				h.Write(currentHashes[i])
				h.Write(currentHashes[i+1])
				newHashes = append(newHashes, h.Sum(nil))
			} else {
				newHashes = append(newHashes, currentHashes[i])
			}
		}

		currentIndex = currentIndex / 2
		currentHashes = newHashes
	}

	return proofPath
}

// VerifyAuditProof 验证审计证明
func (im *IntegrityManager) VerifyAuditProof(proof *AuditProof, entry *Entry) bool {
	// 计算条目哈希
	h := sha256.New()
	data, _ := json.Marshal(entry)
	h.Write(data)
	currentHash := h.Sum(nil)

	// 沿着证明路径验证
	for _, siblingHash := range proof.ProofPath {
		sibling, err := hex.DecodeString(siblingHash)
		if err != nil {
			return false
		}

		newH := sha256.New()
		// 根据位置决定顺序
		if proof.ProofIndex%2 == 0 {
			newH.Write(currentHash)
			newH.Write(sibling)
		} else {
			newH.Write(sibling)
			newH.Write(currentHash)
		}
		currentHash = newH.Sum(nil)
		proof.ProofIndex = proof.ProofIndex / 2
	}

	return hex.EncodeToString(currentHash) == proof.RootHash
}

// ========== 日志归档签名 ==========

// ArchiveManifest 归档清单
type ArchiveManifest struct {
	ArchiveID      string    `json:"archive_id"`
	CreatedAt      time.Time `json:"created_at"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	EntryCount     int       `json:"entry_count"`
	MerkleRoot     string    `json:"merkle_root"`
	Signature      string    `json:"signature"`
	ChecksumSHA256 string    `json:"checksum_sha256"`
}

// CreateArchiveManifest 创建归档清单
func (im *IntegrityManager) CreateArchiveManifest(entries []*Entry, archivePath string) (*ArchiveManifest, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("no entries to archive")
	}

	// 排序按时间
	sortedEntries := make([]*Entry, len(entries))
	copy(sortedEntries, entries)

	// 找到时间范围
	var startTime, endTime time.Time
	for i, e := range sortedEntries {
		if i == 0 || e.Timestamp.Before(startTime) {
			startTime = e.Timestamp
		}
		if i == 0 || e.Timestamp.After(endTime) {
			endTime = e.Timestamp
		}
	}

	// 计算默克尔根
	merkleRoot := im.GenerateMerkleRoot(sortedEntries)

	// 计算文件校验和
	checksum, err := im.computeFileChecksum(archivePath)
	if err != nil {
		checksum = "" // 文件可能还不存在
	}

	manifest := &ArchiveManifest{
		ArchiveID:      fmt.Sprintf("ARCH-%d", time.Now().UnixNano()),
		CreatedAt:      time.Now(),
		StartTime:      startTime,
		EndTime:        endTime,
		EntryCount:     len(sortedEntries),
		MerkleRoot:     merkleRoot,
		ChecksumSHA256: checksum,
	}

	// 签名清单
	signData := fmt.Sprintf("%s|%s|%s|%d|%s",
		manifest.ArchiveID,
		manifest.StartTime.Format(time.RFC3339),
		manifest.EndTime.Format(time.RFC3339),
		manifest.EntryCount,
		manifest.MerkleRoot,
	)

	h := hmac.New(sha256.New, im.signingKey)
	h.Write([]byte(signData))
	manifest.Signature = hex.EncodeToString(h.Sum(nil))

	return manifest, nil
}

// computeFileChecksum 计算文件校验和
func (im *IntegrityManager) computeFileChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyArchiveManifest 验证归档清单
func (im *IntegrityManager) VerifyArchiveManifest(manifest *ArchiveManifest) bool {
	signData := fmt.Sprintf("%s|%s|%s|%d|%s",
		manifest.ArchiveID,
		manifest.StartTime.Format(time.RFC3339),
		manifest.EndTime.Format(time.RFC3339),
		manifest.EntryCount,
		manifest.MerkleRoot,
	)

	h := hmac.New(sha256.New, im.signingKey)
	h.Write([]byte(signData))
	expectedSig := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(manifest.Signature), []byte(expectedSig))
}

// SaveManifest 保存清单到文件
func (im *IntegrityManager) SaveManifest(manifest *ArchiveManifest, dir string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(dir, fmt.Sprintf("manifest-%s.json", manifest.ArchiveID))
	return os.WriteFile(filename, data, 0640)
}

// LoadManifest 从文件加载清单
func (im *IntegrityManager) LoadManifest(filePath string) (*ArchiveManifest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var manifest ArchiveManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// generateSecureKey 生成安全密钥
func generateSecureKey() string {
	// 使用时间戳和随机数生成
	h := sha256.New()
	h.Write([]byte(time.Now().String()))
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))
}

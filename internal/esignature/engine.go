// Package esignature 提供电子签名功能
package esignature

import (
	"errors"
	"sync"
	"time"
)

// Engine 签名引擎.
type Engine struct {
	mu          sync.RWMutex
	documents   map[string]*Document
	certs       map[string]*Certificate
	workflows   map[string]*Workflow
	templates   map[string]*Template
	auditLog    []AuditEntry
	idCounter   int64
}

// NewEngine 创建签名引擎.
func NewEngine() *Engine {
	return &Engine{
		documents: make(map[string]*Document),
		certs:     make(map[string]*Certificate),
		workflows: make(map[string]*Workflow),
		templates: make(map[string]*Template),
		auditLog:  make([]AuditEntry, 0),
	}
}

// generateID 生成唯一ID.
func (e *Engine) generateID(prefix string) string {
	e.idCounter++
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + string(rune('A'+e.idCounter%26))
}

// addAudit 添加审计记录.
func (e *Engine) addAudit(docID, userID, action, details, ip, ua string) {
	entry := AuditEntry{
		ID:         e.generateID("audit"),
		DocumentID: docID,
		UserID:     userID,
		Action:     action,
		Details:    details,
		IPAddress:  ip,
		UserAgent:  ua,
		Timestamp:  time.Now(),
	}
	e.auditLog = append(e.auditLog, entry)
}

// CreateDocument 创建文档.
func (e *Engine) CreateDocument(req CreateDocRequest) (*Document, error) {
	if req.Title == "" {
		return nil, errors.New("标题不能为空")
	}
	if req.Content == "" {
		return nil, errors.New("内容不能为空")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	doc := &Document{
		ID:         e.generateID("doc"),
		Title:      req.Title,
		Content:    req.Content,
		Creator:    req.Creator,
		Status:     "draft",
		Signers:    make([]Signer, 0),
		Signatures: make([]Signature, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	e.documents[doc.ID] = doc

	e.addAudit(doc.ID, req.Creator, "create", "创建文档", "", "")

	return doc, nil
}

// GetDocument 获取文档.
func (e *Engine) GetDocument(id string) (*Document, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, ok := e.documents[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}
	return doc, nil
}

// UpdateDocument 更新文档.
func (e *Engine) UpdateDocument(id string, req UpdateDocRequest) (*Document, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[id]
	if !ok {
		return nil, errors.New("文档不存在")
	}

	if doc.Status != "draft" {
		return nil, errors.New("只能更新草稿状态的文档")
	}

	if req.Title != nil {
		doc.Title = *req.Title
	}
	if req.Content != nil {
		doc.Content = *req.Content
	}
	doc.UpdatedAt = time.Now()

	return doc, nil
}

// DeleteDocument 删除文档.
func (e *Engine) DeleteDocument(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[id]
	if !ok {
		return errors.New("文档不存在")
	}

	if doc.Status == "completed" {
		return errors.New("不能删除已完成的文档")
	}

	delete(e.documents, id)
	return nil
}

// ListDocuments 列出文档.
func (e *Engine) ListDocuments(creator string) []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range e.documents {
		if creator == "" || doc.Creator == creator {
			result = append(result, doc)
		}
	}
	return result
}

// AddSigner 添加签署人.
func (e *Engine) AddSigner(docID string, req AddSignerRequest) (*Signer, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return nil, errors.New("文档不存在")
	}

	if doc.Status != "draft" && doc.Status != "pending" {
		return nil, errors.New("只能在草稿或待签状态添加签署人")
	}

	signer := Signer{
		ID:       e.generateID("signer"),
		UserID:   req.UserID,
		Name:     req.Name,
		Email:    req.Email,
		Role:     req.Role,
		Order:    req.Order,
		Status:   "pending",
		SignType: req.SignType,
		Required: req.Required,
	}
	doc.Signers = append(doc.Signers, signer)
	doc.UpdatedAt = time.Now()

	return &signer, nil
}

// RemoveSigner 移除签署人.
func (e *Engine) RemoveSigner(docID, signerID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return errors.New("文档不存在")
	}

	for i, signer := range doc.Signers {
		if signer.ID == signerID {
			if signer.Status == "signed" {
				return errors.New("不能移除已签名的签署人")
			}
			doc.Signers = append(doc.Signers[:i], doc.Signers[i+1:]...)
			doc.UpdatedAt = time.Now()
			return nil
		}
	}

	return errors.New("签署人不存在")
}

// SendForSignature 发送签名.
func (e *Engine) SendForSignature(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return errors.New("文档不存在")
	}

	if doc.Status != "draft" {
		return errors.New("只能发送草稿状态的文档")
	}

	if len(doc.Signers) == 0 {
		return errors.New("没有签署人")
	}

	doc.Status = "pending"
	doc.UpdatedAt = time.Now()

	e.addAudit(docID, doc.Creator, "send", "发送签名请求", "", "")

	return nil
}

// Sign 签名.
func (e *Engine) Sign(docID string, req SignRequest) (*Signature, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return nil, errors.New("文档不存在")
	}

	if doc.Status != "pending" && doc.Status != "in_progress" {
		return nil, errors.New("文档不在可签名状态")
	}

	// 查找签署人
	var signer *Signer
	for i, s := range doc.Signers {
		if s.ID == req.SignerID {
			signer = &doc.Signers[i]
			break
		}
	}

	if signer == nil {
		return nil, errors.New("签署人不存在")
	}

	if signer.Status == "signed" {
		return nil, errors.New("已经签名")
	}

	if signer.Status == "declined" {
		return nil, errors.New("已拒绝签名")
	}

	// 创建签名
	signature := Signature{
		ID:         e.generateID("sig"),
		SignerID:   req.SignerID,
		DocumentID: docID,
		Type:       signer.SignType,
		Data:       req.Signature,
		CertID:     req.CertID,
		IPAddress:  req.IPAddress,
		UserAgent:  req.UserAgent,
		Timestamp:  time.Now(),
	}

	// 更新签署人状态
	now := time.Now()
	signer.Status = "signed"
	signer.SignedAt = &now

	doc.Signatures = append(doc.Signatures, signature)
	doc.Status = "in_progress"
	doc.UpdatedAt = time.Now()

	e.addAudit(docID, signer.UserID, "sign", "签名", req.IPAddress, req.UserAgent)

	// 检查是否所有人都签完了
	allSigned := true
	for _, s := range doc.Signers {
		if s.Required && s.Status != "signed" {
			allSigned = false
			break
		}
	}

	if allSigned {
		doc.Status = "completed"
		doc.CompletedAt = &now
		e.addAudit(docID, "", "complete", "所有签署人已完成签名", "", "")
	}

	return &signature, nil
}

// DeclineSignature 拒绝签名.
func (e *Engine) DeclineSignature(docID string, req DeclineRequest) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return errors.New("文档不存在")
	}

	for i, signer := range doc.Signers {
		if signer.ID == req.SignerID {
			if signer.Status == "signed" {
				return errors.New("已签名，无法拒绝")
			}

			now := time.Now()
			doc.Signers[i].Status = "declined"
			doc.Signers[i].DeclinedAt = &now
			doc.Signers[i].DeclinedReason = req.Reason
			doc.UpdatedAt = time.Now()

			e.addAudit(docID, signer.UserID, "decline", "拒绝签名: "+req.Reason, "", "")

			return nil
		}
	}

	return errors.New("签署人不存在")
}

// CancelDocument 取消文档.
func (e *Engine) CancelDocument(docID, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return errors.New("文档不存在")
	}

	if doc.Status == "completed" {
		return errors.New("不能取消已完成的文档")
	}

	doc.Status = "cancelled"
	doc.UpdatedAt = time.Now()

	e.addAudit(docID, userID, "cancel", "取消文档", "", "")

	return nil
}

// GetAuditLog 获取审计日志.
func (e *Engine) GetAuditLog(docID string) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if docID == "" {
		return e.auditLog
	}

	result := make([]AuditEntry, 0)
	for _, entry := range e.auditLog {
		if entry.DocumentID == docID {
			result = append(result, entry)
		}
	}
	return result
}

// GetDocumentStats 获取文档统计.
func (e *Engine) GetDocumentStats() DocumentStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := DocumentStats{}
	for _, doc := range e.documents {
		stats.Total++
		switch doc.Status {
		case "draft":
			stats.Draft++
		case "pending", "in_progress":
			stats.Pending++
		case "completed":
			stats.Completed++
		case "cancelled":
			stats.Cancelled++
		case "expired":
			stats.Expired++
		}
	}
	return stats
}

// GetSignerStats 获取签署人统计.
func (e *Engine) GetSignerStats(docID string) SignerStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := SignerStats{}
	doc, ok := e.documents[docID]
	if !ok {
		return stats
	}

	for _, signer := range doc.Signers {
		stats.Total++
		switch signer.Status {
		case "signed":
			stats.Signed++
		case "pending":
			stats.Pending++
		case "declined":
			stats.Declined++
		}
	}
	return stats
}

// SetExpiry 设置过期时间.
func (e *Engine) SetExpiry(docID string, expiresAt time.Time) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, ok := e.documents[docID]
	if !ok {
		return errors.New("文档不存在")
	}

	doc.ExpiresAt = &expiresAt
	doc.UpdatedAt = time.Now()

	return nil
}

// CheckExpiry 检查过期.
func (e *Engine) CheckExpiry(docID string) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, ok := e.documents[docID]
	if !ok {
		return false, errors.New("文档不存在")
	}

	if doc.ExpiresAt == nil {
		return false, nil
	}

	if time.Now().After(*doc.ExpiresAt) {
		return true, nil
	}

	return false, nil
}

// ExpireDocuments 过期文档.
func (e *Engine) ExpireDocuments() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	expired := 0
	now := time.Now()
	for _, doc := range e.documents {
		if doc.ExpiresAt != nil && now.After(*doc.ExpiresAt) {
			if doc.Status == "pending" || doc.Status == "in_progress" {
				doc.Status = "expired"
				doc.UpdatedAt = now
				expired++
				e.addAudit(doc.ID, "", "expire", "文档已过期", "", "")
			}
		}
	}
	return expired
}

// DuplicateDocument 复制文档.
func (e *Engine) DuplicateDocument(docID, userID string) (*Document, error) {
	e.mu.RLock()
	original, ok := e.documents[docID]
	e.mu.RUnlock()

	if !ok {
		return nil, errors.New("文档不存在")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	newDoc := &Document{
		ID:         e.generateID("doc"),
		Title:      original.Title + " (副本)",
		Content:    original.Content,
		Creator:    userID,
		Status:     "draft",
		Signers:    make([]Signer, 0),
		Signatures: make([]Signature, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	// 复制签署人
	for _, signer := range original.Signers {
		newSigner := signer
		newSigner.ID = e.generateID("signer")
		newSigner.Status = "pending"
		newSigner.SignedAt = nil
		newSigner.DeclinedAt = nil
		newDoc.Signers = append(newDoc.Signers, newSigner)
	}

	e.documents[newDoc.ID] = newDoc
	e.addAudit(newDoc.ID, userID, "create", "复制文档", "", "")

	return newDoc, nil
}

// BulkSign 批量签名.
func (e *Engine) BulkSign(req BulkSignRequest) BulkSignResult {
	result := BulkSignResult{
		Success: make([]string, 0),
		Failed:  make([]string, 0),
		Errors:  make(map[string]string),
	}

	for _, docID := range req.DocumentIDs {
		// 查找文档的签署人
		e.mu.RLock()
		doc, ok := e.documents[docID]
		e.mu.RUnlock()

		if !ok {
			result.Failed = append(result.Failed, docID)
			result.Errors[docID] = "文档不存在"
			continue
		}

		// 查找对应用户的签署人
		signerID := ""
		for _, signer := range doc.Signers {
			if signer.UserID == req.SignerID && signer.Status == "pending" {
				signerID = signer.ID
				break
			}
		}

		if signerID == "" {
			result.Failed = append(result.Failed, docID)
			result.Errors[docID] = "未找到待签签署人"
			continue
		}

		_, err := e.Sign(docID, SignRequest{
			SignerID:  signerID,
			Signature: req.Signature,
		})

		if err != nil {
			result.Failed = append(result.Failed, docID)
			result.Errors[docID] = err.Error()
		} else {
			result.Success = append(result.Success, docID)
		}
	}

	return result
}

// GetPendingDocuments 获取待签文档.
func (e *Engine) GetPendingDocuments(userID string) []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range e.documents {
		if doc.Status != "pending" && doc.Status != "in_progress" {
			continue
		}
		for _, signer := range doc.Signers {
			if signer.UserID == userID && signer.Status == "pending" {
				result = append(result, doc)
				break
			}
		}
	}
	return result
}

// GetCompletedDocuments 获取已完成文档.
func (e *Engine) GetCompletedDocuments(userID string) []*Document {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Document, 0)
	for _, doc := range e.documents {
		if doc.Status != "completed" {
			continue
		}
		if userID == "" || doc.Creator == userID {
			result = append(result, doc)
		}
	}
	return result
}

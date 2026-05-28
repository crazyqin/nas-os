// Package esignature 提供电子签名功能
package esignature

import (
	"time"
)

// Document 签名文档.
type Document struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Creator     string    `json:"creator"`
	Status      string    `json:"status"` // draft, pending, in_progress, completed, cancelled, expired
	Signers     []Signer  `json:"signers"`
	Signatures  []Signature `json:"signatures,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Signer 签署人.
type Signer struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // signer, approver, witness, cc
	Order     int       `json:"order"`
	Status    string    `json:"status"` // pending, signed, declined, expired
	SignType  string    `json:"sign_type"` // electronic, digital, biometric
	Required  bool      `json:"required"`
	SignedAt  *time.Time `json:"signed_at,omitempty"`
	DeclinedAt *time.Time `json:"declined_at,omitempty"`
	DeclinedReason string `json:"declined_reason,omitempty"`
}

// Signature 签名.
type Signature struct {
	ID          string    `json:"id"`
	SignerID    string    `json:"signer_id"`
	DocumentID  string    `json:"document_id"`
	Type        string    `json:"type"` // electronic, digital, biometric
	Data        string    `json:"data"` // 签名数据（图片base64或数字签名）
	CertID      string    `json:"cert_id,omitempty"`
	Algorithm   string    `json:"algorithm,omitempty"`
	Hash        string    `json:"hash,omitempty"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
	TimestampToken string `json:"timestamp_token,omitempty"`
}

// Certificate 数字证书.
type Certificate struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Issuer      string    `json:"issuer"`
	Subject     string    `json:"subject"`
	Serial      string    `json:"serial"`
	NotBefore   time.Time `json:"not_before"`
	NotAfter    time.Time `json:"not_after"`
	PublicKey   string    `json:"public_key"`
	PrivateKey  string    `json:"private_key,omitempty"`
	Algorithm   string    `json:"algorithm"` // RSA, ECDSA
	IsCA        bool      `json:"is_ca"`
	Revoked     bool      `json:"revoked"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Workflow 签名工作流.
type Workflow struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Steps       []WorkflowStep `json:"steps"`
	Status      string        `json:"status"` // active, inactive, archived
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// WorkflowStep 工作流步骤.
type WorkflowStep struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // sign, approve, notify, condition
	Order       int      `json:"order"`
	Assignees   []string `json:"assignees"`
	Parallel    bool     `json:"parallel"` // 并行执行
	Required    bool     `json:"required"`
	Timeout     int      `json:"timeout,omitempty"` // 超时时间（小时）
	Conditions  []Condition `json:"conditions,omitempty"`
}

// Condition 条件.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // eq, neq, gt, lt, gte, lte, in, contains
	Value    string `json:"value"`
}

// AuditEntry 审计记录.
type AuditEntry struct {
	ID          string    `json:"id"`
	DocumentID  string    `json:"document_id"`
	UserID      string    `json:"user_id"`
	Action      string    `json:"action"` // create, sign, decline, cancel, expire, view, download
	Details     string    `json:"details,omitempty"`
	IPAddress   string    `json:"ip_address,omitempty"`
	UserAgent   string    `json:"user_agent,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// Template 签名模板.
type Template struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`
	Signers     []SignerTemplate `json:"signers"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// SignerTemplate 签署人模板.
type SignerTemplate struct {
	Role     string `json:"role"`
	Order    int    `json:"order"`
	Required bool   `json:"required"`
	SignType string `json:"sign_type"`
}

// CreateDocRequest 创建文档请求.
type CreateDocRequest struct {
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
	Creator string `json:"creator"`
}

// UpdateDocRequest 更新文档请求.
type UpdateDocRequest struct {
	Title   *string `json:"title,omitempty"`
	Content *string `json:"content,omitempty"`
}

// AddSignerRequest 添加签署人请求.
type AddSignerRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Role     string `json:"role" binding:"required"`
	Order    int    `json:"order"`
	SignType string `json:"sign_type"`
	Required bool   `json:"required"`
}

// SignRequest 签名请求.
type SignRequest struct {
	SignerID  string `json:"signer_id" binding:"required"`
	Signature string `json:"signature" binding:"required"`
	CertID    string `json:"cert_id,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// DeclineRequest 拒绝签名请求.
type DeclineRequest struct {
	SignerID string `json:"signer_id" binding:"required"`
	Reason   string `json:"reason" binding:"required"`
}

// CreateWorkflowRequest 创建工作流请求.
type CreateWorkflowRequest struct {
	Name        string         `json:"name" binding:"required"`
	Description string         `json:"description,omitempty"`
	Steps       []WorkflowStep `json:"steps"`
}

// UpdateWorkflowRequest 更新工作流请求.
type UpdateWorkflowRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Status      *string `json:"status,omitempty"`
}

// CreateCertRequest 创建证书请求.
type CreateCertRequest struct {
	UserID    string `json:"user_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Issuer    string `json:"issuer"`
	Algorithm string `json:"algorithm"` // RSA, ECDSA
	IsCA      bool   `json:"is_ca"`
	ValidDays int    `json:"valid_days"`
}

// SignWithCertRequest 使用证书签名请求.
type SignWithCertRequest struct {
	DocumentID string `json:"document_id" binding:"required"`
	CertID     string `json:"cert_id" binding:"required"`
	Algorithm  string `json:"algorithm"` // SHA256withRSA, SHA256withECDSA
}

// VerifySignatureRequest 验证签名请求.
type VerifySignatureRequest struct {
	SignatureID string `json:"signature_id" binding:"required"`
}

// VerifyResult 验证结果.
type VerifyResult struct {
	Valid       bool      `json:"valid"`
	SignatureID string    `json:"signature_id"`
	SignerID    string    `json:"signer_id"`
	Timestamp   time.Time `json:"timestamp"`
	CertValid   bool      `json:"cert_valid"`
	CertExpiry  time.Time `json:"cert_expiry"`
	Reason      string    `json:"reason,omitempty"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	Format string `json:"format" binding:"required"` // pdf, json, xml
}

// ExportResult 导出结果.
type ExportResult struct {
	Format   string `json:"format"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
}

// BulkSignRequest 批量签名请求.
type BulkSignRequest struct {
	DocumentIDs []string `json:"document_ids" binding:"required"`
	SignerID    string   `json:"signer_id" binding:"required"`
	Signature   string   `json:"signature" binding:"required"`
}

// BulkSignResult 批量签名结果.
type BulkSignResult struct {
	Success []string `json:"success"`
	Failed  []string `json:"failed"`
	Errors  map[string]string `json:"errors,omitempty"`
}

// DocumentStats 文档统计.
type DocumentStats struct {
	Total      int `json:"total"`
	Draft      int `json:"draft"`
	Pending    int `json:"pending"`
	Completed  int `json:"completed"`
	Cancelled  int `json:"cancelled"`
	Expired    int `json:"expired"`
}

// SignerStats 签署人统计.
type SignerStats struct {
	Total      int `json:"total"`
	Signed     int `json:"signed"`
	Pending    int `json:"pending"`
	Declined   int `json:"declined"`
}

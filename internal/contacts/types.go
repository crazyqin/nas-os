// Package contacts 提供联系人管理功能，参考群晖 Contacts 设计。
// 支持联系人 CRUD、分组管理、vCard 导入导出、搜索、去重、批量导入和分享功能。
package contacts

import "time"

// Contact 联系人.
type Contact struct {
	ID        string    `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	NickName  string    `json:"nick_name,omitempty"`
	Company   string    `json:"company,omitempty"`
	JobTitle  string    `json:"job_title,omitempty"`
	Phones    []Phone   `json:"phones,omitempty"`
	Emails    []Email   `json:"emails,omitempty"`
	Addresses []Address `json:"addresses,omitempty"`
	Groups    []string  `json:"groups,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Birthday  string    `json:"birthday,omitempty"`
	Website   string    `json:"website,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Phone 电话号码.
type Phone struct {
	Type   string `json:"type"` // home, work, mobile, fax, other
	Number string `json:"number"`
}

// Email 邮箱.
type Email struct {
	Type  string `json:"type"` // home, work, other
	Email string `json:"email"`
}

// Address 地址.
type Address struct {
	Type       string `json:"type"` // home, work, other
	Street     string `json:"street,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postal_code,omitempty"`
	Country    string `json:"country,omitempty"`
}

// ContactGroup 联系人分组.
type ContactGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Color       string    `json:"color,omitempty"`
	ContactIDs  []string  `json:"contact_ids,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ContactCreateRequest 创建联系人请求.
type ContactCreateRequest struct {
	FirstName string    `json:"first_name" binding:"required"`
	LastName  string    `json:"last_name"`
	NickName  string    `json:"nick_name,omitempty"`
	Company   string    `json:"company,omitempty"`
	JobTitle  string    `json:"job_title,omitempty"`
	Phones    []Phone   `json:"phones,omitempty"`
	Emails    []Email   `json:"emails,omitempty"`
	Addresses []Address `json:"addresses,omitempty"`
	Groups    []string  `json:"groups,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Birthday  string    `json:"birthday,omitempty"`
	Website   string    `json:"website,omitempty"`
}

// ContactUpdateRequest 更新联系人请求.
type ContactUpdateRequest struct {
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	NickName  string    `json:"nick_name,omitempty"`
	Company   string    `json:"company,omitempty"`
	JobTitle  string    `json:"job_title,omitempty"`
	Phones    []Phone   `json:"phones,omitempty"`
	Emails    []Email   `json:"emails,omitempty"`
	Addresses []Address `json:"addresses,omitempty"`
	Groups    []string  `json:"groups,omitempty"`
	Avatar    string    `json:"avatar,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	Birthday  string    `json:"birthday,omitempty"`
	Website   string    `json:"website,omitempty"`
}

// ContactGroupCreateRequest 创建分组请求.
type ContactGroupCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// ContactGroupUpdateRequest 更新分组请求.
type ContactGroupUpdateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query   string `json:"query" form:"q"`
	Name    string `json:"name" form:"name"`
	Phone   string `json:"phone" form:"phone"`
	Email   string `json:"email" form:"email"`
	Company string `json:"company" form:"company"`
	GroupID string `json:"group_id" form:"group_id"`
	Limit   int    `json:"limit" form:"limit"`
	Offset  int    `json:"offset" form:"offset"`
}

// BatchImportRequest 批量导入请求.
type BatchImportRequest struct {
	Format  string `json:"format"` // csv, vcard
	Content string `json:"content"`
	GroupID string `json:"group_id,omitempty"`
}

// DuplicateContact 重复联系人.
type DuplicateContact struct {
	Contact1 *Contact `json:"contact1"`
	Contact2 *Contact `json:"contact2"`
	Score    float64  `json:"score"` // 相似度评分 0-1
	Reasons  []string `json:"reasons"`
}

// ShareRequest 分享请求.
type ShareRequest struct {
	GroupID    string   `json:"group_id" binding:"required"`
	TargetUser []string `json:"target_users" binding:"required"`
	Permission string   `json:"permission"` // read, write
}

// VCard vCard 格式.
type VCard struct {
	Version   string    `json:"version"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	FullName  string    `json:"full_name"`
	Org       string    `json:"org,omitempty"`
	Title     string    `json:"title,omitempty"`
	Phones    []Phone   `json:"phones,omitempty"`
	Emails    []Email   `json:"emails,omitempty"`
	Addresses []Address `json:"addresses,omitempty"`
	Note      string    `json:"note,omitempty"`
	Birthday  string    `json:"birthday,omitempty"`
	URL       string    `json:"url,omitempty"`
	Photo     string    `json:"photo,omitempty"`
}

// MergeResult 合并结果.
type MergeResult struct {
	Kept     *Contact          `json:"kept"`
	Merged   []*Contact        `json:"merged"`
	FieldMap map[string]string `json:"field_map"`
}

// ShareInfo 分享信息.
type ShareInfo struct {
	ID         string    `json:"id"`
	GroupID    string    `json:"group_id"`
	GroupName  string    `json:"group_name"`
	SharedBy   string    `json:"shared_by"`
	SharedWith []string  `json:"shared_with"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
}

// ImportResult 导入结果.
type ImportResult struct {
	Total    int      `json:"total"`
	Imported int      `json:"imported"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
	Skipped  int      `json:"skipped"`
}

// ExportResult 导出结果.
type ExportResult struct {
	Format  string `json:"format"`
	Content string `json:"content"`
	Count   int    `json:"count"`
}

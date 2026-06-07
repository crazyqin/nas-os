// Package contactsmgr 提供通讯录管理功能
package contactsmgr

import (
	"time"
)

// Contact 联系人.
type Contact struct {
	ID         string     `json:"id"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	FullName   string     `json:"full_name"`
	Nickname   string     `json:"nickname,omitempty"`
	Emails     []Email    `json:"emails,omitempty"`
	Phones     []Phone    `json:"phones,omitempty"`
	Addresses  []Address  `json:"addresses,omitempty"`
	Company    string     `json:"company,omitempty"`
	Title      string     `json:"title,omitempty"` // 职位
	Department string     `json:"department,omitempty"`
	Avatar     string     `json:"avatar,omitempty"` // 头像URL
	Birthday   *time.Time `json:"birthday,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	Groups     []string   `json:"groups,omitempty"` // 所属组ID
	IsFavorite bool       `json:"is_favorite"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// Email 邮箱地址.
type Email struct {
	Type    string `json:"type"` // home, work, other
	Address string `json:"address"`
	Primary bool   `json:"primary"`
}

// Phone 电话号码.
type Phone struct {
	Type    string `json:"type"` // home, work, mobile, fax, other
	Number  string `json:"number"`
	Primary bool   `json:"primary"`
}

// Address 地址.
type Address struct {
	Type       string `json:"type"` // home, work, other
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`
	Primary    bool   `json:"primary"`
}

// ContactGroup 联系人组.
type ContactGroup struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Color        string    `json:"color,omitempty"`
	ContactCount int       `json:"contact_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// VCard vCard 格式数据.
type VCard struct {
	Version      string     `json:"version"` // 3.0 or 4.0
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	FullName     string     `json:"full_name"`
	Emails       []Email    `json:"emails,omitempty"`
	Phones       []Phone    `json:"phones,omitempty"`
	Addresses    []Address  `json:"addresses,omitempty"`
	Organization string     `json:"organization,omitempty"`
	Title        string     `json:"title,omitempty"`
	Birthday     *time.Time `json:"birthday,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	Photo        string     `json:"photo,omitempty"` // base64 编码
}

// DuplicateGroup 重复联系人组.
type DuplicateGroup struct {
	Contacts []*Contact `json:"contacts"`
	Field    string     `json:"field"` // name, email, phone
}

// ========== 请求/响应结构 ==========

// CreateContactRequest 创建联系人请求.
type CreateContactRequest struct {
	FirstName  string     `json:"first_name" binding:"required"`
	LastName   string     `json:"last_name"`
	Nickname   string     `json:"nickname,omitempty"`
	Emails     []Email    `json:"emails,omitempty"`
	Phones     []Phone    `json:"phones,omitempty"`
	Addresses  []Address  `json:"addresses,omitempty"`
	Company    string     `json:"company,omitempty"`
	Title      string     `json:"title,omitempty"`
	Department string     `json:"department,omitempty"`
	Birthday   *time.Time `json:"birthday,omitempty"`
	Notes      string     `json:"notes,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	GroupIDs   []string   `json:"group_ids,omitempty"`
}

// UpdateContactRequest 更新联系人请求.
type UpdateContactRequest struct {
	FirstName  *string    `json:"first_name,omitempty"`
	LastName   *string    `json:"last_name,omitempty"`
	Nickname   *string    `json:"nickname,omitempty"`
	Emails     []Email    `json:"emails,omitempty"`
	Phones     []Phone    `json:"phones,omitempty"`
	Addresses  []Address  `json:"addresses,omitempty"`
	Company    *string    `json:"company,omitempty"`
	Title      *string    `json:"title,omitempty"`
	Department *string    `json:"department,omitempty"`
	Birthday   *time.Time `json:"birthday,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	GroupIDs   []string   `json:"group_ids,omitempty"`
	IsFavorite *bool      `json:"is_favorite,omitempty"`
}

// CreateGroupRequest 创建联系人组请求.
type CreateGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color,omitempty"`
}

// UpdateGroupRequest 更新联系人组请求.
type UpdateGroupRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
}

// ImportVCardRequest 导入 vCard 请求.
type ImportVCardRequest struct {
	Content string `json:"content" binding:"required"` // vCard 格式内容
	GroupID string `json:"group_id"`                   // 导入到指定组
}

// ExportVCardRequest 导出 vCard 请求.
type ExportVCardRequest struct {
	ContactIDs []string `json:"contact_ids"` // 空则导出全部
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query   string `form:"q" binding:"required"`
	GroupID string `form:"group_id"`
}

// AddContactsToGroupRequest 添加联系人到组请求.
type AddContactsToGroupRequest struct {
	ContactIDs []string `json:"contact_ids" binding:"required"`
}

// DeduplicateRequest 去重请求.
type DeduplicateRequest struct {
	Field string `json:"field"` // name, email, phone, auto
}

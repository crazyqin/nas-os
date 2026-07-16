// Package personalcrm 个人关系管理（Personal CRM）
// 管理联系人、互动记录、生日提醒、关系图谱等功能
package personalcrm

import (
	"time"
)

// ContactGroup 联系人分组类型.
type ContactGroup string

const (
	GroupFamily    ContactGroup = "family"    // 家人
	GroupFriend    ContactGroup = "friend"    // 朋友
	GroupColleague ContactGroup = "colleague" // 同事
	GroupClient    ContactGroup = "client"    // 客户
	GroupOther     ContactGroup = "other"     // 其他
)

// InteractionType 互动类型.
type InteractionType string

const (
	InteractionCall   InteractionType = "call"   // 通话
	InteractionMeet   InteractionType = "meet"   // 见面
	InteractionEmail  InteractionType = "email"  // 邮件
	InteractionWechat InteractionType = "wechat" // 微信
	InteractionSMS    InteractionType = "sms"    // 短信
	InteractionOther  InteractionType = "other"  // 其他
)

// RelationType 关系类型.
type RelationType string

const (
	RelationSpouse    RelationType = "spouse"    // 配偶
	RelationParent    RelationType = "parent"    // 父母
	RelationChild     RelationType = "child"     // 子女
	RelationSibling   RelationType = "sibling"   // 兄弟姐妹
	RelationFriend    RelationType = "friend"    // 朋友
	RelationColleague RelationType = "colleague" // 同事
	RelationClient    RelationType = "client"    // 客户
	RelationPartner   RelationType = "partner"   // 合作伙伴
	RelationOther     RelationType = "other"     // 其他
)

// Contact 联系人.
type Contact struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`               // 姓名
	Phone     string         `json:"phone"`              // 电话
	Email     string         `json:"email"`              // 邮箱
	Company   string         `json:"company"`            // 公司
	Birthday  *time.Time     `json:"birthday,omitempty"` // 生日
	Avatar    string         `json:"avatar"`             // 头像URL
	Notes     string         `json:"notes"`              // 备注
	Tags      []string       `json:"tags"`               // 标签
	Groups    []ContactGroup `json:"groups"`             // 分组
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Anniversary 纪念日.
type Anniversary struct {
	ID         string    `json:"id"`
	ContactID  string    `json:"contact_id"`  // 关联联系人ID
	Name       string    `json:"name"`        // 纪念日名称
	Date       time.Time `json:"date"`        // 日期
	RemindDays int       `json:"remind_days"` // 提前提醒天数
	IsYearly   bool      `json:"is_yearly"`   // 是否每年重复
	Notes      string    `json:"notes"`       // 备注
	CreatedAt  time.Time `json:"created_at"`
}

// Interaction 互动记录.
type Interaction struct {
	ID        string          `json:"id"`
	ContactID string          `json:"contact_id"` // 关联联系人ID
	Type      InteractionType `json:"type"`       // 互动类型
	Time      time.Time       `json:"time"`       // 互动时间
	Notes     string          `json:"notes"`      // 备注
	CreatedAt time.Time       `json:"created_at"`
}

// Relationship 联系人之间的关系.
type Relationship struct {
	ID         string       `json:"id"`
	ContactID1 string       `json:"contact_id_1"` // 联系人1 ID
	ContactID2 string       `json:"contact_id_2"` // 联系人2 ID
	Type       RelationType `json:"type"`         // 关系类型
	Notes      string       `json:"notes"`        // 备注
	CreatedAt  time.Time    `json:"created_at"`
}

// Reminder 提醒记录.
type Reminder struct {
	ID        string    `json:"id"`
	ContactID string    `json:"contact_id"` // 关联联系人ID
	Type      string    `json:"type"`       // birthday, anniversary
	Title     string    `json:"title"`      // 提醒标题
	DueDate   time.Time `json:"due_date"`   // 到期日期
	RemindAt  time.Time `json:"remind_at"`  // 提醒时间
	IsSent    bool      `json:"is_sent"`    // 是否已发送
	CreatedAt time.Time `json:"created_at"`
}

// ContactStats 联系人统计.
type ContactStats struct {
	ContactID         string     `json:"contact_id"`
	TotalInteractions int        `json:"total_interactions"`   // 总互动次数
	LastInteraction   *time.Time `json:"last_interaction"`     // 最近一次互动
	InteractionFreq   float64    `json:"interaction_freq"`     // 互动频率（次/月）
	ClosenessScore    float64    `json:"closeness_score"`      // 关系亲密度评分（0-100）
	DaysSinceLastMeet int        `json:"days_since_last_meet"` // 距上次见面天数
}

// SystemStats 系统统计.
type SystemStats struct {
	TotalContacts      int     `json:"total_contacts"`      // 总联系人数
	TotalInteractions  int     `json:"total_interactions"`  // 总互动次数
	TotalRelationships int     `json:"total_relationships"` // 总关系数
	AvgClosenessScore  float64 `json:"avg_closeness_score"` // 平均亲密度评分
	UpcomingReminders  int     `json:"upcoming_reminders"`  // 即将到来的提醒数
}

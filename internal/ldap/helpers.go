package ldap

import (
	"github.com/go-ldap/ldap/v3"
)

// createSearchRequest 创建搜索请求的辅助函数
func createSearchRequest(baseDN, filter string, attributes []string) *ldap.SearchRequest {
	return ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		attributes,
		nil,
	)
}

// MergeAttributeMaps 合并属性映射
func MergeAttributeMaps(base, override AttributeMapping) AttributeMapping {
	result := base

	if override.Username != "" {
		result.Username = override.Username
	}
	if override.Email != "" {
		result.Email = override.Email
	}
	if override.FirstName != "" {
		result.FirstName = override.FirstName
	}
	if override.LastName != "" {
		result.LastName = override.LastName
	}
	if override.FullName != "" {
		result.FullName = override.FullName
	}
	if override.DisplayName != "" {
		result.DisplayName = override.DisplayName
	}
	if override.Phone != "" {
		result.Phone = override.Phone
	}
	if override.Mobile != "" {
		result.Mobile = override.Mobile
	}
	if override.Department != "" {
		result.Department = override.Department
	}
	if override.Title != "" {
		result.Title = override.Title
	}
	if override.EmployeeID != "" {
		result.EmployeeID = override.EmployeeID
	}
	if override.GroupName != "" {
		result.GroupName = override.GroupName
	}
	if override.GroupDescription != "" {
		result.GroupDescription = override.GroupDescription
	}
	if override.GroupMember != "" {
		result.GroupMember = override.GroupMember
	}
	if override.MemberAttribute != "" {
		result.MemberAttribute = override.MemberAttribute
	}
	if override.MemberOfAttribute != "" {
		result.MemberOfAttribute = override.MemberOfAttribute
	}
	if override.UserObjectClass != "" {
		result.UserObjectClass = override.UserObjectClass
	}
	if override.GroupObjectClass != "" {
		result.GroupObjectClass = override.GroupObjectClass
	}

	return result
}

// ValidateConfig 验证配置
func ValidateConfig(config Config) error {
	if config.Name == "" {
		return ErrInvalidConfig
	}

	if config.URL == "" {
		return ErrInvalidConfig
	}

	if config.BaseDN == "" {
		return ErrInvalidConfig
	}

	return nil
}

// GetAttributeValueSafe 安全获取属性值
func GetAttributeValueSafe(entry *ldap.Entry, attr string) string {
	if entry == nil {
		return ""
	}
	return entry.GetAttributeValue(attr)
}

// GetAttributeValuesSafe 安全获取属性值列表
func GetAttributeValuesSafe(entry *ldap.Entry, attr string) []string {
	if entry == nil {
		return []string{}
	}
	return entry.GetAttributeValues(attr)
}
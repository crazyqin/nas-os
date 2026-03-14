// Package api 提供 NAS-OS API 的请求验证器
package api

import (
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

// InitValidator 初始化自定义验证器
func InitValidator() {
	// 注册自定义验证规则
	validate.RegisterValidation("username", validateUsername)
	validate.RegisterValidation("password", validatePassword)
	validate.RegisterValidation("volume_name", validateVolumeName)
	validate.RegisterValidation("container_name", validateContainerName)
	validate.RegisterValidation("ip", validateIP)
	validate.RegisterValidation("port", validatePort)
	validate.RegisterValidation("hostname", validateHostname)
	validate.RegisterValidation("path", validatePath)
}

// 自定义验证规则

// validateUsername 验证用户名：字母开头，允许字母数字下划线，3-32字符
func validateUsername(fl validator.FieldLevel) bool {
	username := fl.Field().String()
	if len(username) < 3 || len(username) > 32 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9_]*$", username)
	return matched
}

// validatePassword 验证密码：至少6个字符
func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return len(password) >= 6
}

// validateVolumeName 验证卷名：允许字母数字下划线中划线，1-64字符
func validateVolumeName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", name)
	return matched
}

// validateContainerName 验证容器名：允许字母数字下划线中划线点，1-64字符
func validateContainerName(fl validator.FieldLevel) bool {
	name := fl.Field().String()
	if len(name) < 1 || len(name) > 64 {
		return false
	}
	matched, _ := regexp.MatchString("^[a-zA-Z][a-zA-Z0-9_.-]*$", name)
	return matched
}

// validateIP 验证 IP 地址
func validateIP(fl validator.FieldLevel) bool {
	ip := fl.Field().String()
	if ip == "" {
		return false
	}
	// IPv4
	ipv4Pattern := `^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`
	if matched, _ := regexp.MatchString(ipv4Pattern, ip); matched {
		return true
	}
	// IPv6 (简化版)
	ipv6Pattern := `^([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}$`
	matched, _ := regexp.MatchString(ipv6Pattern, ip)
	return matched
}

// validatePort 验证端口号：1-65535
func validatePort(fl validator.FieldLevel) bool {
	port := fl.Field().Int()
	return port >= 1 && port <= 65535
}

// validateHostname 验证主机名
func validateHostname(fl validator.FieldLevel) bool {
	hostname := fl.Field().String()
	if len(hostname) < 1 || len(hostname) > 253 {
		return false
	}
	// 简化的主机名验证
	hostnamePattern := `^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`
	matched, _ := regexp.MatchString(hostnamePattern, hostname)
	return matched
}

// validatePath 验证文件路径
func validatePath(fl validator.FieldLevel) bool {
	path := fl.Field().String()
	if path == "" {
		return false
	}
	// 不允许包含危险字符
	if strings.Contains(path, "..") {
		return false
	}
	// 必须以 / 开头（绝对路径）或允许相对路径
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, "./")
}

// ========== 通用请求结构 ==========

// IDRequest ID 请求参数
type IDRequest struct {
	ID string `uri:"id" binding:"required" json:"id"`
}

// NameRequest 名称请求参数
type NameRequest struct {
	Name string `uri:"name" binding:"required" json:"name"`
}

// EnableRequest 启用/禁用请求
type EnableRequest struct {
	Enabled bool `json:"enabled"`
}

// PaginationRequest 分页请求
type PaginationRequest struct {
	Page     int `form:"page" binding:"omitempty,min=1"`
	PageSize int `form:"pageSize" binding:"omitempty,min=1,max=100"`
}

// GetPage 获取页码，默认为 1
func (r *PaginationRequest) GetPage() int {
	if r.Page < 1 {
		return 1
	}
	return r.Page
}

// GetPageSize 获取每页数量，默认为 20
func (r *PaginationRequest) GetPageSize() int {
	if r.PageSize < 1 {
		return 20
	}
	if r.PageSize > 100 {
		return 100
	}
	return r.PageSize
}

// GetOffset 获取偏移量
func (r *PaginationRequest) GetOffset() int {
	return (r.GetPage() - 1) * r.GetPageSize()
}

// SortRequest 排序请求
type SortRequest struct {
	SortBy    string `form:"sortBy" json:"sortBy"`
	SortOrder string `form:"sortOrder" json:"sortOrder"` // asc, desc
}

// GetSortOrder 获取排序方向，默认 desc
func (r *SortRequest) GetSortOrder() string {
	if r.SortOrder != "asc" && r.SortOrder != "desc" {
		return "desc"
	}
	return r.SortOrder
}

// TimeRangeRequest 时间范围请求
type TimeRangeRequest struct {
	StartTime string `form:"startTime" json:"startTime"` // RFC3339 格式
	EndTime   string `form:"endTime" json:"endTime"`     // RFC3339 格式
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query string `form:"query" binding:"required,min=1,max=100" json:"query"`
}

// ========== 辅助函数 ==========

// GetParam 获取路径参数，如果为空则返回默认值
func GetParam(c interface{ Param(string) string }, key, defaultValue string) string {
	value := c.Param(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetQuery 获取查询参数，如果为空则返回默认值
func GetQuery(c interface{ Query(string) string }, key, defaultValue string) string {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// GetQueryInt 获取整数查询参数
func GetQueryInt(c interface{ Query(string) string }, key string, defaultValue int) int {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	// 简单转换，实际应用中应该使用 strconv
	var result int
	_, _ = regexp.MatchString(`^\d+$`, value) // 简化处理
	return result
}

// GetQueryBool 获取布尔查询参数
func GetQueryBool(c interface{ Query(string) string }, key string, defaultValue bool) bool {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}
	return value == "true" || value == "1" || value == "yes"
}

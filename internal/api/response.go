// Package api 提供 NAS-OS API 的通用响应和错误处理
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 通用 API 响应结构
type Response struct {
	Code    int         `json:"code" example:"0"`              // 业务码：0=成功，非0=失败
	Message string      `json:"message" example:"success"`     // 响应消息
	Data    interface{} `json:"data,omitempty"`                // 响应数据
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`         // 错误码
	Message string `json:"message" example:"请求错误"` // 错误消息
}

// PageData 分页数据结构
type PageData struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages,omitempty"`
}

// 业务错误码定义
const (
	CodeSuccess         = 0     // 成功
	CodeBadRequest      = 400   // 请求参数错误
	CodeUnauthorized    = 401   // 未授权
	CodeForbidden       = 403   // 禁止访问
	CodeNotFound        = 404   // 资源不存在
	CodeConflict        = 409   // 资源冲突
	CodeTooManyRequests = 429   // 请求过多
	CodeInternalError   = 500   // 服务器内部错误
	CodeServiceUnavailable = 503 // 服务不可用
)

// ========== 响应辅助函数 ==========

// Success 返回成功响应
func Success(data interface{}) Response {
	return Response{
		Code:    CodeSuccess,
		Message: "success",
		Data:    data,
	}
}

// SuccessWithMessage 返回带自定义消息的成功响应
func SuccessWithMessage(message string, data interface{}) Response {
	return Response{
		Code:    CodeSuccess,
		Message: message,
		Data:    data,
	}
}

// Error 返回错误响应
func Error(code int, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

// ErrorWithData 返回带数据的错误响应
func ErrorWithData(code int, message string, data interface{}) Response {
	return Response{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ========== Gin 上下文响应方法 ==========

// OK 返回 200 成功响应
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Success(data))
}

// OKWithMessage 返回 200 成功响应（带消息）
func OKWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, SuccessWithMessage(message, data))
}

// Created 返回 201 创建成功响应
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Success(data))
}

// CreatedWithMessage 返回 201 创建成功响应（带消息）
func CreatedWithMessage(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, SuccessWithMessage(message, data))
}

// NoContent 返回 204 无内容响应
func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// BadRequest 返回 400 错误请求响应
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Error(CodeBadRequest, message))
}

// Unauthorized 返回 401 未授权响应
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "未授权"
	}
	c.JSON(http.StatusUnauthorized, Error(CodeUnauthorized, message))
}

// Forbidden 返回 403 禁止访问响应
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "禁止访问"
	}
	c.JSON(http.StatusForbidden, Error(CodeForbidden, message))
}

// NotFound 返回 404 未找到响应
func NotFound(c *gin.Context, message string) {
	if message == "" {
		message = "资源不存在"
	}
	c.JSON(http.StatusNotFound, Error(CodeNotFound, message))
}

// Conflict 返回 409 冲突响应
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, Error(CodeConflict, message))
}

// InternalError 返回 500 内部错误响应
func InternalError(c *gin.Context, message string) {
	if message == "" {
		message = "服务器内部错误"
	}
	c.JSON(http.StatusInternalServerError, Error(CodeInternalError, message))
}

// ServiceUnavailable 返回 503 服务不可用响应
func ServiceUnavailable(c *gin.Context, message string) {
	if message == "" {
		message = "服务暂时不可用"
	}
	c.JSON(http.StatusServiceUnavailable, Error(CodeServiceUnavailable, message))
}

// ========== 分页响应 ==========

// Page 返回分页数据响应
func Page(c *gin.Context, items interface{}, total int64, page, pageSize int) {
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	c.JSON(http.StatusOK, Success(PageData{
		Items:      items,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}))
}

// ========== 错误处理辅助函数 ==========

// HandleError 统一错误处理
// 根据 error 类型自动选择合适的 HTTP 状态码和消息
func HandleError(c *gin.Context, err error, notFoundMsg string) {
	if err == nil {
		return
	}

	// 检查是否是已定义的错误类型
	switch {
	case isNotFoundError(err):
		NotFound(c, notFoundMsg)
	case isBadRequestError(err):
		BadRequest(c, err.Error())
	case isConflictError(err):
		Conflict(c, err.Error())
	case isForbiddenError(err):
		Forbidden(c, err.Error())
	case isUnauthorizedError(err):
		Unauthorized(c, err.Error())
	default:
		InternalError(c, err.Error())
	}
}

// 错误类型检查接口（可由各模块实现）
type (
	notFoundChecker     interface{ NotFound() bool }
	badRequestChecker   interface{ BadRequest() bool }
	conflictChecker     interface{ Conflict() bool }
	forbiddenChecker    interface{ Forbidden() bool }
	unauthorizedChecker interface{ Unauthorized() bool }
)

func isNotFoundError(err error) bool {
	if e, ok := err.(notFoundChecker); ok {
		return e.NotFound()
	}
	return false
}

func isBadRequestError(err error) bool {
	if e, ok := err.(badRequestChecker); ok {
		return e.BadRequest()
	}
	return false
}

func isConflictError(err error) bool {
	if e, ok := err.(conflictChecker); ok {
		return e.Conflict()
	}
	return false
}

func isForbiddenError(err error) bool {
	if e, ok := err.(forbiddenChecker); ok {
		return e.Forbidden()
	}
	return false
}

func isUnauthorizedError(err error) bool {
	if e, ok := err.(unauthorizedChecker); ok {
		return e.Unauthorized()
	}
	return false
}
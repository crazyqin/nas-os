// Package api 提供 NAS-OS API 的通用响应、错误处理和请求验证
package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ========== 响应结构 ==========

// Response 通用 API 响应结构
type Response struct {
	Code    int         `json:"code" example:"0"`          // 业务码：0=成功，非0=失败
	Message string      `json:"message" example:"success"` // 响应消息
	Data    interface{} `json:"data,omitempty"`            // 响应数据
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code    int                    `json:"code" example:"400"`     // 错误码
	Message string                 `json:"message" example:"请求错误"` // 错误消息
	Details map[string]interface{} `json:"details,omitempty"`      // 详细错误信息
}

// PageData 分页数据结构
type PageData struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages,omitempty"`
}

// ========== 业务错误码定义 ==========

const (
	CodeSuccess            = 0   // 成功
	CodeBadRequest         = 400 // 请求参数错误
	CodeUnauthorized       = 401 // 未授权
	CodeForbidden          = 403 // 禁止访问
	CodeNotFound           = 404 // 资源不存在
	CodeMethodNotAllowed   = 405 // 方法不允许
	CodeConflict           = 409 // 资源冲突
	CodeTooManyRequests    = 429 // 请求过多
	CodeInternalError      = 500 // 服务器内部错误
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

// ErrorWithDetails 返回带详细信息的错误响应
func ErrorWithDetails(code int, message string, details map[string]interface{}) Response {
	return Response{
		Code:    code,
		Message: message,
		Data:    details,
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

// BadRequestWithDetails 返回 400 错误请求响应（带详细信息）
func BadRequestWithDetails(c *gin.Context, message string, details map[string]interface{}) {
	c.JSON(http.StatusBadRequest, ErrorWithDetails(CodeBadRequest, message, details))
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

// MethodNotAllowed 返回 405 方法不允许响应
func MethodNotAllowed(c *gin.Context, message string) {
	if message == "" {
		message = "方法不允许"
	}
	c.JSON(http.StatusMethodNotAllowed, Error(CodeMethodNotAllowed, message))
}

// Conflict 返回 409 冲突响应
func Conflict(c *gin.Context, message string) {
	c.JSON(http.StatusConflict, Error(CodeConflict, message))
}

// TooManyRequests 返回 429 请求过多响应
func TooManyRequests(c *gin.Context, message string) {
	if message == "" {
		message = "请求过于频繁"
	}
	c.JSON(http.StatusTooManyRequests, Error(CodeTooManyRequests, message))
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

// ========== 标准错误类型 ==========

// APIError 实现 error 接口的标准 API 错误
type APIError struct {
	Code    int
	Message string
	Err     error // 原始错误
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// NewAPIError 创建 API 错误
func NewAPIError(code int, message string, err ...error) *APIError {
	apiErr := &APIError{
		Code:    code,
		Message: message,
	}
	if len(err) > 0 {
		apiErr.Err = err[0]
	}
	return apiErr
}

// 预定义错误
var (
	ErrBadRequest         = &APIError{Code: CodeBadRequest, Message: "请求参数错误"}
	ErrUnauthorized       = &APIError{Code: CodeUnauthorized, Message: "未授权"}
	ErrForbidden          = &APIError{Code: CodeForbidden, Message: "禁止访问"}
	ErrNotFound           = &APIError{Code: CodeNotFound, Message: "资源不存在"}
	ErrConflict           = &APIError{Code: CodeConflict, Message: "资源冲突"}
	ErrInternal           = &APIError{Code: CodeInternalError, Message: "服务器内部错误"}
	ErrServiceUnavailable = &APIError{Code: CodeServiceUnavailable, Message: "服务不可用"}
)

// NewBadRequestError 创建 400 错误
func NewBadRequestError(message string, err ...error) *APIError {
	return NewAPIError(CodeBadRequest, message, err...)
}

// NewNotFoundError 创建 404 错误
func NewNotFoundError(message string, err ...error) *APIError {
	return NewAPIError(CodeNotFound, message, err...)
}

// NewConflictError 创建 409 错误
func NewConflictError(message string, err ...error) *APIError {
	return NewAPIError(CodeConflict, message, err...)
}

// NewInternalError 创建 500 错误
func NewInternalError(message string, err ...error) *APIError {
	return NewAPIError(CodeInternalError, message, err...)
}

// ========== 统一错误处理 ==========

// HandleError 统一错误处理
// 根据 error 类型自动选择合适的 HTTP 状态码和消息
func HandleError(c *gin.Context, err error, notFoundMsg string) {
	if err == nil {
		return
	}

	// 检查是否是 APIError 类型
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		c.JSON(httpStatusFromCode(apiErr.Code), Error(apiErr.Code, apiErr.Message))
		return
	}

	// 检查是否是已定义的错误类型（通过接口）
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

// httpStatusFromCode 根据业务码返回 HTTP 状态码
func httpStatusFromCode(code int) int {
	switch code {
	case CodeSuccess:
		return http.StatusOK
	case CodeBadRequest:
		return http.StatusBadRequest
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeMethodNotAllowed:
		return http.StatusMethodNotAllowed
	case CodeConflict:
		return http.StatusConflict
	case CodeTooManyRequests:
		return http.StatusTooManyRequests
	case CodeInternalError:
		return http.StatusInternalServerError
	case CodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
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

// ========== 请求验证 ==========

var validate = validator.New()

// Validate 验证结构体
func Validate(s interface{}) error {
	return validate.Struct(s)
}

// ValidateVar 验证单个变量
func ValidateVar(field interface{}, tag string) error {
	return validate.Var(field, tag)
}

// BindAndValidate 绑定并验证请求
func BindAndValidate(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindJSON(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	if err := Validate(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	return nil
}

// BindQueryAndValidate 绑定查询参数并验证
func BindQueryAndValidate(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindQuery(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	if err := Validate(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	return nil
}

// BindURIAndValidate 绑定 URI 参数并验证
func BindURIAndValidate(c *gin.Context, req interface{}) error {
	if err := c.ShouldBindUri(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	if err := Validate(req); err != nil {
		return NewBadRequestError(formatValidationError(err), err)
	}
	return nil
}

// formatValidationError 格式化验证错误
func formatValidationError(err error) string {
	if err == nil {
		return ""
	}

	// 处理 validator.ValidationErrors
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range validationErrors {
			switch fe.Tag() {
			case "required":
				return fmt.Sprintf("%s 不能为空", fe.Field())
			case "email":
				return fmt.Sprintf("%s 格式不正确", fe.Field())
			case "min":
				return fmt.Sprintf("%s 长度不能小于 %s", fe.Field(), fe.Param())
			case "max":
				return fmt.Sprintf("%s 长度不能大于 %s", fe.Field(), fe.Param())
			case "len":
				return fmt.Sprintf("%s 长度必须为 %s", fe.Field(), fe.Param())
			case "gte":
				return fmt.Sprintf("%s 必须大于或等于 %s", fe.Field(), fe.Param())
			case "lte":
				return fmt.Sprintf("%s 必须小于或等于 %s", fe.Field(), fe.Param())
			case "oneof":
				return fmt.Sprintf("%s 必须是 %s 之一", fe.Field(), fe.Param())
			default:
				return fmt.Sprintf("%s 验证失败: %s", fe.Field(), fe.Tag())
			}
		}
	}

	return err.Error()
}

// ========== 请求验证中间件 ==========

// ValidateRequest 请求验证中间件
func ValidateRequest(req interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := BindAndValidate(c, req); err != nil {
			BadRequest(c, err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}

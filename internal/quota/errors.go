package quota

import (
	"github.com/gin-gonic/gin"
)

// ========== 错误接口实现 ==========

// NotFound 实现 api.notFoundChecker 接口
func (e *QuotaError) NotFound() bool {
	return e.code == ErrCodeQuotaNotFound || e.code == ErrCodePolicyNotFound
}

// BadRequest 实现 api.badRequestChecker 接口
func (e *QuotaError) BadRequest() bool {
	return e.code == ErrCodeInvalidInput || e.code == ErrCodeInvalidLimit
}

// Conflict 实现 api.conflictChecker 接口
func (e *QuotaError) Conflict() bool {
	return e.code == ErrCodeQuotaExists
}

// ========== 错误码定义 ==========

const (
	// ErrCodeQuotaNotFound 配额未找到错误码
	ErrCodeQuotaNotFound = 1001
	ErrCodeQuotaExists   = 1002
	ErrCodeQuotaExceeded = 1003
	ErrCodeUserNotFound  = 1004
	ErrCodeGroupNotFound = 1005
	// ErrCodeVolumeNotFound 卷未找到错误码
	ErrCodeVolumeNotFound = 1006
	ErrCodeInvalidLimit   = 1007
	ErrCodePolicyNotFound = 1008
	ErrCodeInvalidInput   = 1009
	ErrCodeAlertNotFound  = 1010
)

// QuotaError 配额错误
//
//nolint:revive // 保留 QuotaError 名称以避免与内置 error 类型混淆
type QuotaError struct {
	code    int
	message string
	details map[string]interface{}
}

// Error 实现 error 接口
func (e *QuotaError) Error() string {
	return e.message
}

// Code 返回错误码
func (e *QuotaError) Code() int {
	return e.code
}

// Details 返回错误详情
func (e *QuotaError) Details() map[string]interface{} {
	return e.details
}

// NewQuotaError 创建配额错误
func NewQuotaError(code int, message string, details ...map[string]interface{}) *QuotaError {
	e := &QuotaError{
		code:    code,
		message: message,
	}
	if len(details) > 0 {
		e.details = details[0]
	}
	return e
}

// 预定义错误
var (
	ErrQuotaNotFoundAPI  = &QuotaError{code: ErrCodeQuotaNotFound, message: "配额不存在"}
	ErrQuotaExistsAPI    = &QuotaError{code: ErrCodeQuotaExists, message: "配额已存在"}
	ErrQuotaExceededAPI  = &QuotaError{code: ErrCodeQuotaExceeded, message: "超出配额限制"}
	ErrUserNotFoundAPI   = &QuotaError{code: ErrCodeUserNotFound, message: "用户不存在"}
	ErrGroupNotFoundAPI  = &QuotaError{code: ErrCodeGroupNotFound, message: "用户组不存在"}
	ErrVolumeNotFoundAPI = &QuotaError{code: ErrCodeVolumeNotFound, message: "卷不存在"}
	ErrInvalidLimitAPI   = &QuotaError{code: ErrCodeInvalidLimit, message: "无效的配额限制"}
	ErrPolicyNotFoundAPI = &QuotaError{code: ErrCodePolicyNotFound, message: "清理策略不存在"}
	ErrInvalidInputAPI   = &QuotaError{code: ErrCodeInvalidInput, message: "无效的输入参数"}
)

// WithDetails 添加错误详情
func (e *QuotaError) WithDetails(details map[string]interface{}) *QuotaError {
	e.details = details
	return e
}

// ========== 错误转换 ==========

// ToAPIError 转换标准错误为 API 错误
func ToAPIError(err error) *QuotaError {
	if err == nil {
		return nil
	}

	// 已经是 QuotaError
	if qe, ok := err.(*QuotaError); ok {
		return qe
	}

	// 转换标准错误
	switch err {
	case ErrQuotaNotFound:
		return ErrQuotaNotFoundAPI
	case ErrQuotaExists:
		return ErrQuotaExistsAPI
	case ErrQuotaExceeded:
		return ErrQuotaExceededAPI
	case ErrUserNotFound:
		return ErrUserNotFoundAPI
	case ErrGroupNotFound:
		return ErrGroupNotFoundAPI
	case ErrVolumeNotFound:
		return ErrVolumeNotFoundAPI
	case ErrInvalidLimit:
		return ErrInvalidLimitAPI
	case ErrCleanupPolicyNotFound:
		return ErrPolicyNotFoundAPI
	default:
		return NewQuotaError(ErrCodeInvalidInput, err.Error())
	}
}

// ========== 错误响应辅助函数 ==========

// ErrorResponse 返回错误响应
func ErrorResponse(c *gin.Context, err error) {
	qe := ToAPIError(err)
	if qe == nil {
		return
	}

	status := 400
	switch qe.code {
	case ErrCodeQuotaNotFound, ErrCodePolicyNotFound:
		status = 404
	case ErrCodeQuotaExists:
		status = 409
	}

	response := gin.H{
		"code":    qe.code,
		"message": qe.message,
	}

	if qe.details != nil {
		response["details"] = qe.details
	}

	c.JSON(status, response)
}

// ========== 输入验证 ==========

// ValidateQuotaInput 验证配额输入
func ValidateQuotaInput(input QuotaInput) error {
	if input.Type == "" {
		return NewQuotaError(ErrCodeInvalidInput, "配额类型不能为空")
	}

	if input.Type != QuotaTypeUser && input.Type != QuotaTypeGroup && input.Type != QuotaTypeDirectory {
		return NewQuotaError(ErrCodeInvalidInput, "无效的配额类型")
	}

	if input.TargetID == "" {
		return NewQuotaError(ErrCodeInvalidInput, "目标 ID 不能为空")
	}

	if input.HardLimit == 0 {
		return NewQuotaError(ErrCodeInvalidInput, "硬限制不能为零")
	}

	if input.SoftLimit > input.HardLimit {
		return NewQuotaError(ErrCodeInvalidInput, "软限制不能大于硬限制")
	}

	return nil
}

// ValidateCleanupPolicyInput 验证清理策略输入
func ValidateCleanupPolicyInput(input CleanupPolicyInput) error {
	if input.Name == "" {
		return NewQuotaError(ErrCodeInvalidInput, "策略名称不能为空")
	}

	if input.VolumeName == "" {
		return NewQuotaError(ErrCodeInvalidInput, "卷名不能为空")
	}

	if input.Type == "" {
		return NewQuotaError(ErrCodeInvalidInput, "策略类型不能为空")
	}

	if input.Action == "" {
		return NewQuotaError(ErrCodeInvalidInput, "清理动作不能为空")
	}

	// 验证策略参数
	switch input.Type {
	case CleanupPolicyAge:
		if input.MaxAge <= 0 {
			return NewQuotaError(ErrCodeInvalidInput, "最大保留天数必须大于零")
		}
	case CleanupPolicySize:
		if input.MinSize <= 0 {
			return NewQuotaError(ErrCodeInvalidInput, "最小文件大小必须大于零")
		}
	case CleanupPolicyQuota:
		if input.QuotaPercent <= 0 || input.QuotaPercent > 100 {
			return NewQuotaError(ErrCodeInvalidInput, "配额百分比必须在 1-100 之间")
		}
	case CleanupPolicyAccess:
		if input.MaxAccessAge <= 0 {
			return NewQuotaError(ErrCodeInvalidInput, "最大未访问天数必须大于零")
		}
	}

	return nil
}

// ========== 常用验证器 ==========

// ValidateID 验证 ID
func ValidateID(id string) error {
	if id == "" {
		return NewQuotaError(ErrCodeInvalidInput, "ID 不能为空")
	}
	if len(id) > 64 {
		return NewQuotaError(ErrCodeInvalidInput, "ID 长度不能超过 64")
	}
	return nil
}

// ValidatePath 验证路径
func ValidatePath(path string) error {
	if path == "" {
		return NewQuotaError(ErrCodeInvalidInput, "路径不能为空")
	}
	if len(path) > 1024 {
		return NewQuotaError(ErrCodeInvalidInput, "路径长度不能超过 1024")
	}
	return nil
}

// ValidateVolumeName 验证卷名
func ValidateVolumeName(name string) error {
	if name == "" {
		return NewQuotaError(ErrCodeInvalidInput, "卷名不能为空")
	}
	if len(name) > 64 {
		return NewQuotaError(ErrCodeInvalidInput, "卷名长度不能超过 64")
	}
	return nil
}

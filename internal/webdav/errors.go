package webdav

import "errors"

var (
	// ErrLocked 资源已被锁定.
	ErrLocked = errors.New("资源已被锁定")
	// ErrLockNotFound 锁不存在.
	ErrLockNotFound = errors.New("锁不存在")
	// ErrInvalidLockToken 无效的锁令牌.
	ErrInvalidLockToken = errors.New("无效的锁令牌")
	// ErrQuotaExceeded 配额超出.
	ErrQuotaExceeded = errors.New("配额超出")
	// ErrPermissionDenied 权限被拒绝.
	ErrPermissionDenied = errors.New("权限被拒绝")
	// ErrPathTraversal 路径遍历攻击.
	ErrPathTraversal = errors.New("无效的路径")
	// ErrCopyFailed 复制失败.
	ErrCopyFailed = errors.New("复制失败")
	// ErrMoveFailed 移动失败.
	ErrMoveFailed = errors.New("移动失败")
)

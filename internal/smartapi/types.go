package smartapi

import "errors"

// ErrInvalidKeyID 无效的API密钥ID.
var ErrInvalidKeyID = errors.New("invalid key ID")

// ErrKeyNotFound API密钥未找到.
var ErrKeyNotFound = errors.New("key not found")

// ErrKeyExpired API密钥已过期.
var ErrKeyExpired = errors.New("key expired")

// ErrInvalidConfigID 无效的配置ID.
var ErrInvalidConfigID = errors.New("invalid config ID")

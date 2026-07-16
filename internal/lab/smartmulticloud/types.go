package smartmulticloud

import "errors"

// ErrInvalidAccountID 无效的账号ID.
var ErrInvalidAccountID = errors.New("invalid account ID")

// ErrAccountNotFound 账号未找到.
var ErrAccountNotFound = errors.New("account not found")

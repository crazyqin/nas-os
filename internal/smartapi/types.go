package smartapi

import "errors"

var (
	ErrInvalidKeyID    = errors.New("invalid key ID")
	ErrKeyNotFound     = errors.New("key not found")
	ErrKeyExpired      = errors.New("key expired")
	ErrInvalidConfigID = errors.New("invalid config ID")
)

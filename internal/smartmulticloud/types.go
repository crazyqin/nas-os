package smartmulticloud

import "errors"

var (
	ErrInvalidAccountID = errors.New("invalid account ID")
	ErrAccountNotFound  = errors.New("account not found")
)

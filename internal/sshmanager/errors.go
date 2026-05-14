package sshmanager

import "errors"

var (
	ErrMaxSessionsReached = errors.New("sshmanager: max sessions limit reached")
	ErrSessionNotFound    = errors.New("sshmanager: session not found")
	ErrKeyNotFound        = errors.New("sshmanager: key not found")
	ErrInvalidConfig      = errors.New("sshmanager: invalid config")
)

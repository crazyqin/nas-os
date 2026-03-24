package tiering

import "errors"

var (
	// ErrInvalidPolicy indicates an invalid policy configuration
	ErrInvalidPolicy = errors.New("invalid tiering policy")
	// ErrPolicyNotFound indicates the requested policy was not found
	ErrPolicyNotFound = errors.New("policy not found")
	// ErrAlreadyRunning indicates the manager is already running
	ErrAlreadyRunning = errors.New("tiering manager already running")
	// ErrNotRunning indicates the manager is not running
	ErrNotRunning = errors.New("tiering manager not running")
	// ErrMigrationFailed indicates a migration task failed
	ErrMigrationFailed = errors.New("migration failed")
	// ErrFileNotFound indicates the file was not found
	ErrFileNotFound = errors.New("file not found")
	// ErrSameTier indicates source and target tiers are the same
	ErrSameTier = errors.New("source and target tiers are the same")
)
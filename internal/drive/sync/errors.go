package sync

import "errors"

// Common errors for the sync package.
var (
	ErrTaskNotFound     = errors.New("sync task not found")
	ErrTaskExists       = errors.New("sync task already exists")
	ErrTaskDisabled     = errors.New("sync task is disabled")
	ErrNoFetcher        = errors.New("no remote fetcher configured")
	ErrNoPusher         = errors.New("no remote pusher configured")
	ErrConflictNotFound = errors.New("conflict record not found")
	ErrSyncInProgress   = errors.New("sync already in progress")
	ErrPathRequired     = errors.New("local and remote paths are required")
	ErrIDRequired       = errors.New("sync task ID is required")
	ErrHashMismatch     = errors.New("file hash mismatch after transfer")
	ErrBandwidthExceeded = errors.New("bandwidth limit exceeded")
)

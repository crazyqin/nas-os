package cron

import "errors"

var (
	ErrJobNotFound     = errors.New("cron: job not found")
	ErrMaxConcurrent   = errors.New("cron: max concurrent jobs reached")
	ErrInvalidSchedule = errors.New("cron: invalid schedule")
	ErrCronDisabled    = errors.New("cron: scheduler is disabled")
)

// Package posterwallpro implements the intelligent Poster Wall Pro module.
// Error definitions.
package posterwallpro

import "errors"

var ErrNoLocalPoster = errors.New("poster has no local image; scrape first")
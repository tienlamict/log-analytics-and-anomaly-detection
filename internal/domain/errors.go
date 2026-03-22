package domain

import "errors"

// ErrNotFound is returned by store implementations when a requested document does not exist.
// API handlers use this sentinel to discriminate 404 responses from other errors.
var ErrNotFound = errors.New("not found")

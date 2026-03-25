package storage

import "errors"

var (
	ErrNotFound       = errors.New("object not found")
	ErrInvalidKey     = errors.New("object key is invalid")
	ErrInvalidPayload = errors.New("object payload is invalid json")
	ErrAlreadyExpired = errors.New("object already expired")
)

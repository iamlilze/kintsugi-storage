package storage

import "errors"

// Ниже объявляем стандартные доменные ошибки.
// Их удобно сравнивать через errors.Is в HTTP-слое и тестах.
var (
	ErrNotFound       = errors.New("object not found")
	ErrInvalidKey     = errors.New("object key is invalid")
	ErrInvalidPayload = errors.New("object payload is invalid json")
	ErrAlreadyExpired = errors.New("object already expired")
)

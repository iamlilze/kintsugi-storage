package storage

import (
	"encoding/json"
	"time"
)

// Item - это runtime-представление объекта внутри памяти.
type Item struct {
	Payload   json.RawMessage
	ExpiresAt *time.Time
}

// NewItem создает объект и сразу делает defensive copy входных данных.
func NewItem(payload json.RawMessage, expiresAt *time.Time) Item {
	return Item{
		Payload:   cloneBytes(payload),     // Клонируем байты, чтобы внешний код не мог менять их после Put.
		ExpiresAt: cloneTimePtr(expiresAt), // Клонируем время, чтобы не хранить чужой указатель.
	}
}

// IsExpired проверяет, истек ли объект на заданный момент времени.
func (i Item) IsExpired(now time.Time) bool {
	if i.ExpiresAt == nil {
		return false
	}

	return !now.Before(*i.ExpiresAt)
}

// Clone возвращает безопасную копию Item.
func (i Item) Clone() Item {
	return NewItem(i.Payload, i.ExpiresAt) // Повторно используем конструктор, чтобы не дублировать логику копирования.
}

// cloneBytes делает глубокую копию слайса байтов.
func cloneBytes(src []byte) []byte {
	if src == nil {
		return nil
	}

	dst := make([]byte, len(src))
	copy(dst, src)
	return dst
}

// cloneTimePtr делает копию time.Time по указателю.
func cloneTimePtr(src *time.Time) *time.Time {
	if src == nil {
		return nil
	}

	value := *src
	return &value
}

package httpapi

import (
	"encoding/json"
	"net/http"
)

// errorResponse - минимальный единый формат ошибок API.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON пишет JSON-ответ с заданным статусом.
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if value == nil {
		return
	}

	_ = json.NewEncoder(w).Encode(value) // Кодируем ответ в JSON; ошибку здесь обычно уже некуда осмысленно вернуть.
}

// writeError пишет JSON-ошибку в едином формате.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{
		Error: message,
	})
}

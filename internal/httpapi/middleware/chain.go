package middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

// Chain применяет middleware в обратном порядке, чтобы первый в списке был внешним.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	if handler == nil {
		return http.NotFoundHandler()
	}

	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

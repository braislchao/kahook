package server

import (
	"github.com/google/uuid"
	"net/http"
)

const RequestIDHeader = "X-Request-ID"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		w.Header().Set(RequestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

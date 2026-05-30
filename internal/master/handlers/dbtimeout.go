package handlers

import (
	"context"
	"log"
	"net/http"
	"time"
)

// dbQueryTimeout is the maximum duration for database queries before
// the context is cancelled and a 503 response is returned.
const dbQueryTimeout = 5 * time.Second

// dbTimeoutContextKey is the context key for the database query timeout context.
type dbTimeoutContextKey struct{}

// DBTimeoutMiddleware wraps the request context with a 5-second timeout
// for database query operations. Handlers that detect context cancellation
// should return a 503 JSON response indicating query timeout.
//
// Requirement 11.6: When database query exceeds 5 seconds, Master_Server
// MUST cancel query context and return HTTP 503 with JSON body containing
// error field indicating query has timed out.
func DBTimeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbQueryTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WriteDBTimeoutResponse writes a 503 JSON response indicating a database
// query timeout. This should be called when a database operation fails due
// to context deadline exceeded.
func WriteDBTimeoutResponse(w http.ResponseWriter) {
	log.Printf("database query timeout: context deadline exceeded")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{"error":"database query timed out"}`))
}

// IsContextTimeout returns true if the error is due to context cancellation
// (deadline exceeded or cancelled).
func IsContextTimeout(err error) bool {
	if err == nil {
		return false
	}
	return err == context.DeadlineExceeded || err == context.Canceled
}

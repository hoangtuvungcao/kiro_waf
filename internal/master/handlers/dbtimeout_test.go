package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDBTimeoutMiddleware_SetsContextDeadline(t *testing.T) {
	var gotDeadline time.Time
	var hasDeadline bool

	handler := DBTimeoutMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotDeadline, hasDeadline = r.Context().Deadline()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	before := time.Now()
	handler.ServeHTTP(rec, req)

	if !hasDeadline {
		t.Fatal("expected context to have a deadline")
	}

	// Deadline should be approximately 5 seconds from now.
	expectedDeadline := before.Add(dbQueryTimeout)
	diff := gotDeadline.Sub(expectedDeadline)
	if diff < -100*time.Millisecond || diff > 100*time.Millisecond {
		t.Errorf("deadline diff = %v, expected within 100ms of 5s timeout", diff)
	}
}

func TestDBTimeoutMiddleware_CancelsContextAfterTimeout(t *testing.T) {
	handler := DBTimeoutMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a long-running operation that exceeds the timeout.
		ctx := r.Context()
		select {
		case <-ctx.Done():
			// Context was cancelled — this is expected.
			WriteDBTimeoutResponse(w)
		case <-time.After(10 * time.Second):
			t.Error("context was not cancelled within expected time")
			w.WriteHeader(http.StatusOK)
		}
	}))

	// Use a shorter timeout for the test by creating a request with a pre-set deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "database query timed out" {
		t.Errorf("error = %q, want %q", body["error"], "database query timed out")
	}
}

func TestWriteDBTimeoutResponse_Returns503JSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteDBTimeoutResponse(rec)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", contentType, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body["error"] != "database query timed out" {
		t.Errorf("error = %q, want %q", body["error"], "database query timed out")
	}
}

func TestIsContextTimeout(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"deadline exceeded", context.DeadlineExceeded, true},
		{"context canceled", context.Canceled, true},
		{"other error", http.ErrServerClosed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsContextTimeout(tt.err)
			if got != tt.want {
				t.Errorf("IsContextTimeout(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

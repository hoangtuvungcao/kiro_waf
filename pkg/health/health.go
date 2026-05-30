// Package health provides shared health check utilities used by both
// the master server and client node to report component health status.
//
// This package is intentionally placed in pkg/ (not internal/) because
// it provides a public API that both master and client binaries use,
// and could be consumed by external monitoring tools.
package health

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Status represents the health state of a component.
type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusDegraded Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// Check represents a single health check result.
type Check struct {
	Name    string `json:"name"`
	Status  Status `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Report is the aggregate health report for a service.
type Report struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
	Checks    []Check   `json:"checks"`
}

// Checker manages health checks for a service.
type Checker struct {
	mu        sync.RWMutex
	checks    []CheckFunc
	version   string
	startTime time.Time
}

// CheckFunc is a function that performs a health check and returns the result.
type CheckFunc func() Check

// NewChecker creates a new health checker with the given version string.
func NewChecker(version string) *Checker {
	return &Checker{
		version:   version,
		startTime: time.Now(),
	}
}

// Register adds a health check function to the checker.
func (c *Checker) Register(fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks = append(c.checks, fn)
}

// Run executes all registered health checks and returns an aggregate report.
func (c *Checker) Run() Report {
	c.mu.RLock()
	checks := make([]CheckFunc, len(c.checks))
	copy(checks, c.checks)
	c.mu.RUnlock()

	results := make([]Check, 0, len(checks))
	overall := StatusHealthy

	for _, fn := range checks {
		result := fn()
		results = append(results, result)

		switch result.Status {
		case StatusUnhealthy:
			overall = StatusUnhealthy
		case StatusDegraded:
			if overall != StatusUnhealthy {
				overall = StatusDegraded
			}
		}
	}

	return Report{
		Status:    overall,
		Timestamp: time.Now().UTC(),
		Version:   c.version,
		Uptime:    time.Since(c.startTime).Round(time.Second).String(),
		Checks:    results,
	}
}

// Handler returns an http.HandlerFunc that serves the health check report as JSON.
// Returns HTTP 200 for healthy/degraded, HTTP 503 for unhealthy.
func (c *Checker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report := c.Run()

		w.Header().Set("Content-Type", "application/json")
		if report.Status == StatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	}
}

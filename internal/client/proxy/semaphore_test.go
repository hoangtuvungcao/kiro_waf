package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewSemaphore(t *testing.T) {
	t.Run("default capacity when zero", func(t *testing.T) {
		sem := NewSemaphore(0)
		if sem.Cap() != DefaultMaxGoroutines {
			t.Errorf("expected cap %d, got %d", DefaultMaxGoroutines, sem.Cap())
		}
	})

	t.Run("default capacity when negative", func(t *testing.T) {
		sem := NewSemaphore(-5)
		if sem.Cap() != DefaultMaxGoroutines {
			t.Errorf("expected cap %d, got %d", DefaultMaxGoroutines, sem.Cap())
		}
	})

	t.Run("custom capacity", func(t *testing.T) {
		sem := NewSemaphore(100)
		if sem.Cap() != 100 {
			t.Errorf("expected cap 100, got %d", sem.Cap())
		}
	})
}

func TestSemaphore_AcquireRelease(t *testing.T) {
	sem := NewSemaphore(3)

	// Acquire all slots
	if !sem.Acquire() {
		t.Fatal("expected Acquire to succeed (slot 1)")
	}
	if !sem.Acquire() {
		t.Fatal("expected Acquire to succeed (slot 2)")
	}
	if !sem.Acquire() {
		t.Fatal("expected Acquire to succeed (slot 3)")
	}

	if sem.Len() != 3 {
		t.Errorf("expected Len 3, got %d", sem.Len())
	}

	// Should fail at capacity
	if sem.Acquire() {
		t.Fatal("expected Acquire to fail at capacity")
	}

	// Release one and try again
	sem.Release()
	if sem.Len() != 2 {
		t.Errorf("expected Len 2 after release, got %d", sem.Len())
	}

	if !sem.Acquire() {
		t.Fatal("expected Acquire to succeed after Release")
	}
}

func TestSemaphore_Concurrent(t *testing.T) {
	const maxConcurrency = 5
	sem := NewSemaphore(maxConcurrency)

	var concurrent int64
	var maxObserved int64
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !sem.Acquire() {
				return
			}
			defer sem.Release()

			cur := atomic.AddInt64(&concurrent, 1)
			// Track max observed concurrency
			for {
				old := atomic.LoadInt64(&maxObserved)
				if cur <= old || atomic.CompareAndSwapInt64(&maxObserved, old, cur) {
					break
				}
			}

			time.Sleep(time.Millisecond)
			atomic.AddInt64(&concurrent, -1)
		}()
	}

	wg.Wait()

	if maxObserved > int64(maxConcurrency) {
		t.Errorf("max observed concurrency %d exceeded limit %d", maxObserved, maxConcurrency)
	}
}

func TestSemaphoreMiddleware(t *testing.T) {
	t.Run("allows requests under capacity", func(t *testing.T) {
		sem := NewSemaphore(2)
		handler := SemaphoreMiddleware(sem, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("returns 503 at capacity", func(t *testing.T) {
		sem := NewSemaphore(1)

		// Block the single slot
		blocker := make(chan struct{})
		handler := SemaphoreMiddleware(sem, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-blocker
			w.WriteHeader(http.StatusOK)
		}))

		// First request occupies the slot
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/first", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()

		// Give the goroutine time to acquire the semaphore
		time.Sleep(10 * time.Millisecond)

		// Second request should get 503
		req := httptest.NewRequest("GET", "/second", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", rec.Code)
		}

		// Unblock the first request
		close(blocker)
		wg.Wait()
	})

	t.Run("releases slot after handler completes", func(t *testing.T) {
		sem := NewSemaphore(1)
		handler := SemaphoreMiddleware(sem, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// First request
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if sem.Len() != 0 {
			t.Errorf("expected semaphore to be released, Len=%d", sem.Len())
		}

		// Second request should also succeed
		req = httptest.NewRequest("GET", "/", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 on second request, got %d", rec.Code)
		}
	})
}

func TestRecoverMiddleware(t *testing.T) {
	t.Run("passes through normal requests", func(t *testing.T) {
		handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("hello"))
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "hello" {
			t.Errorf("expected body 'hello', got %q", rec.Body.String())
		}
	})

	t.Run("recovers from panic and returns 500", func(t *testing.T) {
		handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("something went wrong")
		}))

		req := httptest.NewRequest("GET", "/panic", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "internal server error") {
			t.Errorf("expected 'internal server error' in body, got %q", rec.Body.String())
		}
	})

	t.Run("continues serving after panic", func(t *testing.T) {
		callCount := 0
		handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				panic("first request panics")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("recovered"))
		}))

		// First request panics
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("first request: expected 500, got %d", rec.Code)
		}

		// Second request should succeed
		req = httptest.NewRequest("GET", "/", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("second request: expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "recovered" {
			t.Errorf("second request: expected 'recovered', got %q", rec.Body.String())
		}
	})

	t.Run("recovers from nil panic", func(t *testing.T) {
		handler := RecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic(nil)
		}))

		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()

		// Should not crash the test process
		handler.ServeHTTP(rec, req)
		// With panic(nil), recover() returns nil, so the middleware won't trigger
		// the error response. This is expected Go behavior.
	})
}

func TestServerConfig(t *testing.T) {
	t.Run("default config values", func(t *testing.T) {
		cfg := DefaultServerConfig()
		if cfg.ReadTimeout != 30*time.Second {
			t.Errorf("expected ReadTimeout 30s, got %v", cfg.ReadTimeout)
		}
		if cfg.WriteTimeout != 60*time.Second {
			t.Errorf("expected WriteTimeout 60s, got %v", cfg.WriteTimeout)
		}
		if cfg.IdleTimeout != 120*time.Second {
			t.Errorf("expected IdleTimeout 120s, got %v", cfg.IdleTimeout)
		}
	})

	t.Run("ApplyTo sets server timeouts", func(t *testing.T) {
		cfg := DefaultServerConfig()
		srv := &http.Server{}
		cfg.ApplyTo(srv)

		if srv.ReadTimeout != 30*time.Second {
			t.Errorf("expected server ReadTimeout 30s, got %v", srv.ReadTimeout)
		}
		if srv.WriteTimeout != 60*time.Second {
			t.Errorf("expected server WriteTimeout 60s, got %v", srv.WriteTimeout)
		}
		if srv.IdleTimeout != 120*time.Second {
			t.Errorf("expected server IdleTimeout 120s, got %v", srv.IdleTimeout)
		}
	})

	t.Run("ApplyTo skips zero values", func(t *testing.T) {
		cfg := ServerConfig{} // all zero
		srv := &http.Server{
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  15 * time.Second,
		}
		cfg.ApplyTo(srv)

		// Should not overwrite existing values with zero
		if srv.ReadTimeout != 5*time.Second {
			t.Errorf("expected ReadTimeout to remain 5s, got %v", srv.ReadTimeout)
		}
		if srv.WriteTimeout != 10*time.Second {
			t.Errorf("expected WriteTimeout to remain 10s, got %v", srv.WriteTimeout)
		}
		if srv.IdleTimeout != 15*time.Second {
			t.Errorf("expected IdleTimeout to remain 15s, got %v", srv.IdleTimeout)
		}
	})
}

func TestNewConfiguredServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	srv := NewConfiguredServer(":8080", handler)

	if srv.Addr != ":8080" {
		t.Errorf("expected addr :8080, got %s", srv.Addr)
	}
	if srv.ReadTimeout != DefaultReadTimeout {
		t.Errorf("expected ReadTimeout %v, got %v", DefaultReadTimeout, srv.ReadTimeout)
	}
	if srv.WriteTimeout != DefaultWriteTimeout {
		t.Errorf("expected WriteTimeout %v, got %v", DefaultWriteTimeout, srv.WriteTimeout)
	}
	if srv.Handler == nil {
		t.Error("expected handler to be set")
	}
}

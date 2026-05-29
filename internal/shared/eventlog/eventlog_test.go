package eventlog

import (
	"path/filepath"
	"testing"
	"time"

	"kiro_waf/internal/shared/storage"
)

func TestAppendJSONLWritesAndRotates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if _, err := AppendJSONL(path, map[string]string{"event": "first"}, Options{MaxBytes: 200, MaxBackups: 2}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	result, err := AppendJSONL(path, map[string]string{"event": "second", "payload": "012345678901234567890123456789"}, Options{MaxBytes: 40, MaxBackups: 2})
	if err != nil {
		t.Fatalf("append second: %v", err)
	}
	if !result.Written || !result.Rotated || result.RotatedPath == "" {
		t.Fatalf("unexpected append result: %#v", result)
	}
	assertPathExists(t, path)
	assertPathExists(t, path+".1")
}

func TestAppendJSONLRateLimitsByKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	statePath := filepath.Join(dir, "rate.json")
	now := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	opts := Options{
		Now:                now,
		RateLimitStatePath: statePath,
		RateLimitKey:       "runtime:process_exec",
		RateLimitWindow:    time.Minute,
		RateLimitMax:       1,
	}
	first, err := AppendJSONL(path, map[string]string{"event": "one"}, opts)
	if err != nil {
		t.Fatalf("append first: %v", err)
	}
	if !first.Written || first.Suppressed {
		t.Fatalf("first result = %#v, want written", first)
	}
	second, err := AppendJSONL(path, map[string]string{"event": "two"}, opts)
	if err != nil {
		t.Fatalf("append second: %v", err)
	}
	if !second.Suppressed || second.Written {
		t.Fatalf("second result = %#v, want suppressed", second)
	}
	count, err := storage.CountJSONLLines(path)
	if err != nil {
		t.Fatalf("count jsonl: %v", err)
	}
	if count != 1 {
		t.Fatalf("line count = %d, want 1", count)
	}
	opts.Now = now.Add(2 * time.Minute)
	third, err := AppendJSONL(path, map[string]string{"event": "three"}, opts)
	if err != nil {
		t.Fatalf("append third: %v", err)
	}
	if !third.Written || third.Suppressed {
		t.Fatalf("third result = %#v, want written after window reset", third)
	}
}

func TestAppendJSONLRejectsBlankPath(t *testing.T) {
	if _, err := AppendJSONL("", map[string]string{"event": "bad"}, Options{}); err == nil {
		t.Fatal("expected blank path error")
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := storage.CountJSONLLines(path); err != nil {
		t.Fatalf("expected jsonl path %s: %v", path, err)
	}
}

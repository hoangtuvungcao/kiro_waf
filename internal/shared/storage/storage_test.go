package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSONAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "state.json")
	if err := WriteJSONAtomic(path, map[string]string{"status": "ok"}); err != nil {
		t.Fatalf("write json atomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty file")
	}
	var decoded map[string]string
	if err := ReadJSON(path, &decoded); err != nil {
		t.Fatalf("read json: %v", err)
	}
	if decoded["status"] != "ok" {
		t.Fatalf("decoded status = %q, want ok", decoded["status"])
	}
}

func TestAppendJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events", "events.jsonl")
	if err := AppendJSONL(path, map[string]string{"event": "one"}); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if err := AppendJSONL(path, map[string]string{"event": "two"}); err != nil {
		t.Fatalf("append second: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected jsonl content")
	}
	count, err := CountJSONLLines(path)
	if err != nil {
		t.Fatalf("count jsonl: %v", err)
	}
	if count != 2 {
		t.Fatalf("jsonl lines = %d, want 2", count)
	}
}

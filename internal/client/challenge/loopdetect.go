package challenge

import (
	"sync"
	"time"
)

// loopRecord tracks challenge issuance timestamps for a given IP+challengeType key.
type loopRecord struct {
	timestamps []time.Time
}

// LoopDetector detects redirect loops where the same IP receives the same
// challenge type repeatedly in a short window. When a loop is detected,
// the challenge should be bypassed to avoid infinite redirects.
type LoopDetector struct {
	mu      sync.Mutex
	records map[string]*loopRecord
}

// NewLoopDetector creates a new LoopDetector instance.
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		records: make(map[string]*loopRecord),
	}
}

// ShouldBypass returns true if the same IP has received the same challenge type
// more than 3 times within the last 10 seconds, indicating a redirect loop.
func (ld *LoopDetector) ShouldBypass(ip string, challengeType string) bool {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	key := ip + ":" + challengeType
	rec, exists := ld.records[key]
	if !exists {
		return false
	}

	now := time.Now()
	cutoff := now.Add(-10 * time.Second)

	// Count timestamps within the 10-second window
	count := 0
	for _, ts := range rec.timestamps {
		if ts.After(cutoff) {
			count++
		}
	}

	return count > 3
}

// Record records a challenge issuance for the given IP and challenge type.
func (ld *LoopDetector) Record(ip string, challengeType string) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	key := ip + ":" + challengeType
	rec, exists := ld.records[key]
	if !exists {
		rec = &loopRecord{}
		ld.records[key] = rec
	}

	now := time.Now()
	cutoff := now.Add(-10 * time.Second)

	// Prune old timestamps while recording
	fresh := make([]time.Time, 0, len(rec.timestamps)+1)
	for _, ts := range rec.timestamps {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	fresh = append(fresh, now)
	rec.timestamps = fresh
}

// Cleanup removes entries older than 30 seconds to prevent unbounded memory growth.
func (ld *LoopDetector) Cleanup() {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-30 * time.Second)

	for key, rec := range ld.records {
		// Remove entries where all timestamps are older than 30 seconds
		hasRecent := false
		for _, ts := range rec.timestamps {
			if ts.After(cutoff) {
				hasRecent = true
				break
			}
		}
		if !hasRecent {
			delete(ld.records, key)
		}
	}
}

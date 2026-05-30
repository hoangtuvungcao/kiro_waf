package monitor

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// HealthSnapshot represents the full state snapshot written to disk periodically.
// It captures rate-limit state, session state, ban list, and XDP statistics.
type HealthSnapshot struct {
	Timestamp      time.Time              `json:"timestamp"`
	RateLimitState map[string]RateBucket  `json:"rate_limit_state"`
	SessionState   map[string]SessionData `json:"session_state"`
	BanList        []BanEntry             `json:"ban_list"`
	XDPStats       XDPStats               `json:"xdp_stats"`
}

// RateBucket represents a rate-limit bucket for a single IP.
type RateBucket struct {
	IP        string    `json:"ip"`
	Count     int       `json:"count"`
	WindowEnd time.Time `json:"window_end"`
}

// SessionData represents a session entry.
type SessionData struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// BanEntry represents a banned IP entry.
type BanEntry struct {
	IP        string    `json:"ip"`
	ExpiresAt time.Time `json:"expires_at"`
	Reason    string    `json:"reason"`
}

// SnapshotWriter handles periodic state snapshot writing to disk.
type SnapshotWriter struct {
	// config holds snapshot configuration (interval, max size, path).
	config MonitorConfig

	// stateProvider is a function that returns the current state to snapshot.
	stateProvider func() HealthSnapshot

	// done is closed when the snapshot loop exits.
	done chan struct{}

	// cancel cancels the snapshot loop context.
	cancel context.CancelFunc

	// lastSnapshot holds the timestamp of the last successful snapshot write.
	lastSnapshot time.Time

	// nowFunc allows overriding time.Now for testing.
	nowFunc func() time.Time

	// writeFileFunc allows overriding file writing for testing.
	writeFileFunc func(path string, data []byte) error
}

// SnapshotOption is a functional option for configuring the SnapshotWriter.
type SnapshotOption func(*SnapshotWriter)

// WithSnapshotNowFunc overrides the time function for testing.
func WithSnapshotNowFunc(fn func() time.Time) SnapshotOption {
	return func(sw *SnapshotWriter) {
		sw.nowFunc = fn
	}
}

// WithSnapshotWriteFileFunc overrides the file write function for testing.
func WithSnapshotWriteFileFunc(fn func(path string, data []byte) error) SnapshotOption {
	return func(sw *SnapshotWriter) {
		sw.writeFileFunc = fn
	}
}

// NewSnapshotWriter creates a new SnapshotWriter with the given configuration.
func NewSnapshotWriter(config MonitorConfig, stateProvider func() HealthSnapshot, opts ...SnapshotOption) *SnapshotWriter {
	cfg := applyDefaults(config)
	sw := &SnapshotWriter{
		config:        cfg,
		stateProvider: stateProvider,
		done:          make(chan struct{}),
		nowFunc:       time.Now,
	}

	for _, opt := range opts {
		opt(sw)
	}

	return sw
}

// Start begins the periodic snapshot writing loop.
func (sw *SnapshotWriter) Start(ctx context.Context) {
	childCtx, cancel := context.WithCancel(ctx)
	sw.cancel = cancel

	go sw.run(childCtx)
}

// Stop gracefully stops the snapshot writer.
func (sw *SnapshotWriter) Stop() {
	if sw.cancel != nil {
		sw.cancel()
	}
	<-sw.done
}

// LastSnapshotTime returns the timestamp of the last successful snapshot.
func (sw *SnapshotWriter) LastSnapshotTime() time.Time {
	return sw.lastSnapshot
}

// run is the main snapshot loop that writes state to disk every SnapshotInterval.
func (sw *SnapshotWriter) run(ctx context.Context) {
	defer close(sw.done)

	ticker := time.NewTicker(sw.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sw.writeSnapshot()
		}
	}
}

// writeSnapshot captures the current state and writes it to disk as gzip-compressed JSON.
// On I/O failure: logs a warning, keeps the old snapshot, and retries next cycle.
func (sw *SnapshotWriter) writeSnapshot() {
	snapshot := sw.stateProvider()
	snapshot.Timestamp = sw.nowFunc()

	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Snapshot: failed to marshal state: %v", err)
		return
	}

	// Compress with gzip
	compressed, err := compressGzip(data)
	if err != nil {
		log.Printf("Snapshot: failed to compress state: %v", err)
		return
	}

	// Check size limit and truncate if necessary
	if int64(len(compressed)) > sw.config.SnapshotMaxSize {
		compressed, err = sw.truncateSnapshot(snapshot)
		if err != nil {
			log.Printf("Snapshot: failed to truncate snapshot to fit size limit: %v", err)
			return
		}
	}

	// Write to disk
	snapshotPath := filepath.Join(sw.config.SnapshotPath, "state.snapshot.gz")
	if err := sw.writeFile(snapshotPath, compressed); err != nil {
		log.Printf("Snapshot: WARNING - failed to write snapshot to %s: %v (will retry next cycle)", snapshotPath, err)
		return
	}

	sw.lastSnapshot = sw.nowFunc()
}

// truncateSnapshot removes the oldest entries from the snapshot until it fits within
// the configured maximum size (64MB). It removes oldest rate-limit entries first,
// then oldest session entries, then oldest ban entries.
func (sw *SnapshotWriter) truncateSnapshot(snapshot HealthSnapshot) ([]byte, error) {
	// Strategy: progressively remove oldest entries until size fits
	// Priority: remove rate-limit entries first (most numerous), then sessions, then bans

	for {
		// Try to reduce rate-limit state (remove oldest entries by WindowEnd)
		if len(snapshot.RateLimitState) > 0 {
			snapshot.RateLimitState = truncateRateLimitState(snapshot.RateLimitState)
		} else if len(snapshot.SessionState) > 0 {
			// Then reduce session state (remove oldest by CreatedAt)
			snapshot.SessionState = truncateSessionState(snapshot.SessionState)
		} else if len(snapshot.BanList) > 0 {
			// Then reduce ban list (remove oldest by ExpiresAt)
			snapshot.BanList = truncateBanList(snapshot.BanList)
		} else {
			// Nothing left to truncate
			break
		}

		data, err := json.Marshal(snapshot)
		if err != nil {
			return nil, fmt.Errorf("marshal after truncation: %w", err)
		}

		compressed, err := compressGzip(data)
		if err != nil {
			return nil, fmt.Errorf("compress after truncation: %w", err)
		}

		if int64(len(compressed)) <= sw.config.SnapshotMaxSize {
			return compressed, nil
		}
	}

	// Final attempt with empty state
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("marshal minimal snapshot: %w", err)
	}

	compressed, err := compressGzip(data)
	if err != nil {
		return nil, fmt.Errorf("compress minimal snapshot: %w", err)
	}

	if int64(len(compressed)) > sw.config.SnapshotMaxSize {
		return nil, fmt.Errorf("snapshot exceeds %d bytes even after full truncation", sw.config.SnapshotMaxSize)
	}

	return compressed, nil
}

// truncateRateLimitState removes the oldest half of rate-limit entries (by WindowEnd).
func truncateRateLimitState(state map[string]RateBucket) map[string]RateBucket {
	if len(state) <= 1 {
		return make(map[string]RateBucket)
	}

	// Sort entries by WindowEnd (oldest first)
	type entry struct {
		key    string
		bucket RateBucket
	}
	entries := make([]entry, 0, len(state))
	for k, v := range state {
		entries = append(entries, entry{key: k, bucket: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].bucket.WindowEnd.Before(entries[j].bucket.WindowEnd)
	})

	// Keep the newest half
	keepFrom := len(entries) / 2
	result := make(map[string]RateBucket, len(entries)-keepFrom)
	for _, e := range entries[keepFrom:] {
		result[e.key] = e.bucket
	}
	return result
}

// truncateSessionState removes the oldest half of session entries (by CreatedAt).
func truncateSessionState(state map[string]SessionData) map[string]SessionData {
	if len(state) <= 1 {
		return make(map[string]SessionData)
	}

	type entry struct {
		key     string
		session SessionData
	}
	entries := make([]entry, 0, len(state))
	for k, v := range state {
		entries = append(entries, entry{key: k, session: v})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].session.CreatedAt.Before(entries[j].session.CreatedAt)
	})

	// Keep the newest half
	keepFrom := len(entries) / 2
	result := make(map[string]SessionData, len(entries)-keepFrom)
	for _, e := range entries[keepFrom:] {
		result[e.key] = e.session
	}
	return result
}

// truncateBanList removes the oldest half of ban entries (by ExpiresAt).
func truncateBanList(list []BanEntry) []BanEntry {
	if len(list) <= 1 {
		return nil
	}

	// Sort by ExpiresAt (oldest first)
	sorted := make([]BanEntry, len(list))
	copy(sorted, list)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ExpiresAt.Before(sorted[j].ExpiresAt)
	})

	// Keep the newest half
	keepFrom := len(sorted) / 2
	return sorted[keepFrom:]
}

// compressGzip compresses data using gzip.
func compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)

	if _, err := writer.Write(data); err != nil {
		return nil, fmt.Errorf("gzip write: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}

	return buf.Bytes(), nil
}

// DecompressGzip decompresses gzip data. Useful for reading snapshots back.
func DecompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(reader); err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}

	return buf.Bytes(), nil
}

// writeFile writes data to the specified path, creating directories as needed.
// Uses atomic write pattern: write to temp file, then rename.
func (sw *SnapshotWriter) writeFile(path string, data []byte) error {
	if sw.writeFileFunc != nil {
		return sw.writeFileFunc(path, data)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create snapshot directory %s: %w", dir, err)
	}

	// Write to temp file first (atomic write pattern)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp snapshot: %w", err)
	}

	// Rename temp file to final path (atomic on most filesystems)
	if err := os.Rename(tmpPath, path); err != nil {
		// Clean up temp file on rename failure
		os.Remove(tmpPath)
		return fmt.Errorf("rename snapshot: %w", err)
	}

	return nil
}

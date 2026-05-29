package eventlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kiro_waf/internal/shared/storage"
)

type Options struct {
	Now                time.Time
	MaxBytes           int64
	MaxBackups         int
	RateLimitStatePath string
	RateLimitKey       string
	RateLimitWindow    time.Duration
	RateLimitMax       int
}

type Result struct {
	Path        string `json:"path"`
	Written     bool   `json:"written"`
	Rotated     bool   `json:"rotated"`
	RotatedPath string `json:"rotated_path,omitempty"`
	Suppressed  bool   `json:"suppressed"`
	Reason      string `json:"reason,omitempty"`
}

type rateLimitState struct {
	Windows map[string]rateLimitWindow `json:"windows"`
}

type rateLimitWindow struct {
	StartedAt string `json:"started_at"`
	Count     int    `json:"count"`
}

func AppendJSONL(path string, value any, opts Options) (Result, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Result{}, errors.New("event log path is required")
	}
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	result := Result{Path: path}
	allowed, state, err := checkRateLimit(path, opts)
	if err != nil {
		return Result{}, err
	}
	if !allowed {
		result.Suppressed = true
		result.Reason = "rate_limited"
		return result, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return Result{}, err
	}
	payload = append(payload, '\n')
	rotated, rotatedPath, err := rotateIfNeeded(path, int64(len(payload)), opts)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Result{}, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Result{}, err
	}
	_, writeErr := f.Write(payload)
	closeErr := f.Close()
	if writeErr != nil {
		return Result{}, writeErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	if state != nil {
		if err := storage.WriteJSONAtomic(rateLimitStatePath(path, opts), state); err != nil {
			return Result{}, err
		}
	}
	result.Written = true
	result.Rotated = rotated
	result.RotatedPath = rotatedPath
	return result, nil
}

func checkRateLimit(path string, opts Options) (bool, *rateLimitState, error) {
	if strings.TrimSpace(opts.RateLimitKey) == "" || opts.RateLimitMax <= 0 || opts.RateLimitWindow <= 0 {
		return true, nil, nil
	}
	statePath := rateLimitStatePath(path, opts)
	state := rateLimitState{Windows: map[string]rateLimitWindow{}}
	if err := storage.ReadJSON(statePath, &state); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, nil, err
	}
	if state.Windows == nil {
		state.Windows = map[string]rateLimitWindow{}
	}
	key := strings.TrimSpace(opts.RateLimitKey)
	window := state.Windows[key]
	startedAt, err := time.Parse(time.RFC3339, window.StartedAt)
	if window.StartedAt == "" || err != nil || !opts.Now.Before(startedAt.Add(opts.RateLimitWindow)) {
		window = rateLimitWindow{StartedAt: opts.Now.Format(time.RFC3339), Count: 0}
	}
	if window.Count >= opts.RateLimitMax {
		state.Windows[key] = window
		return false, &state, nil
	}
	window.Count++
	state.Windows[key] = window
	return true, &state, nil
}

func rotateIfNeeded(path string, incomingBytes int64, opts Options) (bool, string, error) {
	if opts.MaxBytes <= 0 {
		return false, "", nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	if info.Size()+incomingBytes <= opts.MaxBytes {
		return false, "", nil
	}
	backups := opts.MaxBackups
	if backups <= 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, "", err
		}
		return true, "", nil
	}
	for i := backups - 1; i >= 1; i-- {
		src := backupPath(path, i)
		dst := backupPath(path, i+1)
		if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return false, "", err
		}
		_ = os.Remove(dst)
		if err := os.Rename(src, dst); err != nil {
			return false, "", err
		}
	}
	rotatedPath := backupPath(path, 1)
	_ = os.Remove(rotatedPath)
	if err := os.Rename(path, rotatedPath); err != nil {
		return false, "", err
	}
	return true, rotatedPath, nil
}

func rateLimitStatePath(path string, opts Options) string {
	if strings.TrimSpace(opts.RateLimitStatePath) != "" {
		return strings.TrimSpace(opts.RateLimitStatePath)
	}
	return path + ".ratelimit.json"
}

func backupPath(path string, index int) string {
	return fmt.Sprintf("%s.%d", path, index)
}

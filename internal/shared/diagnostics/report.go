package diagnostics

import (
	"runtime"
	"time"

	"kiro_waf/internal/shared/buildinfo"
	"kiro_waf/internal/shared/config"
)

// SystemReport contains system-level information for the `kiro-cli report` command.
type SystemReport struct {
	GeneratedAt string       `json:"generated_at"`
	Version     string       `json:"version"`
	GoVersion   string       `json:"go_version"`
	OS          string       `json:"os"`
	Arch        string       `json:"arch"`
	NumCPU      int          `json:"num_cpu"`
	NumGoroutine int         `json:"num_goroutine"`
	MemStats    MemInfo      `json:"memory"`
	Config      ConfigInfo   `json:"config"`
}

// MemInfo contains memory usage information.
type MemInfo struct {
	AllocMB      float64 `json:"alloc_mb"`
	TotalAllocMB float64 `json:"total_alloc_mb"`
	SysMB        float64 `json:"sys_mb"`
	NumGC        uint32  `json:"num_gc"`
}

// ConfigInfo contains configuration summary.
type ConfigInfo struct {
	Mode    string `json:"mode"`
	Plan    string `json:"plan"`
	Sites   int    `json:"sites"`
}

// BuildSystemReport generates a system report with CPU, memory, and configuration info.
func BuildSystemReport(rt config.RuntimeConfig, now time.Time) SystemReport {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemReport{
		GeneratedAt:  now.Format(time.RFC3339),
		Version:      buildinfo.Version,
		GoVersion:    runtime.Version(),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		NumCPU:       runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
		MemStats: MemInfo{
			AllocMB:      float64(m.Alloc) / 1024 / 1024,
			TotalAllocMB: float64(m.TotalAlloc) / 1024 / 1024,
			SysMB:        float64(m.Sys) / 1024 / 1024,
			NumGC:        m.NumGC,
		},
		Config: ConfigInfo{
			Mode:  rt.Mode,
			Plan:  rt.Plan,
			Sites: len(rt.Sites),
		},
	}
}

package monitor

import "time"

const (
	// DefaultCheckInterval is the interval between health checks (10 seconds).
	DefaultCheckInterval = 10 * time.Second

	// DefaultHealthEndpoint is the internal health check endpoint path.
	DefaultHealthEndpoint = "/__kiro/health"

	// DefaultHealthTimeout is the timeout for each health check request (5 seconds).
	DefaultHealthTimeout = 5 * time.Second

	// DefaultMaxConsecFailures is the number of consecutive health check failures
	// before triggering a service restart (3 failures = 30 seconds).
	DefaultMaxConsecFailures = 3

	// DefaultMaxRestartFailures is the number of consecutive restart failures
	// within the cooldown window before sending an alert to Master Server.
	DefaultMaxRestartFailures = 3

	// DefaultRestartCooldown is the wait time after max restart failures before retrying.
	DefaultRestartCooldown = 60 * time.Second

	// DefaultOfflineThreshold is the duration of lost Master Server connectivity
	// before switching to offline mode.
	DefaultOfflineThreshold = 60 * time.Second

	// DefaultReconnectInterval is the initial interval between reconnection attempts.
	DefaultReconnectInterval = 30 * time.Second

	// DefaultMaxReconnectBackoff is the maximum interval between reconnection attempts.
	DefaultMaxReconnectBackoff = 5 * time.Minute

	// DefaultSnapshotInterval is the interval between state snapshots written to disk.
	DefaultSnapshotInterval = 60 * time.Second

	// DefaultSnapshotMaxSize is the maximum size of a single snapshot file (64MB).
	DefaultSnapshotMaxSize = 64 * 1024 * 1024

	// DefaultSnapshotPath is the default directory for state snapshots.
	DefaultSnapshotPath = "/var/lib/kiro/snapshots/"

	// DefaultEmergencyDuration is how long emergency mode lasts after recovery.
	DefaultEmergencyDuration = 5 * time.Minute

	// DefaultEmergencyThreshold is the traffic multiplier (5x rate-limit threshold)
	// that triggers emergency recovery mode on crash.
	DefaultEmergencyThreshold = 5.0

	// DefaultDDoSThreshold is the traffic multiplier (10x rate-limit threshold)
	// that triggers DDoS detection and XDP strict mode.
	DefaultDDoSThreshold = 10.0

	// DefaultDDoSWindow is the sliding window duration for DDoS traffic analysis.
	DefaultDDoSWindow = 10 * time.Second
)

// MonitorConfig holds all configuration for the Health Monitor.
type MonitorConfig struct {
	// CheckInterval is the interval between health checks. Default 10 seconds.
	CheckInterval time.Duration

	// HealthEndpoint is the internal health check endpoint (e.g., "/__kiro/health").
	HealthEndpoint string

	// HealthTimeout is the timeout for each health check HTTP request. Default 5 seconds.
	HealthTimeout time.Duration

	// MaxConsecFailures is the number of consecutive health check failures
	// before triggering a service restart. Default 3.
	MaxConsecFailures int

	// MaxRestartFailures is the number of consecutive restart failures
	// within RestartCooldown before alerting Master Server. Default 3.
	MaxRestartFailures int

	// RestartCooldown is the wait time after max restart failures before retrying. Default 60 seconds.
	RestartCooldown time.Duration

	// OfflineThreshold is the duration of lost Master connectivity before offline mode. Default 60 seconds.
	OfflineThreshold time.Duration

	// ReconnectInterval is the initial reconnection attempt interval. Default 30 seconds.
	ReconnectInterval time.Duration

	// MaxReconnectBackoff is the maximum reconnection interval. Default 5 minutes.
	MaxReconnectBackoff time.Duration

	// SnapshotInterval is the interval between state snapshots. Default 60 seconds.
	SnapshotInterval time.Duration

	// SnapshotMaxSize is the maximum snapshot file size in bytes. Default 64MB.
	SnapshotMaxSize int64

	// SnapshotPath is the directory for state snapshot files.
	SnapshotPath string

	// EmergencyDuration is how long emergency mode lasts. Default 5 minutes.
	EmergencyDuration time.Duration

	// EmergencyThreshold is the traffic multiplier for emergency mode. Default 5x.
	EmergencyThreshold float64

	// DDoSThreshold is the traffic multiplier for DDoS detection. Default 10x.
	DDoSThreshold float64

	// DDoSWindow is the sliding window for DDoS traffic analysis. Default 10 seconds.
	DDoSWindow time.Duration

	// PlanConfig holds the Package_Plan limits for enforcement.
	PlanConfig PlanConfig
}

// PlanConfig defines the limits for the current Package_Plan.
type PlanConfig struct {
	// PlanName is the name of the current plan (community, pro, enterprise).
	PlanName string

	// MaxDomains is the maximum number of protected domains.
	MaxDomains int

	// XDPEnabled indicates whether XDP filtering is available.
	XDPEnabled bool

	// OTAEnabled indicates whether OTA updates are available.
	OTAEnabled bool

	// MaxRPM is the maximum requests per minute per IP.
	MaxRPM int
}

// DefaultConfig returns a MonitorConfig with all default values.
func DefaultConfig() MonitorConfig {
	return MonitorConfig{
		CheckInterval:       DefaultCheckInterval,
		HealthEndpoint:      DefaultHealthEndpoint,
		HealthTimeout:       DefaultHealthTimeout,
		MaxConsecFailures:   DefaultMaxConsecFailures,
		MaxRestartFailures:  DefaultMaxRestartFailures,
		RestartCooldown:     DefaultRestartCooldown,
		OfflineThreshold:    DefaultOfflineThreshold,
		ReconnectInterval:   DefaultReconnectInterval,
		MaxReconnectBackoff: DefaultMaxReconnectBackoff,
		SnapshotInterval:    DefaultSnapshotInterval,
		SnapshotMaxSize:     DefaultSnapshotMaxSize,
		SnapshotPath:        DefaultSnapshotPath,
		EmergencyDuration:   DefaultEmergencyDuration,
		EmergencyThreshold:  DefaultEmergencyThreshold,
		DDoSThreshold:       DefaultDDoSThreshold,
		DDoSWindow:          DefaultDDoSWindow,
	}
}

package monitor

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TrafficAnalyzer analyzes traffic patterns using a sliding window to detect
// emergency conditions and DDoS attacks.
// Requirements: 13.5, 13.11
type TrafficAnalyzer struct {
	mu sync.RWMutex

	// window is the sliding window duration for traffic analysis (default 10 seconds).
	window time.Duration

	// samples holds traffic samples within the current window.
	samples []TrafficSample

	// planThreshold is the rate-limit threshold from the current Package_Plan (requests per second).
	planThreshold uint64

	// emergencyThreshold is the multiplier for emergency mode (default 5x).
	emergencyThreshold float64

	// ddosThreshold is the multiplier for DDoS detection (default 10x).
	ddosThreshold float64

	// nowFunc returns the current time (overridable for testing).
	nowFunc func() time.Time
}

// TrafficSample represents a single traffic measurement at a point in time.
type TrafficSample struct {
	// Timestamp is when the sample was recorded.
	Timestamp time.Time `json:"timestamp"`

	// RequestsPerSecond is the measured request rate at this point.
	RequestsPerSecond uint64 `json:"requests_per_second"`
}

// TrafficAnalyzerConfig holds configuration for the TrafficAnalyzer.
type TrafficAnalyzerConfig struct {
	// Window is the sliding window duration (default 10 seconds).
	Window time.Duration

	// PlanThreshold is the rate-limit threshold from the Package_Plan (requests per second).
	PlanThreshold uint64

	// EmergencyThreshold is the multiplier for emergency mode (default 5x).
	EmergencyThreshold float64

	// DDoSThreshold is the multiplier for DDoS detection (default 10x).
	DDoSThreshold float64

	// NowFunc overrides the time function for testing.
	NowFunc func() time.Time
}

// NewTrafficAnalyzer creates a new TrafficAnalyzer with the given configuration.
func NewTrafficAnalyzer(cfg TrafficAnalyzerConfig) *TrafficAnalyzer {
	window := cfg.Window
	if window <= 0 {
		window = DefaultDDoSWindow
	}

	emergencyThreshold := cfg.EmergencyThreshold
	if emergencyThreshold <= 0 {
		emergencyThreshold = DefaultEmergencyThreshold
	}

	ddosThreshold := cfg.DDoSThreshold
	if ddosThreshold <= 0 {
		ddosThreshold = DefaultDDoSThreshold
	}

	nowFunc := cfg.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return &TrafficAnalyzer{
		window:             window,
		planThreshold:      cfg.PlanThreshold,
		emergencyThreshold: emergencyThreshold,
		ddosThreshold:      ddosThreshold,
		nowFunc:            nowFunc,
	}
}

// RecordSample adds a new traffic sample to the analyzer.
// Old samples outside the sliding window are automatically pruned.
func (ta *TrafficAnalyzer) RecordSample(rps uint64) {
	ta.mu.Lock()
	defer ta.mu.Unlock()

	now := ta.nowFunc()
	ta.samples = append(ta.samples, TrafficSample{
		Timestamp:         now,
		RequestsPerSecond: rps,
	})

	// Prune old samples outside the window
	ta.pruneOldSamples(now)
}

// CurrentRate returns the average request rate within the sliding window.
// Returns 0 if no samples are available.
func (ta *TrafficAnalyzer) CurrentRate() uint64 {
	ta.mu.RLock()
	defer ta.mu.RUnlock()

	now := ta.nowFunc()
	return ta.currentRateAt(now)
}

// currentRateAt calculates the average rate at a given time (must hold at least RLock).
func (ta *TrafficAnalyzer) currentRateAt(now time.Time) uint64 {
	if len(ta.samples) == 0 {
		return 0
	}

	windowStart := now.Add(-ta.window)
	var total uint64
	var count uint64

	for _, s := range ta.samples {
		if s.Timestamp.After(windowStart) || s.Timestamp.Equal(windowStart) {
			total += s.RequestsPerSecond
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / count
}

// IsEmergency returns true when the current traffic rate exceeds 5x the plan threshold.
// This indicates a crash during high traffic should trigger emergency recovery.
func (ta *TrafficAnalyzer) IsEmergency() bool {
	ta.mu.RLock()
	defer ta.mu.RUnlock()

	if ta.planThreshold == 0 {
		return false
	}

	now := ta.nowFunc()
	rate := ta.currentRateAt(now)
	threshold := uint64(float64(ta.planThreshold) * ta.emergencyThreshold)

	return rate > threshold
}

// IsDDoS returns true when the current traffic rate exceeds 10x the plan threshold.
// This triggers XDP strict mode and Master Server alert.
func (ta *TrafficAnalyzer) IsDDoS() bool {
	ta.mu.RLock()
	defer ta.mu.RUnlock()

	if ta.planThreshold == 0 {
		return false
	}

	now := ta.nowFunc()
	rate := ta.currentRateAt(now)
	threshold := uint64(float64(ta.planThreshold) * ta.ddosThreshold)

	return rate > threshold
}

// SetPlanThreshold updates the plan threshold (useful when plan changes).
func (ta *TrafficAnalyzer) SetPlanThreshold(threshold uint64) {
	ta.mu.Lock()
	defer ta.mu.Unlock()
	ta.planThreshold = threshold
}

// GetPlanThreshold returns the current plan threshold.
func (ta *TrafficAnalyzer) GetPlanThreshold() uint64 {
	ta.mu.RLock()
	defer ta.mu.RUnlock()
	return ta.planThreshold
}

// SampleCount returns the number of samples currently in the window.
func (ta *TrafficAnalyzer) SampleCount() int {
	ta.mu.RLock()
	defer ta.mu.RUnlock()
	return len(ta.samples)
}

// pruneOldSamples removes samples that are outside the sliding window.
// Must be called with mu held.
func (ta *TrafficAnalyzer) pruneOldSamples(now time.Time) {
	windowStart := now.Add(-ta.window)

	// Find the first sample within the window
	firstValid := 0
	for i, s := range ta.samples {
		if s.Timestamp.After(windowStart) || s.Timestamp.Equal(windowStart) {
			firstValid = i
			break
		}
		firstValid = i + 1
	}

	if firstValid > 0 {
		ta.samples = ta.samples[firstValid:]
	}
}

// EmergencyRecovery manages the emergency recovery mode.
// When traffic > 5x threshold + crash → restart with rate-limit reduced 50% for 5 minutes.
// Requirements: 13.5
type EmergencyRecovery struct {
	mu sync.RWMutex

	// active indicates whether emergency mode is currently active.
	active bool

	// activatedAt is when emergency mode was activated.
	activatedAt time.Time

	// duration is how long emergency mode lasts (default 5 minutes).
	duration time.Duration

	// originalRateLimit is the original rate-limit before emergency reduction.
	originalRateLimit uint64

	// emergencyRateLimit is the reduced rate-limit during emergency (50% of original).
	emergencyRateLimit uint64

	// onActivate is called when emergency mode is activated.
	onActivate func(reducedRateLimit uint64)

	// onDeactivate is called when emergency mode expires and normal limits are restored.
	onDeactivate func(originalRateLimit uint64)

	// nowFunc returns the current time (overridable for testing).
	nowFunc func() time.Time
}

// EmergencyRecoveryConfig holds configuration for EmergencyRecovery.
type EmergencyRecoveryConfig struct {
	// Duration is how long emergency mode lasts (default 5 minutes).
	Duration time.Duration

	// OriginalRateLimit is the normal rate-limit threshold.
	OriginalRateLimit uint64

	// OnActivate is called when emergency mode is activated with the reduced rate limit.
	OnActivate func(reducedRateLimit uint64)

	// OnDeactivate is called when emergency mode expires with the original rate limit.
	OnDeactivate func(originalRateLimit uint64)

	// NowFunc overrides the time function for testing.
	NowFunc func() time.Time
}

// NewEmergencyRecovery creates a new EmergencyRecovery manager.
func NewEmergencyRecovery(cfg EmergencyRecoveryConfig) *EmergencyRecovery {
	duration := cfg.Duration
	if duration <= 0 {
		duration = DefaultEmergencyDuration
	}

	nowFunc := cfg.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return &EmergencyRecovery{
		duration:          duration,
		originalRateLimit: cfg.OriginalRateLimit,
		emergencyRateLimit: cfg.OriginalRateLimit / 2, // 50% reduction
		onActivate:        cfg.OnActivate,
		onDeactivate:      cfg.OnDeactivate,
		nowFunc:           nowFunc,
	}
}

// Activate enters emergency recovery mode.
// The rate-limit is reduced by 50% for the configured duration.
func (er *EmergencyRecovery) Activate() {
	er.mu.Lock()
	defer er.mu.Unlock()

	if er.active {
		return // Already in emergency mode
	}

	er.active = true
	er.activatedAt = er.nowFunc()

	log.Printf("Health monitor: EMERGENCY mode activated - rate-limit reduced from %d to %d for %s",
		er.originalRateLimit, er.emergencyRateLimit, er.duration)

	if er.onActivate != nil {
		er.onActivate(er.emergencyRateLimit)
	}
}

// CheckExpiry checks if emergency mode has expired and deactivates it if so.
// Returns true if emergency mode was deactivated during this call.
func (er *EmergencyRecovery) CheckExpiry() bool {
	er.mu.Lock()
	defer er.mu.Unlock()

	if !er.active {
		return false
	}

	now := er.nowFunc()
	if now.Sub(er.activatedAt) < er.duration {
		return false // Still within emergency duration
	}

	// Emergency period expired, restore normal limits
	er.active = false

	log.Printf("Health monitor: emergency mode expired, restoring original rate-limit %d",
		er.originalRateLimit)

	if er.onDeactivate != nil {
		er.onDeactivate(er.originalRateLimit)
	}

	return true
}

// IsActive returns whether emergency mode is currently active.
func (er *EmergencyRecovery) IsActive() bool {
	er.mu.RLock()
	defer er.mu.RUnlock()
	return er.active
}

// GetCurrentRateLimit returns the current effective rate-limit
// (reduced during emergency, original otherwise).
func (er *EmergencyRecovery) GetCurrentRateLimit() uint64 {
	er.mu.RLock()
	defer er.mu.RUnlock()

	if er.active {
		return er.emergencyRateLimit
	}
	return er.originalRateLimit
}

// RemainingDuration returns how much time is left in emergency mode.
// Returns 0 if not in emergency mode.
func (er *EmergencyRecovery) RemainingDuration() time.Duration {
	er.mu.RLock()
	defer er.mu.RUnlock()

	if !er.active {
		return 0
	}

	elapsed := er.nowFunc().Sub(er.activatedAt)
	remaining := er.duration - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// DDoSDetector detects DDoS attacks and triggers XDP strict mode.
// When traffic > 10x threshold → activate XDP strict mode + alert Master within 5 seconds.
// Requirements: 13.11
type DDoSDetector struct {
	mu sync.RWMutex

	// active indicates whether DDoS detection mode is currently active.
	active bool

	// detectedAt is when the DDoS was first detected.
	detectedAt time.Time

	// alertDeadline is the deadline for alerting Master (5 seconds from detection).
	alertDeadline time.Duration

	// alertSent indicates whether the alert has been sent to Master.
	alertSent bool

	// xdpStrictFunc is called to activate XDP strict mode.
	xdpStrictFunc func() error

	// alertFunc is called to alert the Master Server.
	alertFunc func(alertType string, message string)

	// nowFunc returns the current time (overridable for testing).
	nowFunc func() time.Time
}

// DDoSDetectorConfig holds configuration for the DDoS detector.
type DDoSDetectorConfig struct {
	// AlertDeadline is the maximum time to alert Master after detection (default 5 seconds).
	AlertDeadline time.Duration

	// XDPStrictFunc is called to activate XDP strict mode.
	XDPStrictFunc func() error

	// AlertFunc is called to alert the Master Server.
	AlertFunc func(alertType string, message string)

	// NowFunc overrides the time function for testing.
	NowFunc func() time.Time
}

// NewDDoSDetector creates a new DDoS detector.
func NewDDoSDetector(cfg DDoSDetectorConfig) *DDoSDetector {
	alertDeadline := cfg.AlertDeadline
	if alertDeadline <= 0 {
		alertDeadline = 5 * time.Second
	}

	nowFunc := cfg.NowFunc
	if nowFunc == nil {
		nowFunc = time.Now
	}

	return &DDoSDetector{
		alertDeadline: alertDeadline,
		xdpStrictFunc: cfg.XDPStrictFunc,
		alertFunc:     cfg.AlertFunc,
		nowFunc:       nowFunc,
	}
}

// Detect is called when DDoS conditions are met (traffic > 10x threshold).
// It activates XDP strict mode and alerts the Master Server within 5 seconds.
func (dd *DDoSDetector) Detect(currentRate, threshold uint64) {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	if dd.active {
		return // Already handling DDoS
	}

	dd.active = true
	dd.detectedAt = dd.nowFunc()
	dd.alertSent = false

	log.Printf("Health monitor: DDoS DETECTED - traffic rate %d exceeds 10x threshold %d",
		currentRate, threshold)

	// Activate XDP strict mode immediately
	if dd.xdpStrictFunc != nil {
		if err := dd.xdpStrictFunc(); err != nil {
			log.Printf("Health monitor: failed to activate XDP strict mode: %v", err)
		} else {
			log.Printf("Health monitor: XDP strict mode activated")
		}
	}

	// Alert Master Server (must be within 5 seconds)
	if dd.alertFunc != nil {
		message := fmt.Sprintf("DDoS detected: traffic rate %d rps exceeds 10x threshold %d rps",
			currentRate, threshold)
		dd.alertFunc("ddos_detected", message)
		dd.alertSent = true
		log.Printf("Health monitor: DDoS alert sent to Master")
	}
}

// Clear deactivates DDoS detection mode when traffic returns to normal.
func (dd *DDoSDetector) Clear() {
	dd.mu.Lock()
	defer dd.mu.Unlock()

	if !dd.active {
		return
	}

	dd.active = false
	log.Printf("Health monitor: DDoS condition cleared, traffic returned to normal")
}

// IsActive returns whether DDoS detection is currently active.
func (dd *DDoSDetector) IsActive() bool {
	dd.mu.RLock()
	defer dd.mu.RUnlock()
	return dd.active
}

// PlanEnforcer enforces Package_Plan limits on configuration.
// It rejects configurations that exceed the limits of the current plan.
// Requirements: 13.10
type PlanEnforcer struct {
	mu sync.RWMutex

	// currentPlan holds the current plan configuration.
	currentPlan PlanConfig
}

// RequestedConfig represents a configuration request to be validated against plan limits.
type RequestedConfig struct {
	// Domains is the number of protected domains requested.
	Domains int `json:"domains"`

	// XDPEnabled indicates whether XDP filtering is requested.
	XDPEnabled bool `json:"xdp_enabled"`

	// OTAEnabled indicates whether OTA updates are requested.
	OTAEnabled bool `json:"ota_enabled"`

	// CustomRPM is the requested rate-limit in requests per minute per IP.
	CustomRPM int `json:"custom_rpm"`
}

// PlanViolation describes a specific plan limit violation.
type PlanViolation struct {
	// Field is the configuration field that violates the plan.
	Field string `json:"field"`

	// Requested is the requested value.
	Requested string `json:"requested"`

	// Limit is the plan limit.
	Limit string `json:"limit"`

	// Message is a human-readable description of the violation.
	Message string `json:"message"`
}

// PlanEnforcementError is returned when a configuration exceeds plan limits.
type PlanEnforcementError struct {
	// PlanName is the current plan name.
	PlanName string `json:"plan_name"`

	// Violations lists all plan limit violations.
	Violations []PlanViolation `json:"violations"`
}

// Error implements the error interface.
func (e *PlanEnforcementError) Error() string {
	if len(e.Violations) == 0 {
		return fmt.Sprintf("plan %s: no violations", e.PlanName)
	}
	return fmt.Sprintf("plan %s: configuration exceeds plan limits (%d violations)",
		e.PlanName, len(e.Violations))
}

// NewPlanEnforcer creates a new PlanEnforcer with the given plan configuration.
func NewPlanEnforcer(plan PlanConfig) *PlanEnforcer {
	return &PlanEnforcer{
		currentPlan: plan,
	}
}

// EnforceLimits validates a requested configuration against the current plan limits.
// Returns nil if the configuration is within limits, or a PlanEnforcementError with violations.
func (pe *PlanEnforcer) EnforceLimits(config RequestedConfig) error {
	pe.mu.RLock()
	plan := pe.currentPlan
	pe.mu.RUnlock()

	var violations []PlanViolation

	// Check domain limit
	if plan.MaxDomains > 0 && config.Domains > plan.MaxDomains {
		violations = append(violations, PlanViolation{
			Field:     "domains",
			Requested: fmt.Sprintf("%d", config.Domains),
			Limit:     fmt.Sprintf("%d", plan.MaxDomains),
			Message:   fmt.Sprintf("requested %d domains exceeds plan limit of %d", config.Domains, plan.MaxDomains),
		})
	}

	// Check XDP availability
	if config.XDPEnabled && !plan.XDPEnabled {
		violations = append(violations, PlanViolation{
			Field:     "xdp_enabled",
			Requested: "true",
			Limit:     "false",
			Message:   fmt.Sprintf("XDP filtering is not available on plan %s", plan.PlanName),
		})
	}

	// Check OTA availability
	if config.OTAEnabled && !plan.OTAEnabled {
		violations = append(violations, PlanViolation{
			Field:     "ota_enabled",
			Requested: "true",
			Limit:     "false",
			Message:   fmt.Sprintf("OTA updates are not available on plan %s", plan.PlanName),
		})
	}

	// Check RPM limit
	if plan.MaxRPM > 0 && config.CustomRPM > plan.MaxRPM {
		violations = append(violations, PlanViolation{
			Field:     "custom_rpm",
			Requested: fmt.Sprintf("%d", config.CustomRPM),
			Limit:     fmt.Sprintf("%d", plan.MaxRPM),
			Message:   fmt.Sprintf("requested %d RPM exceeds plan limit of %d RPM", config.CustomRPM, plan.MaxRPM),
		})
	}

	if len(violations) > 0 {
		return &PlanEnforcementError{
			PlanName:   plan.PlanName,
			Violations: violations,
		}
	}

	return nil
}

// UpdatePlan updates the current plan configuration.
func (pe *PlanEnforcer) UpdatePlan(plan PlanConfig) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.currentPlan = plan
	log.Printf("Health monitor: plan updated to %s (domains=%d, xdp=%v, ota=%v, rpm=%d)",
		plan.PlanName, plan.MaxDomains, plan.XDPEnabled, plan.OTAEnabled, plan.MaxRPM)
}

// GetCurrentPlan returns the current plan configuration.
func (pe *PlanEnforcer) GetCurrentPlan() PlanConfig {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	return pe.currentPlan
}

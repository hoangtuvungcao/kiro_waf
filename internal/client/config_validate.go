package client

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// FieldError represents a single validation failure for a config field.
type FieldError struct {
	Field  string // e.g. "license_key", "client.master_url"
	Reason string // e.g. "required but empty", "invalid URL format"
}

// ConfigValidationError collects all validation failures.
type ConfigValidationError struct {
	Errors []FieldError
}

// Error implements the error interface.
// Format: "config validation failed: license_key: required but empty; client.master_url: invalid URL format"
func (e *ConfigValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "config validation failed: (no details)"
	}
	parts := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		parts[i] = fe.Field + ": " + fe.Reason
	}
	return "config validation failed: " + strings.Join(parts, "; ")
}

// validateClientConfig checks all required fields and value formats.
// Returns a ConfigValidationError listing ALL validation failures (not just the first).
// Returns nil if the config is valid.
func validateClientConfig(cfg clientConfig) error {
	var errs []FieldError

	// Validate required fields (non-empty)
	if strings.TrimSpace(cfg.LicenseKey) == "" {
		errs = append(errs, FieldError{Field: "license_key", Reason: "required but empty"})
	}
	if strings.TrimSpace(cfg.CookieSecret) == "" {
		errs = append(errs, FieldError{Field: "cookie_secret", Reason: "required but empty"})
	}
	if strings.TrimSpace(cfg.MasterURL) == "" {
		errs = append(errs, FieldError{Field: "client.master_url", Reason: "required but empty"})
	}
	if strings.TrimSpace(cfg.BackendURL) == "" {
		errs = append(errs, FieldError{Field: "backend_url", Reason: "required but empty"})
	}

	// Validate URL format for master_url (only if non-empty)
	if strings.TrimSpace(cfg.MasterURL) != "" {
		if err := validateURL(cfg.MasterURL); err != "" {
			errs = append(errs, FieldError{Field: "client.master_url", Reason: err})
		}
	}

	// Validate URL format for backend_url (only if non-empty)
	if strings.TrimSpace(cfg.BackendURL) != "" {
		if err := validateURL(cfg.BackendURL); err != "" {
			errs = append(errs, FieldError{Field: "backend_url", Reason: err})
		}
	}

	// Validate CIDR format for each entry in admin_ips
	for i, cidr := range cfg.AdminIPs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			errs = append(errs, FieldError{
				Field:  fmt.Sprintf("admin.allow_ips[%d]", i),
				Reason: fmt.Sprintf("invalid CIDR notation %q", cidr),
			})
		}
	}

	// Validate positive integers for rate-limit fields (only if non-zero, since zero means "use default")
	if cfg.RPMPerIP < 0 {
		errs = append(errs, FieldError{Field: "client.rpm_per_ip", Reason: "must be a positive integer"})
	}
	if cfg.SubnetRPM < 0 {
		errs = append(errs, FieldError{Field: "client.subnet_rpm", Reason: "must be a positive integer"})
	}
	if cfg.HardBlockAfter < 0 {
		errs = append(errs, FieldError{Field: "client.hard_block_after", Reason: "must be a positive integer"})
	}
	if cfg.BlockTTLSeconds < 0 {
		errs = append(errs, FieldError{Field: "client.block_ttl_seconds", Reason: "must be a positive integer"})
	}
	if cfg.HeartbeatSeconds < 0 {
		errs = append(errs, FieldError{Field: "client.heartbeat_seconds", Reason: "must be a positive integer"})
	}
	if cfg.UpdateSeconds < 0 {
		errs = append(errs, FieldError{Field: "client.update_seconds", Reason: "must be a positive integer"})
	}

	if len(errs) > 0 {
		return &ConfigValidationError{Errors: errs}
	}
	return nil
}

// validateURL checks that a URL has http:// or https:// scheme and a non-empty host.
// Returns an empty string if valid, or a reason string if invalid.
func validateURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "invalid URL format (must be http:// or https://)"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "invalid URL format (must be http:// or https://)"
	}
	if parsed.Host == "" {
		return "invalid URL format (must be http:// or https:// with non-empty host)"
	}
	return ""
}

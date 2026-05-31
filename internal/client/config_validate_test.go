package client

import (
	"strings"
	"testing"
)

func TestValidateClientConfig_AllValid(t *testing.T) {
	cfg := clientConfig{
		LicenseKey:       "KIRO-TEST-1234",
		CookieSecret:     "secret-40-chars-long-enough-for-testing",
		MasterURL:        "https://firewall.example.com",
		BackendURL:       "http://127.0.0.1:3000",
		RPMPerIP:         120,
		SubnetRPM:        1800,
		HardBlockAfter:   360,
		BlockTTLSeconds:  900,
		HeartbeatSeconds: 60,
		UpdateSeconds:    300,
		AdminIPs:         []string{"203.0.113.10/32", "10.0.0.0/8"},
	}

	err := validateClientConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateClientConfig_AllRequiredMissing(t *testing.T) {
	cfg := clientConfig{}

	err := validateClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing required fields, got nil")
	}

	valErr, ok := err.(*ConfigValidationError)
	if !ok {
		t.Fatalf("expected *ConfigValidationError, got %T", err)
	}

	// Should report all 4 required fields
	if len(valErr.Errors) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(valErr.Errors), valErr.Errors)
	}

	// Check that all required fields are listed
	fields := make(map[string]bool)
	for _, fe := range valErr.Errors {
		fields[fe.Field] = true
	}
	for _, required := range []string{"license_key", "cookie_secret", "client.master_url", "backend_url"} {
		if !fields[required] {
			t.Errorf("expected field %q in errors, not found", required)
		}
	}
}

func TestValidateClientConfig_InvalidURLs(t *testing.T) {
	cfg := clientConfig{
		LicenseKey:   "KIRO-TEST-1234",
		CookieSecret: "secret-40-chars-long-enough-for-testing",
		MasterURL:    "ftp://invalid-scheme.com",
		BackendURL:   "not-a-url",
	}

	err := validateClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid URLs, got nil")
	}

	valErr := err.(*ConfigValidationError)
	foundMaster := false
	foundBackend := false
	for _, fe := range valErr.Errors {
		if fe.Field == "client.master_url" && strings.Contains(fe.Reason, "invalid URL format") {
			foundMaster = true
		}
		if fe.Field == "backend_url" && strings.Contains(fe.Reason, "invalid URL format") {
			foundBackend = true
		}
	}
	if !foundMaster {
		t.Error("expected validation error for client.master_url with invalid URL format")
	}
	if !foundBackend {
		t.Error("expected validation error for backend_url with invalid URL format")
	}
}

func TestValidateClientConfig_InvalidCIDR(t *testing.T) {
	cfg := clientConfig{
		LicenseKey:   "KIRO-TEST-1234",
		CookieSecret: "secret-40-chars-long-enough-for-testing",
		MasterURL:    "https://firewall.example.com",
		BackendURL:   "http://127.0.0.1:3000",
		AdminIPs:     []string{"203.0.113.10/32", "192.168.1.999/32", "invalid"},
	}

	err := validateClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}

	valErr := err.(*ConfigValidationError)
	cidrErrors := 0
	for _, fe := range valErr.Errors {
		if strings.HasPrefix(fe.Field, "admin.allow_ips[") {
			cidrErrors++
			if !strings.Contains(fe.Reason, "invalid CIDR notation") {
				t.Errorf("expected 'invalid CIDR notation' in reason, got: %s", fe.Reason)
			}
		}
	}
	if cidrErrors != 2 {
		t.Errorf("expected 2 CIDR errors, got %d", cidrErrors)
	}
}

func TestValidateClientConfig_NegativeIntegers(t *testing.T) {
	cfg := clientConfig{
		LicenseKey:       "KIRO-TEST-1234",
		CookieSecret:     "secret-40-chars-long-enough-for-testing",
		MasterURL:        "https://firewall.example.com",
		BackendURL:       "http://127.0.0.1:3000",
		RPMPerIP:         -1,
		SubnetRPM:        -5,
		HardBlockAfter:   -10,
		BlockTTLSeconds:  -100,
		HeartbeatSeconds: -60,
		UpdateSeconds:    -300,
	}

	err := validateClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for negative integers, got nil")
	}

	valErr := err.(*ConfigValidationError)
	negativeFields := 0
	for _, fe := range valErr.Errors {
		if strings.Contains(fe.Reason, "must be a positive integer") {
			negativeFields++
		}
	}
	if negativeFields != 6 {
		t.Errorf("expected 6 negative integer errors, got %d", negativeFields)
	}
}

func TestValidateClientConfig_ZeroIntegersAreValid(t *testing.T) {
	// Zero means "use default" — should not trigger validation errors
	cfg := clientConfig{
		LicenseKey:       "KIRO-TEST-1234",
		CookieSecret:     "secret-40-chars-long-enough-for-testing",
		MasterURL:        "https://firewall.example.com",
		BackendURL:       "http://127.0.0.1:3000",
		RPMPerIP:         0,
		SubnetRPM:        0,
		HardBlockAfter:   0,
		BlockTTLSeconds:  0,
		HeartbeatSeconds: 0,
		UpdateSeconds:    0,
	}

	err := validateClientConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error for zero values, got: %v", err)
	}
}

func TestValidateClientConfig_URLWithEmptyHost(t *testing.T) {
	cfg := clientConfig{
		LicenseKey:   "KIRO-TEST-1234",
		CookieSecret: "secret-40-chars-long-enough-for-testing",
		MasterURL:    "http://",
		BackendURL:   "https://",
	}

	err := validateClientConfig(cfg)
	if err == nil {
		t.Fatal("expected error for URLs with empty host, got nil")
	}

	valErr := err.(*ConfigValidationError)
	foundMaster := false
	foundBackend := false
	for _, fe := range valErr.Errors {
		if fe.Field == "client.master_url" && strings.Contains(fe.Reason, "non-empty host") {
			foundMaster = true
		}
		if fe.Field == "backend_url" && strings.Contains(fe.Reason, "non-empty host") {
			foundBackend = true
		}
	}
	if !foundMaster {
		t.Error("expected validation error for client.master_url with empty host")
	}
	if !foundBackend {
		t.Error("expected validation error for backend_url with empty host")
	}
}

func TestConfigValidationError_ErrorString(t *testing.T) {
	err := &ConfigValidationError{
		Errors: []FieldError{
			{Field: "license_key", Reason: "required but empty"},
			{Field: "client.master_url", Reason: "invalid URL format (must be http:// or https://)"},
		},
	}

	msg := err.Error()
	if !strings.HasPrefix(msg, "config validation failed: ") {
		t.Errorf("error message should start with 'config validation failed: ', got: %s", msg)
	}
	if !strings.Contains(msg, "license_key: required but empty") {
		t.Errorf("error message should contain license_key error, got: %s", msg)
	}
	if !strings.Contains(msg, "client.master_url: invalid URL format") {
		t.Errorf("error message should contain master_url error, got: %s", msg)
	}
}

func TestValidateClientConfig_EmptyAdminIPs(t *testing.T) {
	// Empty AdminIPs slice should be valid
	cfg := clientConfig{
		LicenseKey:   "KIRO-TEST-1234",
		CookieSecret: "secret-40-chars-long-enough-for-testing",
		MasterURL:    "https://firewall.example.com",
		BackendURL:   "http://127.0.0.1:3000",
		AdminIPs:     []string{},
	}

	err := validateClientConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error for empty AdminIPs, got: %v", err)
	}
}

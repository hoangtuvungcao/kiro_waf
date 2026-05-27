package proxy

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kiro_waf/internal/shared/config"
)

func TestGenerateNginxSupportsDomainAndRouteMappings(t *testing.T) {
	runtime := proxyRuntime(t)
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate nginx: %v", err)
	}
	cfg := plan.NginxConfig
	assertContains(t, cfg, "upstream api_pool")
	assertContains(t, cfg, "server 127.0.0.1:4000")
	assertContains(t, cfg, "upstream frontend_pool")
	assertContains(t, cfg, "server 127.0.0.1:3000")
	assertContains(t, cfg, "server 127.0.0.1:3001")
	assertContains(t, cfg, "server_name example.com www.example.com;")
	assertContains(t, cfg, "server_name brand-a.example.net brand-b.example.net;")
	assertContains(t, cfg, "location /api/search")
	assertContains(t, cfg, "proxy_pass http://api_pool;")
	assertContains(t, cfg, "# kiro_route_rpm_per_ip 30")
	assertContains(t, cfg, "proxy_cache_valid 200 10s;")
	assertContains(t, cfg, "include cloudflare-real-ip.conf;")
	assertContains(t, plan.CloudflareRealIP, "set_real_ip_from 198.51.100.0/24;")
	assertContains(t, plan.CloudflareRealIP, "set_real_ip_from 2001:db8::/32;")
	if len(plan.Warnings) == 0 {
		t.Fatal("expected flexible_http sensitive route warning")
	}
}

func TestGenerateNginxFlexibleHTTPDoesNotRequireCert(t *testing.T) {
	runtime := proxyRuntime(t)
	runtime.Sites[0].CertFile = ""
	runtime.Sites[0].KeyFile = ""
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate flexible http: %v", err)
	}
	assertContains(t, plan.NginxConfig, "listen 80;")
	if strings.Contains(plan.NginxConfig, "ssl_certificate") {
		t.Fatal("flexible_http must not emit ssl_certificate")
	}
}

func TestGenerateNginxFullStrictRequiresCertKey(t *testing.T) {
	runtime := proxyRuntime(t)
	runtime.Sites[0].TLSMode = "full_strict"
	runtime.Sites[0].CertFile = ""
	runtime.Sites[0].KeyFile = ""
	if _, err := GenerateNginx(runtime); err == nil {
		t.Fatal("expected full_strict cert/key rejection")
	}
}

func TestGenerateNginxFullStrictEmitsSSL(t *testing.T) {
	runtime := proxyRuntime(t)
	runtime.Sites[0].TLSMode = "full_strict"
	runtime.Sites[0].CertFile = "/etc/kiro/certs/example.com.pem"
	runtime.Sites[0].KeyFile = "/etc/kiro/certs/example.com.key"
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate full strict: %v", err)
	}
	assertContains(t, plan.NginxConfig, "listen 443 ssl;")
	assertContains(t, plan.NginxConfig, "ssl_certificate /etc/kiro/certs/example.com.pem;")
	assertContains(t, plan.NginxConfig, "ssl_certificate_key /etc/kiro/certs/example.com.key;")
}

func TestGenerateNginxServerModeDisablesProxy(t *testing.T) {
	runtime := proxyRuntime(t)
	runtime.Mode = "server"
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate server mode: %v", err)
	}
	assertContains(t, plan.NginxConfig, "proxy disabled in server mode")
	if strings.Contains(plan.NginxConfig, "server_name") {
		t.Fatal("server mode must not generate website server blocks")
	}
}

func TestValidatePlanUsesRunner(t *testing.T) {
	runtime := proxyRuntime(t)
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate nginx: %v", err)
	}
	runner := &fakeValidateRunner{}
	if err := ValidatePlan(t.TempDir(), plan, runner); err != nil {
		t.Fatalf("validate plan: %v", err)
	}
	if runner.ValidatedPath == "" {
		t.Fatal("expected runner to receive config path")
	}
	if _, err := os.Stat(runner.ValidatedPath); err != nil {
		t.Fatalf("expected config file: %v", err)
	}
}

func TestValidatePlanReturnsRunnerError(t *testing.T) {
	runtime := proxyRuntime(t)
	plan, err := GenerateNginx(runtime)
	if err != nil {
		t.Fatalf("generate nginx: %v", err)
	}
	runnerErr := errors.New("nginx bad")
	if err := ValidatePlan(t.TempDir(), plan, &fakeValidateRunner{Err: runnerErr}); !errors.Is(err, runnerErr) {
		t.Fatalf("expected runner error, got %v", err)
	}
}

func proxyRuntime(t *testing.T) config.RuntimeConfig {
	t.Helper()
	dir := t.TempDir()
	v4 := filepath.Join(dir, "cf-v4.txt")
	v6 := filepath.Join(dir, "cf-v6.txt")
	if err := os.WriteFile(v4, []byte("198.51.100.0/24\n"), 0o644); err != nil {
		t.Fatalf("write cf v4: %v", err)
	}
	if err := os.WriteFile(v6, []byte("2001:db8::/32\n"), 0o644); err != nil {
		t.Fatalf("write cf v6: %v", err)
	}
	return config.RuntimeConfig{
		Mode: "full",
		CFOriginLock: config.RuntimeCloudflareOriginLock{
			Enabled:  true,
			IPv4File: v4,
			IPv6File: v6,
		},
		BackendPools: []config.RuntimeBackendPool{
			{
				ID: "frontend_pool",
				Upstreams: []config.RuntimeUpstream{
					{ID: "frontend_2", URL: "http://127.0.0.1:3001"},
					{ID: "frontend_1", URL: "http://127.0.0.1:3000"},
				},
			},
			{
				ID:        "api_pool",
				Upstreams: []config.RuntimeUpstream{{ID: "api_1", URL: "http://127.0.0.1:4000"}},
			},
		},
		Sites: []config.RuntimeSite{
			{
				ID:                 "main_site",
				Domains:            []string{"example.com", "www.example.com"},
				TLSMode:            "flexible_http",
				DefaultBackendPool: "frontend_pool",
				Routes: []config.RuntimeRoute{
					{Path: "/", BackendPool: "frontend_pool"},
					{Path: "/login", BackendPool: "api_pool", RPMPerIP: 10},
					{Path: "/api/search", BackendPool: "api_pool", RPMPerIP: 30, CacheSeconds: 10},
					{Path: "/upload", BackendPool: "api_pool", MaxBodyMB: 10},
				},
			},
			{
				ID:                 "shared_backend_site",
				Domains:            []string{"brand-a.example.net", "brand-b.example.net"},
				TLSMode:            "flexible_http",
				DefaultBackendPool: "frontend_pool",
			},
		},
	}
}

type fakeValidateRunner struct {
	ValidatedPath string
	Err           error
}

func (r *fakeValidateRunner) Validate(path string) error {
	r.ValidatedPath = path
	return r.Err
}

func assertContains(t *testing.T, value string, want string) {
	t.Helper()
	if !strings.Contains(value, want) {
		t.Fatalf("expected %q in:\n%s", want, value)
	}
}

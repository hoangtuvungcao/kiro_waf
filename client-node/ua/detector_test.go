package ua

import "testing"

func TestIsAutomationUA_EmptyString(t *testing.T) {
	if !IsAutomationUA("") {
		t.Error("expected empty UA to be detected as automation")
	}
}

func TestIsAutomationUA_KnownAttackTools(t *testing.T) {
	cases := []struct {
		name string
		ua   string
	}{
		{"sqlmap", "sqlmap/1.5#stable"},
		{"sqlmap mixed case", "SQLMap/1.5"},
		{"python-requests", "python-requests/2.28.0"},
		{"python-requests mixed case", "Python-Requests/2.31.0"},
		{"python-urllib", "Python-urllib/3.9"},
		{"libwww-perl", "libwww-perl/6.67"},
		{"libwww-perl mixed case", "Libwww-Perl/6.67"},
		{"httpclient", "Apache-HttpClient/4.5.13"},
		{"go-http-client", "Go-http-client/1.1"},
		{"nikto", "Nikto/2.1.6"},
		{"nikto mixed case", "NIKTO/2.1.6"},
		{"masscan", "masscan/1.3"},
		{"masscan in UA", "Mozilla/5.0 masscan"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsAutomationUA(tc.ua) {
				t.Errorf("expected %q to be detected as automation", tc.ua)
			}
		})
	}
}

func TestIsAutomationUA_PrefixPatterns(t *testing.T) {
	cases := []struct {
		name string
		ua   string
	}{
		{"curl lowercase", "curl/7.88.1"},
		{"curl uppercase", "Curl/7.88.1"},
		{"wget lowercase", "wget/1.21"},
		{"wget uppercase", "Wget/1.21.3"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsAutomationUA(tc.ua) {
				t.Errorf("expected %q to be detected as automation", tc.ua)
			}
		})
	}
}

func TestIsAutomationUA_ValidBrowsers(t *testing.T) {
	cases := []struct {
		name string
		ua   string
	}{
		{
			"Chrome on Windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		{
			"Firefox on Linux",
			"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
		},
		{
			"Safari on macOS",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
		},
		{
			"Edge on Windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		},
		{
			"Chrome on Android",
			"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.144 Mobile Safari/537.36",
		},
		{
			"Safari on iOS",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsAutomationUA(tc.ua) {
				t.Errorf("expected %q to NOT be detected as automation", tc.ua)
			}
		})
	}
}

func TestIsAutomationUA_CustomNonAttackUAs(t *testing.T) {
	cases := []struct {
		name string
		ua   string
	}{
		{"custom app", "MyApp/1.0"},
		{"monitoring tool", "UptimeRobot/2.0"},
		{"search bot", "Googlebot/2.1"},
		{"custom with curl in name", "securlify/1.0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IsAutomationUA(tc.ua) {
				t.Errorf("expected %q to NOT be detected as automation", tc.ua)
			}
		})
	}
}

func TestIsAutomationUA_CaseInsensitive(t *testing.T) {
	// Verify case-insensitive matching works for all patterns
	cases := []struct {
		name string
		ua   string
	}{
		{"SQLMAP all caps", "SQLMAP/1.0"},
		{"Python-Requests title case", "Python-Requests/2.28"},
		{"LIBWWW-PERL all caps", "LIBWWW-PERL/6.0"},
		{"CURL prefix caps", "CURL/7.0"},
		{"HttpClient mixed", "HttpClient/4.5"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsAutomationUA(tc.ua) {
				t.Errorf("expected %q to be detected as automation (case-insensitive)", tc.ua)
			}
		})
	}
}

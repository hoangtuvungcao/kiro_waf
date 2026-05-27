package firewall

import "testing"

func TestTempBanStatementIPv4(t *testing.T) {
	statement, err := TempBanStatement("192.0.2.10", 300)
	if err != nil {
		t.Fatalf("temp ban statement: %v", err)
	}
	assertContains(t, statement, "kiro_temp_ban_v4")
	assertContains(t, statement, "192.0.2.10 timeout 300s")
}

func TestTempBanStatementIPv6(t *testing.T) {
	statement, err := TempBanStatement("2001:db8::10", 300)
	if err != nil {
		t.Fatalf("temp ban statement: %v", err)
	}
	assertContains(t, statement, "kiro_temp_ban_v6")
	assertContains(t, statement, "2001:db8::10 timeout 300s")
}

func TestAddTempBanAppliesStatement(t *testing.T) {
	runner := &fakeRunner{}
	if err := AddTempBan(runner, "192.0.2.10", 120); err != nil {
		t.Fatalf("add temp ban: %v", err)
	}
	if runner.applies != 1 {
		t.Fatalf("applies = %d, want 1", runner.applies)
	}
	assertContains(t, runner.current, "kiro_temp_ban_v4")
}

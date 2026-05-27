package firewall

import (
	"errors"
	"fmt"
	"net"
)

func TempBanStatement(ipText string, timeoutSeconds int) (string, error) {
	ip := net.ParseIP(ipText)
	if ip == nil {
		return "", fmt.Errorf("invalid IP %q", ipText)
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 900
	}
	setName := "kiro_temp_ban_v6"
	normalized := ip.String()
	if ip.To4() != nil {
		setName = "kiro_temp_ban_v4"
		normalized = ip.To4().String()
	}
	return fmt.Sprintf("add element inet kiro_waf %s { %s timeout %ds }\n", setName, normalized, timeoutSeconds), nil
}

func AddTempBan(runner Runner, ipText string, timeoutSeconds int) error {
	if runner == nil {
		return errors.New("firewall runner is required")
	}
	statement, err := TempBanStatement(ipText, timeoutSeconds)
	if err != nil {
		return err
	}
	return runner.Apply(statement)
}

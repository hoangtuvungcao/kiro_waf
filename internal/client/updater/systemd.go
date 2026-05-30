package updater

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// checkSystemdActive checks if a systemd service is in active state.
// Returns true if the service is active (running), false otherwise.
func checkSystemdActive(ctx context.Context, serviceName string) (bool, error) {
	if serviceName == "" {
		return false, fmt.Errorf("updater: empty service name")
	}

	cmd := exec.CommandContext(ctx, "systemctl", "is-active", serviceName)
	output, err := cmd.Output()
	if err != nil {
		// systemctl is-active returns non-zero exit code when service is not active
		// This is expected behavior, not an error
		return false, nil
	}

	status := strings.TrimSpace(string(output))
	return status == "active", nil
}

// restartSystemdService restarts a systemd service.
func restartSystemdService(ctx context.Context, serviceName string) error {
	if serviceName == "" {
		return fmt.Errorf("updater: empty service name")
	}

	cmd := exec.CommandContext(ctx, "systemctl", "restart", serviceName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("updater: systemctl restart %s failed: %s: %w",
			serviceName, strings.TrimSpace(string(output)), err)
	}

	return nil
}

package machinefingerprint

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"kiro_waf/internal/shared/licenseverify"
)

func TestSnapshotFingerprintAndBinding(t *testing.T) {
	snapshot := Snapshot{
		MachineID:     "machine-id",
		Hostname:      "kiro-test",
		KernelRelease: "6.8.0-test",
		PrimaryMAC:    "00:11:22:33:44:55",
		PhysicalMACs:  []string{"00:11:22:33:44:55", "66:77:88:99:aa:bb"},
	}

	hashA := snapshot.FingerprintHash("salt-2026")
	hashB := snapshot.FingerprintHash("salt-2026")
	if hashA == "" || hashA != hashB {
		t.Fatal("expected stable fingerprint hash")
	}

	binding := snapshot.Binding("salt-2026")
	if binding.FingerprintHash != hashA {
		t.Fatal("expected binding to use snapshot fingerprint hash")
	}
	if binding.MachineIDHash != licenseverify.HashValue("machine-id") {
		t.Fatal("expected machine id hash")
	}
	if binding.PrimaryMACHash != licenseverify.HashValue("00:11:22:33:44:55") {
		t.Fatal("expected primary mac hash")
	}
}

func TestCollectUsesMachineIDRouteAndKernelFiles(t *testing.T) {
	dir := t.TempDir()
	machineIDPath := filepath.Join(dir, "machine-id")
	routePath := filepath.Join(dir, "route")
	kernelPath := filepath.Join(dir, "osrelease")
	if err := os.WriteFile(machineIDPath, []byte("machine-id\n"), 0o644); err != nil {
		t.Fatalf("write machine id: %v", err)
	}
	if err := os.WriteFile(routePath, []byte("Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\neth9\t00000000\t00000000\t0003\t0\t0\t10\t00000000\t0\t0\t0\n"), 0o644); err != nil {
		t.Fatalf("write route: %v", err)
	}
	if err := os.WriteFile(kernelPath, []byte("6.8.0-test\n"), 0o644); err != nil {
		t.Fatalf("write kernel: %v", err)
	}

	snapshot, err := Collect(Options{
		MachineIDPath:     machineIDPath,
		RoutePath:         routePath,
		KernelReleasePath: kernelPath,
		Hostname:          "kiro-test",
	})
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if snapshot.MachineID != "machine-id" {
		t.Fatalf("machine id = %q", snapshot.MachineID)
	}
	if snapshot.Hostname != "kiro-test" {
		t.Fatalf("hostname = %q", snapshot.Hostname)
	}
	if snapshot.KernelRelease != "6.8.0-test" {
		t.Fatalf("kernel release = %q", snapshot.KernelRelease)
	}
}

func TestPhysicalMACsSortsAndSkipsLoopback(t *testing.T) {
	macs := physicalMACs([]net.Interface{
		{Name: "lo", Flags: net.FlagLoopback, HardwareAddr: net.HardwareAddr{0, 0, 0, 0, 0, 1}},
		{Name: "eth1", HardwareAddr: net.HardwareAddr{0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb}},
		{Name: "eth0", HardwareAddr: net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}},
	})

	if len(macs) != 2 || macs[0] != "00:11:22:33:44:55" || macs[1] != "66:77:88:99:aa:bb" {
		t.Fatalf("unexpected mac list: %#v", macs)
	}
}

func TestDefaultRouteInterfaceChoosesLowestMetric(t *testing.T) {
	path := filepath.Join(t.TempDir(), "route")
	content := "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t00000000\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth1\t00000000\t00000000\t0003\t0\t0\t10\t00000000\t0\t0\t0\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write route: %v", err)
	}

	iface, err := defaultRouteInterface(path)
	if err != nil {
		t.Fatalf("default route: %v", err)
	}
	if iface != "eth1" {
		t.Fatalf("interface = %q, want eth1", iface)
	}
}

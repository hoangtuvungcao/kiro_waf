package machinefingerprint

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"

	"kiro_waf/internal/shared/licenseverify"
)

const (
	defaultMachineIDPath     = "/etc/machine-id"
	fallbackMachineIDPath    = "/var/lib/dbus/machine-id"
	defaultRoutePath         = "/proc/net/route"
	defaultKernelReleasePath = "/proc/sys/kernel/osrelease"
)

type Options struct {
	MachineIDPath     string
	RoutePath         string
	KernelReleasePath string
	Hostname          string
}

type Snapshot struct {
	MachineID     string
	Hostname      string
	KernelRelease string
	PrimaryMAC    string
	PhysicalMACs  []string
}

func Collect(opts Options) (Snapshot, error) {
	machineID, err := readMachineID(opts.MachineIDPath)
	if err != nil {
		return Snapshot{}, err
	}
	hostname := strings.TrimSpace(opts.Hostname)
	if hostname == "" {
		hostname, err = os.Hostname()
		if err != nil {
			return Snapshot{}, fmt.Errorf("read hostname: %w", err)
		}
	}

	kernelRelease := ""
	kernelPath := opts.KernelReleasePath
	if kernelPath == "" {
		kernelPath = defaultKernelReleasePath
	}
	if raw, err := os.ReadFile(kernelPath); err == nil {
		kernelRelease = strings.TrimSpace(string(raw))
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("list network interfaces: %w", err)
	}
	allMACs := physicalMACs(interfaces)

	routePath := opts.RoutePath
	if routePath == "" {
		routePath = defaultRoutePath
	}
	defaultIface, _ := defaultRouteInterface(routePath)
	primaryMAC := primaryMACForInterface(interfaces, defaultIface)
	if primaryMAC == "" && len(allMACs) > 0 {
		primaryMAC = allMACs[0]
	}

	return Snapshot{
		MachineID:     machineID,
		Hostname:      hostname,
		KernelRelease: kernelRelease,
		PrimaryMAC:    primaryMAC,
		PhysicalMACs:  allMACs,
	}, nil
}

func (s Snapshot) FingerprintHash(salt string) string {
	allMACsHash := licenseverify.HashValue(strings.Join(s.PhysicalMACs, ","))
	return licenseverify.FingerprintHash(
		s.MachineID,
		s.PrimaryMAC,
		allMACsHash,
		s.Hostname,
		s.KernelRelease,
		salt,
	)
}

func (s Snapshot) Binding(salt string) licenseverify.MachineBinding {
	return licenseverify.MachineBinding{
		MachineIDHash:   licenseverify.HashValue(s.MachineID),
		PrimaryMACHash:  licenseverify.HashValue(s.PrimaryMAC),
		FingerprintHash: s.FingerprintHash(salt),
	}
}

func readMachineID(path string) (string, error) {
	paths := []string{path}
	if path == "" {
		paths = []string{defaultMachineIDPath, fallbackMachineIDPath}
	}
	for _, candidate := range paths {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		raw, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		machineID := strings.TrimSpace(string(raw))
		if machineID != "" {
			return machineID, nil
		}
	}
	return "", errors.New("machine id is required for license fingerprint")
}

func physicalMACs(interfaces []net.Interface) []string {
	macs := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		macs = append(macs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(macs)
	return macs
}

func defaultRouteInterface(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("route table is empty")
	}
	bestIface := ""
	bestMetric := int(^uint(0) >> 1)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || fields[1] != "00000000" {
			continue
		}
		metric := 0
		if len(fields) > 6 {
			if parsed, err := strconv.Atoi(fields[6]); err == nil {
				metric = parsed
			}
		}
		if bestIface == "" || metric < bestMetric {
			bestIface = fields[0]
			bestMetric = metric
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if bestIface == "" {
		return "", errors.New("default route interface not found")
	}
	return bestIface, nil
}

func primaryMACForInterface(interfaces []net.Interface, name string) string {
	if name == "" {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Name == name && len(iface.HardwareAddr) > 0 {
			return strings.ToLower(iface.HardwareAddr.String())
		}
	}
	return ""
}

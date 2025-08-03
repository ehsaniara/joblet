package collectors

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// HostCollector collects host system information
type HostCollector struct {
	logger     *logger.Logger
	cachedInfo *domain.HostInfo
	lastUpdate time.Time
}

// NewHostCollector creates a new host information collector
func NewHostCollector() *HostCollector {
	return &HostCollector{
		logger: logger.WithField("component", "host-collector"),
	}
}

// Collect gathers current host information
func (c *HostCollector) Collect() (*domain.HostInfo, error) {
	// Cache host info for 5 minutes since it doesn't change frequently
	if c.cachedInfo != nil && time.Since(c.lastUpdate) < 5*time.Minute {
		// Update only the dynamic fields
		uptime, bootTime, err := c.getUptimeInfo()
		if err == nil {
			c.cachedInfo.Uptime = uptime
			c.cachedInfo.BootTime = bootTime
		}
		return c.cachedInfo, nil
	}

	hostname, err := c.getHostname()
	if err != nil {
		hostname = "unknown"
	}

	osInfo, err := c.getOSInfo()
	if err != nil {
		osInfo = "unknown"
	}

	kernelVersion, err := c.getKernelVersion()
	if err != nil {
		kernelVersion = "unknown"
	}

	uptime, bootTime, err := c.getUptimeInfo()
	if err != nil {
		uptime = 0
		bootTime = time.Time{}
	}

	hostInfo := &domain.HostInfo{
		Hostname:     hostname,
		OS:           osInfo,
		Kernel:       kernelVersion,
		Architecture: runtime.GOARCH,
		Uptime:       uptime,
		BootTime:     bootTime,
	}

	// Cache the result
	c.cachedInfo = hostInfo
	c.lastUpdate = time.Now()

	return hostInfo, nil
}

// getHostname reads the system hostname
func (c *HostCollector) getHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		// Try reading from /proc/sys/kernel/hostname
		data, fileErr := os.ReadFile("/proc/sys/kernel/hostname")
		if fileErr != nil {
			return "", err
		}
		hostname = strings.TrimSpace(string(data))
	}
	return hostname, nil
}

// getOSInfo reads OS information from /etc/os-release
func (c *HostCollector) getOSInfo() (string, error) {
	// Try /etc/os-release first (systemd standard)
	if osInfo := c.parseOSRelease("/etc/os-release"); osInfo != "" {
		return osInfo, nil
	}

	// Try /usr/lib/os-release as fallback
	if osInfo := c.parseOSRelease("/usr/lib/os-release"); osInfo != "" {
		return osInfo, nil
	}

	// Try /etc/lsb-release (Ubuntu/Debian)
	if osInfo := c.parseLSBRelease("/etc/lsb-release"); osInfo != "" {
		return osInfo, nil
	}

	// Try reading /etc/issue as last resort
	data, err := os.ReadFile("/etc/issue")
	if err != nil {
		return "", err
	}

	// Clean up the issue string
	issue := strings.TrimSpace(string(data))
	// Remove escape sequences and extra info
	lines := strings.Split(issue, "\n")
	if len(lines) > 0 {
		issue = strings.TrimSpace(lines[0])
		// Remove common suffixes
		issue = strings.ReplaceAll(issue, "\\n", "")
		issue = strings.ReplaceAll(issue, "\\l", "")
		issue = strings.TrimSpace(issue)
	}

	if issue == "" {
		return "Linux", nil
	}

	return issue, nil
}

// parseOSRelease parses os-release format files
func (c *HostCollector) parseOSRelease(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	var prettyName, name, version string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = c.parseQuotedValue(line[12:])
		} else if strings.HasPrefix(line, "NAME=") {
			name = c.parseQuotedValue(line[5:])
		} else if strings.HasPrefix(line, "VERSION=") {
			version = c.parseQuotedValue(line[8:])
		}
	}

	// Prefer PRETTY_NAME, fallback to NAME + VERSION
	if prettyName != "" {
		return prettyName
	}
	if name != "" && version != "" {
		return fmt.Sprintf("%s %s", name, version)
	}
	if name != "" {
		return name
	}

	return ""
}

// parseLSBRelease parses lsb-release format files
func (c *HostCollector) parseLSBRelease(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return ""
	}
	defer file.Close()

	var description, distId, release string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "DISTRIB_DESCRIPTION=") {
			description = c.parseQuotedValue(line[20:])
		} else if strings.HasPrefix(line, "DISTRIB_ID=") {
			distId = c.parseQuotedValue(line[11:])
		} else if strings.HasPrefix(line, "DISTRIB_RELEASE=") {
			release = c.parseQuotedValue(line[16:])
		}
	}

	if description != "" {
		return description
	}
	if distId != "" && release != "" {
		return fmt.Sprintf("%s %s", distId, release)
	}
	if distId != "" {
		return distId
	}

	return ""
}

// parseQuotedValue removes quotes from a value
func (c *HostCollector) parseQuotedValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// getKernelVersion reads kernel version from /proc/version
func (c *HostCollector) getKernelVersion() (string, error) {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(data))

	// Extract just the version number part
	fields := strings.Fields(version)
	if len(fields) >= 3 {
		return fields[2], nil // Third field is usually the version
	}

	return version, nil
}

// getUptimeInfo reads system uptime from /proc/uptime
func (c *HostCollector) getUptimeInfo() (time.Duration, time.Time, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, time.Time{}, err
	}

	fields := strings.Fields(strings.TrimSpace(string(data)))
	if len(fields) < 1 {
		return 0, time.Time{}, fmt.Errorf("invalid uptime format")
	}

	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, time.Time{}, err
	}

	uptime := time.Duration(uptimeSeconds * float64(time.Second))
	bootTime := time.Now().Add(-uptime)

	return uptime, bootTime, nil
}

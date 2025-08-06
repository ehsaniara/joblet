package network

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// DNSManager handles DNS resolution for job networks
type DNSManager struct {
	stateDir string
}

// NewDNSManager creates a new DNS manager
func NewDNSManager(stateDir string) *DNSManager {
	return &DNSManager{
		stateDir: stateDir,
	}
}

// SetupJobDNS configures DNS resolution for a job within its network namespace.
// It generates custom hosts file entries for hostname resolution between jobs in the same network,
// enabling jobs to communicate using hostnames rather than raw IP addresses.
func (dm *DNSManager) SetupJobDNS(pid int, alloc *JobAllocation, networkJobs map[string]*JobAllocation) error {
	// Skip DNS setup for special networks
	if alloc.Network == "none" || alloc.Network == "isolated" {
		return nil
	}

	// Create hosts content
	hostsContent := dm.generateHostsContent(alloc, networkJobs)

	// Write to temporary file
	hostsPath := filepath.Join(dm.stateDir, fmt.Sprintf("hosts-%s", alloc.JobID))
	if err := os.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	// Setup bind mount into container
	return dm.bindMountHosts(pid, hostsPath)
}

// generateHostsContent creates /etc/hosts content for the job
func (dm *DNSManager) generateHostsContent(alloc *JobAllocation, networkJobs map[string]*JobAllocation) string {
	var builder strings.Builder

	// Standard entries
	builder.WriteString("127.0.0.1   localhost\n")
	builder.WriteString("::1         localhost\n")

	// Job's own entry
	if alloc.IP != nil {
		builder.WriteString(fmt.Sprintf("%s   %s\n", alloc.IP.String(), alloc.Hostname))

		// short hostname alias
		if shortName := getShortHostname(alloc.Hostname); shortName != "" {
			builder.WriteString(fmt.Sprintf("%s   %s\n", alloc.IP.String(), shortName))
		}
	}

	// Other jobs in the same network
	builder.WriteString("\n# Other jobs in network\n")
	for jobID, job := range networkJobs {
		if jobID != alloc.JobID && job.IP != nil {
			builder.WriteString(fmt.Sprintf("%s   %s\n", job.IP.String(), job.Hostname))

			// short alias for easier access
			if shortName := getShortHostname(job.Hostname); shortName != "" {
				builder.WriteString(fmt.Sprintf("%s   %s\n", job.IP.String(), shortName))
			}
		}
	}

	return builder.String()
}

// bindMountHosts mounts the hosts file into the container namespace
func (dm *DNSManager) bindMountHosts(pid int, _ string) error {
	// Target path in the container
	targetPath := fmt.Sprintf("/proc/%d/root/etc/hosts", pid)

	// Create a bind mount
	// Note: This requires the process to have its filesystem set up
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// For containers with mount namespace, we need to enter the namespace
	// This is handled by the filesystem isolation layer
	// Here we just prepare the file for mounting

	return nil
}

// CleanupJobDNS removes DNS configuration for a job
func (dm *DNSManager) CleanupJobDNS(jobID string) error {
	hostsPath := filepath.Join(dm.stateDir, fmt.Sprintf("hosts-%s", jobID))

	// Remove the hosts file
	if err := os.Remove(hostsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hosts file: %w", err)
	}

	return nil
}

// UpdateNetworkDNS updates DNS entries when jobs join/leave
func (dm *DNSManager) UpdateNetworkDNS(networkName string, activeJobs []*JobAllocation) error {
	// Build job map for the network - FIXED: use JobAllocation from network package
	jobMap := make(map[string]*JobAllocation)
	for _, job := range activeJobs {
		jobMap[job.JobID] = job
	}

	// Update each job's hosts file
	for _, job := range activeJobs {
		// Skip if job doesn't have a valid PID (not running)
		pid := dm.getJobPID(job.JobID)
		if pid == 0 {
			continue
		}

		// Regenerate hosts file with updated entries
		if err := dm.SetupJobDNS(pid, job, jobMap); err != nil {
			// Log but don't fail - job might be terminating
			continue
		}
	}

	return nil
}

// getJobPID retrieves the PID for a running job
func (dm *DNSManager) getJobPID(jobID string) int {
	// This would integrate with the job state management
	// For now, return 0 to indicate not found
	// In real implementation, this would query the job state
	return 0
}

// Helper function to get short hostname
// getShortHostname extracts a short alias from a full hostname for DNS resolution.
// It simplifies job hostnames by removing prefixes and provides convenient short names
// for easier inter-job communication within networks.
func getShortHostname(hostname string) string {
	// Extract meaningful short name from hostname
	// e.g., "job_abc123" -> "abc123"
	if strings.HasPrefix(hostname, "job_") {
		return strings.TrimPrefix(hostname, "job_")
	}

	// For other patterns, return first part before any dots
	parts := strings.Split(hostname, ".")
	if len(parts) > 0 && parts[0] != hostname {
		return parts[0]
	}

	return ""
}

// ResolveHostname resolves a hostname within a network context
func (dm *DNSManager) ResolveHostname(hostname, network string, networkJobs map[string]*JobAllocation) (net.IP, error) {
	// Check if it's already an IP
	if ip := net.ParseIP(hostname); ip != nil {
		return ip, nil
	}

	// Search for matching hostname in network
	for _, job := range networkJobs {
		if job.Hostname == hostname || getShortHostname(job.Hostname) == hostname {
			if job.IP != nil {
				return job.IP, nil
			}
		}
	}

	return nil, fmt.Errorf("hostname '%s' not found in network '%s'", hostname, network)
}

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"joblet/internal/joblet/monitoring/domain"
	"joblet/pkg/logger"
)

// Detector provides cloud environment detection capabilities
type Detector struct {
	logger   *logger.Logger
	client   *http.Client
	cached   *domain.CloudInfo
	lastScan time.Time
}

// NewDetector creates a new cloud environment detector
func NewDetector() *Detector {
	return &Detector{
		logger: logger.WithField("component", "cloud-detector"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// DetectCloudEnvironment detects the current cloud environment
func (d *Detector) DetectCloudEnvironment(ctx context.Context) (*domain.CloudInfo, error) {
	// Cache results for 1 hour since cloud metadata doesn't change
	if d.cached != nil && time.Since(d.lastScan) < time.Hour {
		return d.cached, nil
	}

	d.logger.Debug("detecting cloud environment")

	// Try different detection methods in order of reliability
	detectors := []func(context.Context) (*domain.CloudInfo, error){
		d.detectAWS,
		d.detectAzure,
		d.detectGCP,
		d.detectDigitalOcean,
		d.detectOpenStack,
		d.detectHypervisor, // Generic hypervisor detection
	}

	for _, detector := range detectors {
		if cloudInfo, err := detector(ctx); err == nil && cloudInfo != nil {
			d.cached = cloudInfo
			d.lastScan = time.Now()
			d.logger.Info("detected cloud environment", "provider", cloudInfo.Provider)
			return cloudInfo, nil
		}
	}

	d.logger.Debug("no cloud environment detected, assuming bare metal")
	return nil, nil
}

// detectAWS detects AWS EC2 environment
func (d *Detector) detectAWS(ctx context.Context) (*domain.CloudInfo, error) {
	// Check for AWS DMI information first
	if !d.checkDMIVendor("Amazon EC2") {
		return nil, fmt.Errorf("not AWS EC2")
	}

	// Use IMDSv2 (more secure)
	token, err := d.getAWSToken(ctx)
	if err != nil {
		d.logger.Debug("failed to get AWS token", "error", err)
		return nil, err
	}

	metadata, err := d.getAWSMetadata(ctx, token)
	if err != nil {
		d.logger.Debug("failed to get AWS metadata", "error", err)
		return nil, err
	}

	return metadata, nil
}

// getAWSToken gets an IMDSv2 token
func (d *Detector) getAWSToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", "http://169.254.169.254/latest/api/token", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")

	resp, err := d.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get token: %d", resp.StatusCode)
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// getAWSMetadata retrieves AWS instance metadata
func (d *Detector) getAWSMetadata(ctx context.Context, token string) (*domain.CloudInfo, error) {
	baseURL := "http://169.254.169.254/latest/meta-data"

	instanceID := d.getMetadataField(ctx, token, baseURL+"/instance-id")
	instanceType := d.getMetadataField(ctx, token, baseURL+"/instance-type")
	region := d.getMetadataField(ctx, token, baseURL+"/placement/region")
	zone := d.getMetadataField(ctx, token, baseURL+"/placement/availability-zone")

	cloudInfo := &domain.CloudInfo{
		Provider:     "AWS",
		Region:       region,
		Zone:         zone,
		InstanceID:   instanceID,
		InstanceType: instanceType,
		Metadata:     make(map[string]string),
	}

	// Try to get hypervisor type
	if hypervisor := d.getMetadataField(ctx, token, baseURL+"/services/domain"); hypervisor != "" {
		cloudInfo.HypervisorType = hypervisor
	} else {
		cloudInfo.HypervisorType = "xen" // Default for AWS
	}

	return cloudInfo, nil
}

// detectAzure detects Azure environment
func (d *Detector) detectAzure(ctx context.Context) (*domain.CloudInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata", "true")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not Azure: %d", resp.StatusCode)
	}

	var metadata struct {
		Compute struct {
			AzEnvironment     string `json:"azEnvironment"`
			Location          string `json:"location"`
			Name              string `json:"name"`
			ResourceGroupName string `json:"resourceGroupName"`
			SubscriptionID    string `json:"subscriptionId"`
			VMId              string `json:"vmId"`
			VMSize            string `json:"vmSize"`
			Zone              string `json:"zone"`
		} `json:"compute"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	return &domain.CloudInfo{
		Provider:       "Azure",
		Region:         metadata.Compute.Location,
		Zone:           metadata.Compute.Zone,
		InstanceID:     metadata.Compute.VMId,
		InstanceType:   metadata.Compute.VMSize,
		HypervisorType: "hyper-v",
		Metadata: map[string]string{
			"resourceGroup": metadata.Compute.ResourceGroupName,
			"subscription":  metadata.Compute.SubscriptionID,
			"environment":   metadata.Compute.AzEnvironment,
		},
	}, nil
}

// detectGCP detects Google Cloud Platform environment
func (d *Detector) detectGCP(ctx context.Context) (*domain.CloudInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://metadata.google.internal/computeMetadata/v1/instance/?recursive=true", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not GCP: %d", resp.StatusCode)
	}

	var metadata struct {
		ID          int64  `json:"id"`
		MachineType string `json:"machineType"`
		Name        string `json:"name"`
		Zone        string `json:"zone"`
		Hostname    string `json:"hostname"`
		Project     struct {
			ProjectID string `json:"projectId"`
		} `json:"project"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	// Extract region from zone (zone format: projects/PROJECT/zones/ZONE)
	zone := metadata.Zone
	if parts := strings.Split(zone, "/"); len(parts) > 0 {
		zone = parts[len(parts)-1]
	}

	region := zone
	if idx := strings.LastIndex(zone, "-"); idx > 0 {
		region = zone[:idx] // Remove last part after dash
	}

	// Extract machine type
	machineType := metadata.MachineType
	if parts := strings.Split(machineType, "/"); len(parts) > 0 {
		machineType = parts[len(parts)-1]
	}

	return &domain.CloudInfo{
		Provider:       "GCP",
		Region:         region,
		Zone:           zone,
		InstanceID:     fmt.Sprintf("%d", metadata.ID),
		InstanceType:   machineType,
		HypervisorType: "kvm",
		Metadata: map[string]string{
			"project": metadata.Project.ProjectID,
		},
	}, nil
}

// detectDigitalOcean detects DigitalOcean environment
func (d *Detector) detectDigitalOcean(ctx context.Context) (*domain.CloudInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/v1.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not DigitalOcean: %d", resp.StatusCode)
	}

	var metadata struct {
		DropletID int    `json:"droplet_id"`
		Hostname  string `json:"hostname"`
		Region    string `json:"region"`
		Features  struct {
			Virtio bool `json:"virtio"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	return &domain.CloudInfo{
		Provider:       "DigitalOcean",
		Region:         metadata.Region,
		Zone:           metadata.Region, // DO doesn't have zones
		InstanceID:     fmt.Sprintf("%d", metadata.DropletID),
		InstanceType:   "droplet",
		HypervisorType: "kvm",
		Metadata:       make(map[string]string),
	}, nil
}

// detectOpenStack detects OpenStack environment
func (d *Detector) detectOpenStack(ctx context.Context) (*domain.CloudInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/openstack/latest/meta_data.json", nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("not OpenStack: %d", resp.StatusCode)
	}

	var metadata struct {
		UUID             string `json:"uuid"`
		Name             string `json:"name"`
		AvailabilityZone string `json:"availability_zone"`
		Hostname         string `json:"hostname"`
		LaunchIndex      int    `json:"launch_index"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, err
	}

	return &domain.CloudInfo{
		Provider:       "OpenStack",
		Region:         "unknown",
		Zone:           metadata.AvailabilityZone,
		InstanceID:     metadata.UUID,
		InstanceType:   "instance",
		HypervisorType: "kvm",
		Metadata:       make(map[string]string),
	}, nil
}

// Helper methods

func (d *Detector) getMetadataField(ctx context.Context, token, url string) string {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	if token != "" {
		req.Header.Set("X-aws-ec2-metadata-token", token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

func (d *Detector) checkDMIVendor(expectedVendor string) bool {
	vendor := d.getDMIValue("sys_vendor")
	return strings.Contains(strings.ToLower(vendor), strings.ToLower(expectedVendor))
}

func (d *Detector) getDMIValue(field string) string {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/dmi/id/%s", field))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// detectHypervisor detects hypervisor from DMI information
func (d *Detector) detectHypervisor(ctx context.Context) (*domain.CloudInfo, error) {
	vendor := d.getDMIValue("sys_vendor")
	product := d.getDMIValue("product_name")
	version := d.getDMIValue("product_version")

	// Check for common hypervisors
	var hypervisorType, provider string

	lowerVendor := strings.ToLower(vendor)
	lowerProduct := strings.ToLower(product)

	switch {
	case strings.Contains(lowerVendor, "vmware") || strings.Contains(lowerProduct, "vmware"):
		hypervisorType = "vmware"
		provider = "VMware"
	case strings.Contains(lowerVendor, "microsoft") || strings.Contains(lowerProduct, "virtual machine"):
		hypervisorType = "hyper-v"
		provider = "Hyper-V"
	case strings.Contains(lowerVendor, "qemu") || strings.Contains(lowerProduct, "kvm"):
		hypervisorType = "kvm"
		provider = "KVM"
	case strings.Contains(lowerVendor, "xen") || strings.Contains(lowerProduct, "xen"):
		hypervisorType = "xen"
		provider = "Xen"
	case strings.Contains(lowerProduct, "virtualbox"):
		hypervisorType = "virtualbox"
		provider = "VirtualBox"
	case strings.Contains(lowerProduct, "parallels"):
		hypervisorType = "parallels"
		provider = "Parallels"
	default:
		return nil, fmt.Errorf("no hypervisor detected")
	}

	return &domain.CloudInfo{
		Provider:       provider,
		Region:         "unknown",
		Zone:           "unknown",
		InstanceID:     "unknown",
		InstanceType:   "virtual-machine",
		HypervisorType: hypervisorType,
		Metadata: map[string]string{
			"vendor":  vendor,
			"product": product,
			"version": version,
		},
	}, nil
}

// IsVirtualized checks if the system is running in a virtualized environment
func (d *Detector) IsVirtualized() bool {
	// Check common indicators of virtualization
	indicators := []string{
		"/proc/xen",
		"/sys/bus/xen",
		"/proc/device-tree/hypervisor",
	}

	for _, path := range indicators {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	// Check DMI for hypervisor signatures
	vendor := d.getDMIValue("sys_vendor")
	product := d.getDMIValue("product_name")

	hypervisorPatterns := []string{
		"vmware", "qemu", "kvm", "xen", "microsoft corporation",
		"parallels", "virtualbox", "amazon ec2",
	}

	checkString := strings.ToLower(vendor + " " + product)
	for _, pattern := range hypervisorPatterns {
		if strings.Contains(checkString, pattern) {
			return true
		}
	}

	// Check cpuinfo for hypervisor flag
	if cpuinfo, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		if matched, _ := regexp.MatchString(`flags\s*:.*hypervisor`, string(cpuinfo)); matched {
			return true
		}
	}

	return false
}

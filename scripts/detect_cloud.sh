#!/bin/bash
# Cloud environment detection script for Joblet
# Detects AWS EC2, Google Cloud, Azure, DigitalOcean, and other cloud providers

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

detect_aws_ec2() {
    # Check EC2 instance metadata service
    if curl -s --max-time 1 http://169.254.169.254/latest/dynamic/instance-identity/document >/dev/null 2>&1; then
        return 0
    fi

    # Check for EC2 in DMI
    if grep -qi "amazon\|ec2" /sys/class/dmi/id/sys_vendor 2>/dev/null || \
       grep -qi "amazon\|ec2" /sys/class/dmi/id/bios_vendor 2>/dev/null; then
        return 0
    fi

    return 1
}

detect_gcp() {
    # Check GCP metadata service
    if curl -s --max-time 1 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/id >/dev/null 2>&1; then
        return 0
    fi

    # Check for Google in DMI
    if grep -qi "google" /sys/class/dmi/id/sys_vendor 2>/dev/null || \
       grep -qi "google" /sys/class/dmi/id/bios_vendor 2>/dev/null; then
        return 0
    fi

    return 1
}

detect_azure() {
    # Check Azure metadata service
    if curl -s --max-time 1 -H "Metadata: true" http://169.254.169.254/metadata/instance?api-version=2021-02-01 >/dev/null 2>&1; then
        return 0
    fi

    # Check for Microsoft/Azure in DMI
    if grep -qi "microsoft" /sys/class/dmi/id/sys_vendor 2>/dev/null || \
       grep -qi "microsoft" /sys/class/dmi/id/bios_vendor 2>/dev/null; then
        return 0
    fi

    # Check for Azure specific files
    if [ -f /var/lib/waagent/ovf-env.xml ] || [ -d /var/lib/waagent ]; then
        return 0
    fi

    return 1
}

detect_digitalocean() {
    # Check DigitalOcean metadata service
    if curl -s --max-time 1 http://169.254.169.254/metadata/v1/id >/dev/null 2>&1; then
        return 0
    fi

    # Check for DigitalOcean in DMI
    if grep -qi "digitalocean" /sys/class/dmi/id/sys_vendor 2>/dev/null || \
       grep -qi "digitalocean" /sys/class/dmi/id/bios_vendor 2>/dev/null; then
        return 0
    fi

    return 1
}

detect_vultr() {
    # Check Vultr metadata service
    if curl -s --max-time 1 http://169.254.169.254/v1.json >/dev/null 2>&1; then
        local response=$(curl -s --max-time 1 http://169.254.169.254/v1.json 2>/dev/null)
        if echo "$response" | grep -q "instanceid"; then
            return 0
        fi
    fi

    return 1
}

detect_linode() {
    # Check for Linode in DMI
    if grep -qi "linode" /sys/class/dmi/id/sys_vendor 2>/dev/null || \
       grep -qi "linode" /sys/class/dmi/id/bios_vendor 2>/dev/null; then
        return 0
    fi

    return 1
}

detect_cloud_environment() {
    local cloud_provider="none"
    local instance_id=""
    local public_ip=""
    local private_ip=""
    local region=""
    local hostname=""

    if detect_aws_ec2; then
        cloud_provider="aws_ec2"
        instance_id=$(curl -s --max-time 2 http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null)
        public_ip=$(curl -s --max-time 2 http://169.254.169.254/latest/meta-data/public-ipv4 2>/dev/null)
        private_ip=$(curl -s --max-time 2 http://169.254.169.254/latest/meta-data/local-ipv4 2>/dev/null)
        region=$(curl -s --max-time 2 http://169.254.169.254/latest/meta-data/placement/availability-zone 2>/dev/null | sed 's/[a-z]$//')
        hostname=$(curl -s --max-time 2 http://169.254.169.254/latest/meta-data/public-hostname 2>/dev/null)

    elif detect_gcp; then
        cloud_provider="gcp"
        instance_id=$(curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/id 2>/dev/null)
        public_ip=$(curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/access-configs/0/external-ip 2>/dev/null)
        private_ip=$(curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/network-interfaces/0/ip 2>/dev/null)
        region=$(curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/zone 2>/dev/null | awk -F/ '{print $NF}' | sed 's/-[a-z]$//')
        hostname=$(curl -s --max-time 2 -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/hostname 2>/dev/null)

    elif detect_azure; then
        cloud_provider="azure"
        local azure_metadata=$(curl -s --max-time 2 -H "Metadata: true" "http://169.254.169.254/metadata/instance?api-version=2021-02-01" 2>/dev/null)
        if [ -n "$azure_metadata" ]; then
            instance_id=$(echo "$azure_metadata" | grep -o '"vmId":"[^"]*' | cut -d'"' -f4)
            public_ip=$(echo "$azure_metadata" | grep -o '"publicIpAddress":"[^"]*' | cut -d'"' -f4)
            private_ip=$(echo "$azure_metadata" | grep -o '"privateIpAddress":"[^"]*' | cut -d'"' -f4)
            region=$(echo "$azure_metadata" | grep -o '"location":"[^"]*' | cut -d'"' -f4)
        fi

    elif detect_digitalocean; then
        cloud_provider="digitalocean"
        instance_id=$(curl -s --max-time 2 http://169.254.169.254/metadata/v1/id 2>/dev/null)
        public_ip=$(curl -s --max-time 2 http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address 2>/dev/null)
        private_ip=$(curl -s --max-time 2 http://169.254.169.254/metadata/v1/interfaces/private/0/ipv4/address 2>/dev/null)
        region=$(curl -s --max-time 2 http://169.254.169.254/metadata/v1/region 2>/dev/null)
        hostname=$(curl -s --max-time 2 http://169.254.169.254/metadata/v1/hostname 2>/dev/null)

    elif detect_vultr; then
        cloud_provider="vultr"
        local vultr_metadata=$(curl -s --max-time 2 http://169.254.169.254/v1.json 2>/dev/null)
        if [ -n "$vultr_metadata" ]; then
            instance_id=$(echo "$vultr_metadata" | grep -o '"instanceid":"[^"]*' | cut -d'"' -f4)
            public_ip=$(echo "$vultr_metadata" | grep -o '"main_ip":"[^"]*' | cut -d'"' -f4)
            private_ip=$(echo "$vultr_metadata" | grep -o '"internal_ip":"[^"]*' | cut -d'"' -f4)
            region=$(echo "$vultr_metadata" | grep -o '"region":"[^"]*' | cut -d'"' -f4)
            hostname=$(echo "$vultr_metadata" | grep -o '"hostname":"[^"]*' | cut -d'"' -f4)
        fi

    elif detect_linode; then
        cloud_provider="linode"
        # Linode doesn't have a standard metadata service
        # Try to get public IP from external service
        public_ip=$(curl -s --max-time 2 https://checkip.amazonaws.com 2>/dev/null | grep -E '^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$')
    fi

    # Output in format that can be sourced
    cat << EOF
CLOUD_PROVIDER="$cloud_provider"
CLOUD_INSTANCE_ID="$instance_id"
CLOUD_PUBLIC_IP="$public_ip"
CLOUD_PRIVATE_IP="$private_ip"
CLOUD_REGION="$region"
CLOUD_HOSTNAME="$hostname"
EOF
}

display_cloud_info() {
    eval "$(detect_cloud_environment)"

    echo -e "${BLUE}☁️  Cloud Environment Detection${NC}"
    echo "================================"

    if [ "$CLOUD_PROVIDER" = "none" ]; then
        echo -e "${YELLOW}No cloud environment detected${NC}"
        echo "Running on bare metal or unrecognized virtualization platform"
    else
        echo -e "${GREEN}✓ Cloud Provider:${NC} $CLOUD_PROVIDER"

        if [ -n "$CLOUD_INSTANCE_ID" ]; then
            echo -e "${GREEN}✓ Instance ID:${NC} $CLOUD_INSTANCE_ID"
        fi

        if [ -n "$CLOUD_PUBLIC_IP" ]; then
            echo -e "${GREEN}✓ Public IP:${NC} $CLOUD_PUBLIC_IP"
        fi

        if [ -n "$CLOUD_PRIVATE_IP" ]; then
            echo -e "${GREEN}✓ Private IP:${NC} $CLOUD_PRIVATE_IP"
        fi

        if [ -n "$CLOUD_REGION" ]; then
            echo -e "${GREEN}✓ Region:${NC} $CLOUD_REGION"
        fi

        if [ -n "$CLOUD_HOSTNAME" ]; then
            echo -e "${GREEN}✓ Hostname:${NC} $CLOUD_HOSTNAME"
        fi
    fi

    echo
}

if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    case "${1:-display}" in
        --source|-s)
            detect_cloud_environment
            ;;
        --json|-j)
            eval "$(detect_cloud_environment)"
            cat << EOF
{
  "cloud_provider": "$CLOUD_PROVIDER",
  "instance_id": "$CLOUD_INSTANCE_ID",
  "public_ip": "$CLOUD_PUBLIC_IP",
  "private_ip": "$CLOUD_PRIVATE_IP",
  "region": "$CLOUD_REGION",
  "hostname": "$CLOUD_HOSTNAME"
}
EOF
            ;;
        *)
            display_cloud_info
            ;;
    esac
fi
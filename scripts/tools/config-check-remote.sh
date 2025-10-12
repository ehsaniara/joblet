#!/bin/bash

echo "🔍 Checking configuration status on $(hostname)..."

# Check directory structure
echo "📁 Checking directory structure..."
sudo ls -la /opt/joblet/ || echo 'Directory /opt/joblet/ not found'

# Check configuration files
echo "📋 Checking configuration files..."
sudo ls -la /opt/joblet/config/ || echo 'Configuration directory not found'

# Check embedded certificates in server config
echo "🔐 Checking embedded certificates in server config..."
sudo grep -c 'BEGIN CERTIFICATE' /opt/joblet/config/joblet-config.yml 2>/dev/null | xargs echo 'Certificates found:' || echo 'No embedded certificates found'
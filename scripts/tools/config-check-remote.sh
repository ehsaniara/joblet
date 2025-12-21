#!/bin/bash

# JOBLET_HOME defines the installation directory (default: /opt/joblet)
JOBLET_HOME="${JOBLET_HOME:-/opt/joblet}"

echo "🔍 Checking configuration status on $(hostname)..."

# Check directory structure
echo "📁 Checking directory structure..."
sudo ls -la ${JOBLET_HOME}/ || echo "Directory ${JOBLET_HOME}/ not found"

# Check configuration files
echo "📋 Checking configuration files..."
sudo ls -la ${JOBLET_HOME}/config/ || echo 'Configuration directory not found'

# Check embedded certificates in server config
echo "🔐 Checking embedded certificates in server config..."
sudo grep -c 'BEGIN CERTIFICATE' ${JOBLET_HOME}/config/joblet-config.yml 2>/dev/null | xargs echo 'Certificates found:' || echo 'No embedded certificates found'

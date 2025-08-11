#!/bin/bash

# System Detection Library for Multi-Architecture Runtime Setup
# Provides standardized detection of CPU architecture, OS distribution, and package manager

# Global variables for system detection
DETECTED_ARCH=""
DETECTED_OS=""
DETECTED_DISTRO=""
DETECTED_PACKAGE_MANAGER=""
DETECTED_INSTALL_CMD=""
DETECTED_REMOVE_CMD=""

# Detect CPU architecture
detect_architecture() {
    local arch=$(uname -m)
    case "$arch" in
        x86_64|amd64)
            DETECTED_ARCH="amd64"
            ;;
        aarch64|arm64)
            DETECTED_ARCH="arm64"
            ;;
        armv7l|armhf)
            DETECTED_ARCH="armhf"
            ;;
        *)
            echo "❌ Unsupported architecture: $arch"
            echo "Supported architectures: x86_64/amd64, aarch64/arm64, armv7l/armhf"
            return 1
            ;;
    esac
    echo "✅ Detected architecture: $DETECTED_ARCH (raw: $arch)"
}

# Detect OS and distribution
detect_os_distribution() {
    DETECTED_OS=$(uname -s)
    
    if [[ "$DETECTED_OS" != "Linux" ]]; then
        echo "❌ Only Linux is supported for runtime installation"
        echo "Detected OS: $DETECTED_OS"
        return 1
    fi
    
    # Detect Linux distribution
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        DETECTED_DISTRO="$ID"
        
        case "$DETECTED_DISTRO" in
            ubuntu)
                DETECTED_PACKAGE_MANAGER="apt"
                DETECTED_INSTALL_CMD="apt-get update -qq && apt-get install -y"
                DETECTED_REMOVE_CMD="apt-get remove -y"
                ;;
            debian)
                DETECTED_PACKAGE_MANAGER="apt"
                DETECTED_INSTALL_CMD="apt-get update -qq && apt-get install -y"
                DETECTED_REMOVE_CMD="apt-get remove -y"
                ;;
            centos|rhel|rocky|almalinux|amzn)
                DETECTED_PACKAGE_MANAGER="yum"
                DETECTED_INSTALL_CMD="yum install -y"
                DETECTED_REMOVE_CMD="yum remove -y"
                ;;
            fedora)
                DETECTED_PACKAGE_MANAGER="dnf"
                DETECTED_INSTALL_CMD="dnf install -y"
                DETECTED_REMOVE_CMD="dnf remove -y"
                ;;
            opensuse|sles)
                DETECTED_PACKAGE_MANAGER="zypper"
                DETECTED_INSTALL_CMD="zypper install -y"
                DETECTED_REMOVE_CMD="zypper remove -y"
                ;;
            arch|manjaro)
                DETECTED_PACKAGE_MANAGER="pacman"
                DETECTED_INSTALL_CMD="pacman -Sy --noconfirm"
                DETECTED_REMOVE_CMD="pacman -R --noconfirm"
                ;;
            alpine)
                DETECTED_PACKAGE_MANAGER="apk"
                DETECTED_INSTALL_CMD="apk add --no-cache"
                DETECTED_REMOVE_CMD="apk del"
                ;;
            *)
                echo "❌ Unsupported Linux distribution: $DETECTED_DISTRO"
                echo "Supported distributions: Ubuntu, Debian, CentOS/RHEL/Amazon Linux, Fedora, openSUSE, Arch, Alpine"
                return 1
                ;;
        esac
        
        echo "✅ Detected distribution: $DETECTED_DISTRO"
        echo "✅ Package manager: $DETECTED_PACKAGE_MANAGER"
    else
        echo "❌ Cannot detect Linux distribution (/etc/os-release not found)"
        return 1
    fi
}

# Get Java download URL based on architecture
get_java_download_url() {
    local java_version="$1"
    local base_url=""
    local arch_suffix=""
    
    case "$DETECTED_ARCH" in
        amd64)
            arch_suffix="x64_linux"
            ;;
        arm64)
            arch_suffix="aarch64_linux"
            ;;
        armhf)
            echo "❌ Java: ARM 32-bit (armhf) not supported by Eclipse Adoptium"
            return 1
            ;;
        *)
            echo "❌ Java: Unsupported architecture for Java: $DETECTED_ARCH"
            return 1
            ;;
    esac
    
    case "$java_version" in
        17)
            base_url="https://github.com/adoptium/temurin17-binaries/releases/download/jdk-17.0.12%2B7/OpenJDK17U-jdk_${arch_suffix}_hotspot_17.0.12_7.tar.gz"
            ;;
        21)
            base_url="https://github.com/adoptium/temurin21-binaries/releases/download/jdk-21.0.4%2B7/OpenJDK21U-jdk_${arch_suffix}_hotspot_21.0.4_7.tar.gz"
            ;;
        *)
            echo "❌ Java: Unsupported Java version: $java_version"
            return 1
            ;;
    esac
    
    echo "$base_url"
}

# Get Python architecture-specific package names
get_python_packages() {
    local packages=""
    
    case "$DETECTED_PACKAGE_MANAGER" in
        apt)
            packages="python3 python3-dev python3-pip python3-venv build-essential libssl-dev libffi-dev"
            ;;
        yum|dnf)
            # Amazon Linux 2 uses python3, Amazon Linux 2023 uses python3.11
            packages="python3 python3-devel python3-pip python3-venv gcc openssl-devel libffi-devel"
            # Check for Amazon Linux specific packages
            if [[ "$DETECTED_DISTRO" == "amzn" ]]; then
                packages="$packages python3.11-devel"
            fi
            ;;
        zypper)
            packages="python3 python3-devel python3-pip python3-venv gcc libopenssl-devel libffi-devel"
            ;;
        pacman)
            packages="python python-pip base-devel openssl libffi"
            ;;
        apk)
            packages="python3 python3-dev py3-pip python3-venv build-base openssl-dev libffi-dev"
            ;;
        *)
            echo "❌ Python: Unsupported package manager for Python packages: $DETECTED_PACKAGE_MANAGER"
            return 1
            ;;
    esac
    
    echo "$packages"
}

# Install packages using detected package manager
install_packages() {
    local packages="$1"
    echo "📦 Installing packages: $packages"
    eval "$DETECTED_INSTALL_CMD $packages"
}

# Remove packages using detected package manager
remove_packages() {
    local packages="$1"
    echo "🧹 Removing packages: $packages"
    eval "$DETECTED_REMOVE_CMD $packages"
}

# Download file with best available tool
download_file() {
    local url="$1"
    local output_file="$2"
    
    # Check if we have any download tools available
    local has_wget=false
    local has_curl=false
    local need_cleanup=false
    
    if command -v wget >/dev/null 2>&1; then
        has_wget=true
    fi
    
    if command -v curl >/dev/null 2>&1; then
        has_curl=true
    fi
    
    # If no download tool available, install one temporarily
    if [[ "$has_wget" == false && "$has_curl" == false ]]; then
        case "$DETECTED_PACKAGE_MANAGER" in
            apt)
                install_packages "wget"
                has_wget=true
                need_cleanup=true
                export JOBLET_CLEANUP_WGET=true
                ;;
            *)
                install_packages "curl"
                has_curl=true
                need_cleanup=true
                export JOBLET_CLEANUP_CURL=true
                ;;
        esac
    fi
    
    # Use available download tool
    if [[ "$has_wget" == true ]]; then
        wget -q --show-progress "$url" -O "$output_file"
    elif [[ "$has_curl" == true ]]; then
        curl -L --progress-bar "$url" -o "$output_file"
    else
        echo "❌ No download tool available"
        return 1
    fi
}

# Main detection function - call this first
detect_system() {
    echo "🔍 Detecting system configuration..."
    echo "=================================="
    
    detect_architecture || return 1
    detect_os_distribution || return 1
    
    echo ""
    echo "📋 System Detection Summary"
    echo "=========================="
    echo "Architecture: $DETECTED_ARCH"
    echo "OS: $DETECTED_OS"
    echo "Distribution: $DETECTED_DISTRO"
    echo "Package Manager: $DETECTED_PACKAGE_MANAGER"
    echo ""
    
    return 0
}

# Display supported platforms
show_supported_platforms() {
    echo ""
    echo "🌐 Supported Platforms"
    echo "====================="
    echo ""
    echo "CPU Architectures:"
    echo "  ✅ x86_64/amd64 (Intel/AMD 64-bit)"
    echo "  ✅ aarch64/arm64 (ARM 64-bit)"
    echo "  ⚠️  armv7l/armhf (ARM 32-bit - limited Java support)"
    echo ""
    echo "Linux Distributions:"
    echo "  ✅ Ubuntu/Debian (apt)"
    echo "  ✅ CentOS/RHEL/Rocky/AlmaLinux/Amazon Linux (yum)"
    echo "  ✅ Fedora (dnf)"
    echo "  ✅ openSUSE/SLES (zypper)"
    echo "  ✅ Arch/Manjaro (pacman)"
    echo "  ✅ Alpine (apk)"
    echo ""
    echo "Runtime Support Matrix:"
    echo "  ✅ Java 17/21: amd64, arm64"
    echo "  ✅ Python 3.11: amd64, arm64, armhf"
    echo ""
}

# Export functions for use by setup scripts
export -f detect_architecture
export -f detect_os_distribution
export -f get_java_download_url
export -f get_python_packages
export -f install_packages
export -f remove_packages
export -f download_file
export -f detect_system
export -f show_supported_platforms
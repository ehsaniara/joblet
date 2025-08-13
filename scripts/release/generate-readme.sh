#!/bin/bash
# Generate README.md for release package

PLATFORM="$1"
GOOS="$2"
GOARCH="$3"
OUTPUT_FILE="$4"

cat > "$OUTPUT_FILE" << EOF
# Joblet RNX CLI - $PLATFORM

## Installation

### Linux
\`\`\`bash
tar -xzf rnx-$PLATFORM.tar.gz
cd rnx-$PLATFORM
./install.sh [install-directory]
\`\`\`

### macOS
\`\`\`bash
tar -xzf rnx-$PLATFORM.tar.gz
cd rnx-$PLATFORM
./install.sh [install-directory]
\`\`\`

### Windows
\`\`\`cmd
tar -xzf rnx-$PLATFORM.tar.gz
cd rnx-$PLATFORM
install.bat [install-directory]
\`\`\`

## Platform Support

- **RNX CLI**: Works on all platforms
- **Joblet Server**: Linux only
- **Admin UI**: Requires Linux server with Node.js

## Architecture: $GOARCH
## OS: $GOOS

For full server installation, use the Linux release.
EOF
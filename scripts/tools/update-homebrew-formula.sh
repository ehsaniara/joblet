#!/bin/bash
set -e

# Script to update the Homebrew formula after a new release
# Usage: ./scripts/update-homebrew-formula.sh v1.0.0

VERSION="${1}"
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

echo "🍺 Updating Homebrew formula for version $VERSION"

# URLs for the new version
AMD64_URL="https://github.com/ehsaniara/joblet/releases/download/${VERSION}/rnx-${VERSION}-darwin-amd64.tar.gz"
ARM64_URL="https://github.com/ehsaniara/joblet/releases/download/${VERSION}/rnx-${VERSION}-darwin-arm64.tar.gz"

echo "📦 Downloading archives to calculate checksums..."

# Download and calculate checksums
curl -sL "$AMD64_URL" -o /tmp/rnx-amd64.tar.gz
curl -sL "$ARM64_URL" -o /tmp/rnx-arm64.tar.gz

AMD64_SHA=$(shasum -a 256 /tmp/rnx-amd64.tar.gz | cut -d' ' -f1)
ARM64_SHA=$(shasum -a 256 /tmp/rnx-arm64.tar.gz | cut -d' ' -f1)

echo "✅ Checksums calculated:"
echo "   AMD64: $AMD64_SHA"
echo "   ARM64: $ARM64_SHA"

# Update the formula
FORMULA_FILE="Formula/rnx.rb"

# Backup the current formula
cp "$FORMULA_FILE" "$FORMULA_FILE.backup"

# Update URLs and checksums
sed -i.bak "s|url \"https://github.com/ehsaniara/joblet/releases/download/[^\"]*darwin-amd64.tar.gz\"|url \"$AMD64_URL\"|" "$FORMULA_FILE"
sed -i.bak "s|url \"https://github.com/ehsaniara/joblet/releases/download/[^\"]*darwin-arm64.tar.gz\"|url \"$ARM64_URL\"|" "$FORMULA_FILE"

# Update SHA256 checksums (first occurrence for amd64, second for arm64)
awk -v amd64="$AMD64_SHA" -v arm64="$ARM64_SHA" '
    /sha256/ && !done_amd64 { 
        sub(/sha256 "[^"]*"/, "sha256 \"" amd64 "\""); 
        done_amd64=1 
    } 
    /sha256/ && done_amd64 && !done_arm64 { 
        sub(/sha256 "[^"]*"/, "sha256 \"" arm64 "\""); 
        done_arm64=1 
    } 
    { print }
' "$FORMULA_FILE.backup" > "$FORMULA_FILE"

# Clean up
rm -f "$FORMULA_FILE.bak" "$FORMULA_FILE.backup"
rm -f /tmp/rnx-amd64.tar.gz /tmp/rnx-arm64.tar.gz

echo "✅ Formula updated successfully!"
echo ""
echo "📋 Next steps:"
echo "1. Test the formula: brew install --build-from-source Formula/rnx.rb"
echo "2. Commit the changes: git add Formula/rnx.rb && git commit -m 'Update Homebrew formula to $VERSION'"
echo "3. Push to main: git push origin main"
echo ""
echo "Users can then update with:"
echo "  brew tap ehsaniara/joblet https://github.com/ehsaniara/joblet"
echo "  brew upgrade rnx"
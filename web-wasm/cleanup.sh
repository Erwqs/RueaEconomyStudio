#!/bin/bash

# Cleanup script for Ruea Economy Studio
# This script helps free up disk space for WASM builds

echo "Ruea Economy Studio - Disk Space Cleanup"
echo "========================================"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Check current disk usage
echo -e "${BLUE}Current disk usage:${NC}"
df -h / | tail -1

echo ""
echo -e "${YELLOW}Files that can be safely removed:${NC}"

# Find large files that can be cleaned
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Test files and debug binaries
find "$PROJECT_ROOT" -name "*.lz4" -size +1M -exec ls -lh {} \; | head -10
find "$PROJECT_ROOT" -name "__debug_bin*" -exec ls -lh {} \; | head -5
find "$PROJECT_ROOT" -name "*.log" -size +1M -exec ls -lh {} \; | head -5

echo ""
echo -e "${YELLOW}Temporary and build files:${NC}"
find "$PROJECT_ROOT" -name "*.tmp" -o -name "*.temp" 2>/dev/null | head -5
find "$PROJECT_ROOT" -path "*/build/*" -name "*.o" 2>/dev/null | head -5

echo ""
echo -e "${BLUE}Recommendations:${NC}"
echo "1. Remove test .lz4 files (they appear to be save files)"
echo "2. Clean osxcross build directory if not needed"
echo "3. Remove debug binaries (__debug_bin*)"
echo "4. Consider moving large files to external storage"

echo ""
echo -e "${RED}WARNING: Only remove files you're sure you don't need!${NC}"
echo ""
echo -e "${YELLOW}Quick cleanup commands:${NC}"
echo "# Remove debug binaries:"
echo "find '$PROJECT_ROOT' -name '__debug_bin*' -delete"
echo ""
echo "# Remove test LZ4 files (backup first!):"
echo "find '$PROJECT_ROOT' -name '*.lz4' -size +10M -delete"
echo ""
echo "# Clean osxcross build (if not needed):"
echo "rm -rf '$PROJECT_ROOT/osxcross/build'"

echo ""
echo -e "${GREEN}After cleanup, retry the WASM build with:${NC}"
echo "cd '$PROJECT_ROOT/web-wasm' && bash build.sh"
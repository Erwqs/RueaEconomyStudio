#!/bin/bash

# Build script for Ruea Economy Studio Web Version
# This script builds the WASM binary and prepares the web deployment

set -e

echo "Building Ruea Economy Studio for WebAssembly..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project directories
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB_DIR="$PROJECT_ROOT/web-wasm"
OUTPUT_DIR="$PROJECT_ROOT/web-wasm"

echo -e "${BLUE}Project root: $PROJECT_ROOT${NC}"
echo -e "${BLUE}Web directory: $WEB_DIR${NC}"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}Error: Go is not installed or not in PATH${NC}"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | cut -d' ' -f3 | cut -d'o' -f2)
echo -e "${BLUE}Go version: $GO_VERSION${NC}"

# Set WASM environment variables
export GOOS=js
export GOARCH=wasm

echo -e "${YELLOW}Building WASM binary...${NC}"

# Change to web-wasm directory and build
cd "$WEB_DIR"

# Build the WASM binary
go build -o ruea.wasm ./main.go

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ WASM binary built successfully${NC}"
else
    echo -e "${RED}✗ Failed to build WASM binary${NC}"
    exit 1
fi

# Copy wasm_exec.js from Go installation
WASM_EXEC_JS=$(go env GOROOT)/misc/wasm/wasm_exec.js

if [ -f "$WASM_EXEC_JS" ]; then
    cp "$WASM_EXEC_JS" "$OUTPUT_DIR/"
    echo -e "${GREEN}✓ Copied wasm_exec.js${NC}"
else
    echo -e "${RED}✗ Could not find wasm_exec.js in Go installation${NC}"
    echo -e "${YELLOW}You may need to download it manually from: https://github.com/golang/go/blob/master/misc/wasm/wasm_exec.js${NC}"
fi

# Check file sizes
echo -e "${BLUE}File sizes:${NC}"
ls -lh "$OUTPUT_DIR/ruea.wasm" "$OUTPUT_DIR/index.html" "$OUTPUT_DIR/wasm_exec.js" 2>/dev/null || true

# Create a simple server script
cat > "$OUTPUT_DIR/serve.py" << 'EOF'
#!/usr/bin/env python3
"""
Simple HTTP server for serving WebAssembly files.
This server adds the necessary MIME types for WASM files.
"""

import http.server
import socketserver
import mimetypes
import os

# Add WASM MIME type
mimetypes.add_type('application/wasm', '.wasm')

PORT = 8000

class WASMHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        # Add headers for WASM
        if self.path.endswith('.wasm'):
            self.send_header('Content-Type', 'application/wasm')
        # Add CORS headers for development
        self.send_header('Cross-Origin-Embedder-Policy', 'require-corp')
        self.send_header('Cross-Origin-Opener-Policy', 'same-origin')
        super().end_headers()

def main():
    print(f"Starting server at http://localhost:{PORT}")
    print("Press Ctrl+C to stop the server")
    
    with socketserver.TCPServer(("", PORT), WASMHandler) as httpd:
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nServer stopped")

if __name__ == "__main__":
    main()
EOF

chmod +x "$OUTPUT_DIR/serve.py"

echo -e "${GREEN}✓ Created development server script (serve.py)${NC}"

# Create README for web deployment
cat > "$OUTPUT_DIR/README.md" << 'EOF'
# Ruea Economy Studio - Web Version

This directory contains the WebAssembly build of Ruea Economy Studio.

## Files

- `index.html` - Main HTML page
- `ruea.wasm` - WebAssembly binary
- `wasm_exec.js` - Go WebAssembly runtime
- `serve.py` - Development server script

## Running Locally

### Option 1: Python Server (Recommended)
```bash
python3 serve.py
```

### Option 2: Node.js (if available)
```bash
npx serve .
```

### Option 3: Python Simple Server
```bash
python3 -m http.server 8000
```

Then open http://localhost:8000 in your browser.

## Deployment

For production deployment, ensure your web server:

1. Serves `.wasm` files with `application/wasm` MIME type
2. Enables compression (gzip/brotli) for better loading times
3. Sets appropriate CORS headers if needed

## Browser Requirements

- Modern browser with WebAssembly support
- JavaScript enabled
- For best performance: Chrome/Edge/Firefox latest versions

## Known Limitations (Web Version)

- File system dialogs disabled (no native file picker)
- Clipboard operations may be limited
- Performance may be slower than desktop version
- Some native OS integrations unavailable

## Troubleshooting

- If the screen appears 0x0, refresh the page
- If scrolling is too sensitive, it should be automatically dampened
- Check browser console for error messages
- Ensure you're serving over HTTP/HTTPS (not file://)
EOF

echo -e "${GREEN}✓ Build complete!${NC}"
echo -e "${BLUE}Files created in: $OUTPUT_DIR${NC}"
echo -e "${YELLOW}To test locally, run:${NC}"
echo -e "  ${GREEN}cd $OUTPUT_DIR && python3 serve.py${NC}"
echo -e "${YELLOW}Then open: ${GREEN}http://localhost:8000${NC}"
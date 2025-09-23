#!/usr/bin/env python3
"""
Enhanced HTTP server for serving WebAssembly files.
This server properly sets MIME types for WASM files.
"""

import http.server
import socketserver
import mimetypes
import os
from urllib.parse import urlparse

# Ensure WASM MIME type is registered
mimetypes.init()
mimetypes.add_type('application/wasm', '.wasm')

PORT = 8099

class WASMHandler(http.server.SimpleHTTPRequestHandler):
    
    def guess_type(self, path):
        """Override guess_type to ensure .wasm files get correct MIME type"""
        # Use mimetypes module directly instead of super() to avoid unpacking issues
        
        # Force correct MIME type for WASM files
        if path.lower().endswith('.wasm'):
            return 'application/wasm', None
        elif path.lower().endswith('.js'):
            return 'application/javascript', None
        elif path.lower().endswith('.html'):
            return 'text/html', None
        else:
            # Use mimetypes directly for other files
            return mimetypes.guess_type(path)
    
    def end_headers(self):
        # Add CORS and security headers for development
        self.send_header('Cross-Origin-Embedder-Policy', 'require-corp')
        self.send_header('Cross-Origin-Opener-Policy', 'same-origin')
        
        # Ensure no caching during development
        self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
        self.send_header('Pragma', 'no-cache')
        self.send_header('Expires', '0')
        
        super().end_headers()
    
    def do_GET(self):
        """Handle GET requests with proper logging"""
        parsed_path = urlparse(self.path)
        file_path = parsed_path.path
        
        print(f"Request: {file_path}")
        
        # Log MIME type for debugging
        if file_path.endswith('.wasm'):
            print(f"  -> Serving WASM file with MIME type: application/wasm")
        elif file_path.endswith('.js'):
            print(f"  -> Serving JS file")
        
        return super().do_GET()

def main():
    print("=" * 50)
    print("Ruea Economy Studio - WebAssembly Development Server")
    print("=" * 50)
    print(f"Server starting at: http://localhost:{PORT}")
    print(f"Serving directory: {os.getcwd()}")
    print()
    
    # Check for required files
    required_files = ['index.html', 'ruea.wasm', 'wasm_exec.js']
    missing_files = []
    
    for file in required_files:
        if os.path.exists(file):
            size = os.path.getsize(file)
            size_mb = size / (1024 * 1024)
            print(f"✓ {file} ({size_mb:.2f} MB)")
        else:
            missing_files.append(file)
            print(f"✗ {file} - MISSING!")
    
    if missing_files:
        print(f"\nERROR: Missing required files: {missing_files}")
        print("Please run the build script first: ./build.sh")
        return
    
    print(f"\n🚀 Open http://localhost:{PORT} in your browser")
    print("📋 Check browser console for any error messages")
    print("🛑 Press Ctrl+C to stop the server\n")
    
    try:
        with socketserver.TCPServer(("", PORT), WASMHandler) as httpd:
            httpd.serve_forever()
    except KeyboardInterrupt:
        print("\n\n🛑 Server stopped by user")
    except Exception as e:
        print(f"\n❌ Server error: {e}")

if __name__ == "__main__":
    main()
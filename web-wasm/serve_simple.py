#!/usr/bin/env python3
"""
Ultra-simple HTTP server for WASM files with explicit MIME type handling.
"""

import http.server
import socketserver
import os
from urllib.parse import urlparse

PORT = 8099

class WASMHandler(http.server.SimpleHTTPRequestHandler):
    
    def end_headers(self):
        # Set CORS headers for development
        self.send_header('Cross-Origin-Embedder-Policy', 'require-corp')
        self.send_header('Cross-Origin-Opener-Policy', 'same-origin')
        
        # Disable caching for development
        self.send_header('Cache-Control', 'no-cache, no-store, must-revalidate')
        self.send_header('Pragma', 'no-cache')
        self.send_header('Expires', '0')
        
        super().end_headers()
    
    def do_GET(self):
        """Handle GET requests with explicit MIME type setting"""
        path = self.path
        if path == '/':
            path = '/index.html'
        
        # Remove leading slash for file operations
        file_path = path.lstrip('/')
        
        print(f"Request: {path}")
        
        try:
            if os.path.exists(file_path):
                # Set content type based on file extension
                if file_path.endswith('.wasm'):
                    content_type = 'application/wasm'
                    print(f"  -> Serving WASM file with MIME type: {content_type}")
                elif file_path.endswith('.js'):
                    content_type = 'application/javascript'
                elif file_path.endswith('.html'):
                    content_type = 'text/html; charset=utf-8'
                elif file_path.endswith('.css'):
                    content_type = 'text/css'
                else:
                    content_type = 'application/octet-stream'
                
                # Send response
                self.send_response(200)
                self.send_header('Content-Type', content_type)
                
                # Get file size
                file_size = os.path.getsize(file_path)
                self.send_header('Content-Length', str(file_size))
                
                self.end_headers()
                
                # Send file content
                with open(file_path, 'rb') as f:
                    self.wfile.write(f.read())
                    
                print(f"  -> Sent {file_size} bytes as {content_type}")
                
            else:
                print(f"  -> File not found: {file_path}")
                self.send_error(404, f"File not found: {file_path}")
                
        except Exception as e:
            print(f"  -> Error serving {file_path}: {e}")
            self.send_error(500, f"Server error: {e}")

def main():
    print("=" * 60)
    print("Ruea Economy Studio - WebAssembly Development Server")
    print("=" * 60)
    print(f"Server starting at: http://localhost:{PORT}")
    print(f"Serving directory: {os.getcwd()}")
    print()
    
    # Check for required files
    required_files = ['index.html', 'ruea.wasm', 'wasm_exec.js']
    all_good = True
    
    for file in required_files:
        if os.path.exists(file):
            size = os.path.getsize(file)
            size_mb = size / (1024 * 1024)
            print(f"✓ {file} ({size_mb:.2f} MB)")
        else:
            all_good = False
            print(f"✗ {file} - MISSING!")
    
    if not all_good:
        print(f"\nERROR: Missing required files!")
        print("Please ensure you have built the WASM file and copied wasm_exec.js")
        return
    
    print(f"\n🚀 Open http://localhost:{PORT} in your browser")
    print("📋 Check browser console and this terminal for messages")
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
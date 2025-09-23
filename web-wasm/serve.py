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

PORT = 8099

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

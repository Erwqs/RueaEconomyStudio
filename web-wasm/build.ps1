# Build script for Ruea Economy Studio Web Version (PowerShell)
# This script builds the WASM binary and prepares the web deployment

param(
    [switch]$Verbose
)

Write-Host "Building Ruea Economy Studio for WebAssembly..." -ForegroundColor Blue

# Project directories
$ProjectRoot = Split-Path -Parent $PSScriptRoot
$WebDir = Join-Path $ProjectRoot "web-wasm"
$OutputDir = $WebDir

Write-Host "Project root: $ProjectRoot" -ForegroundColor Cyan
Write-Host "Web directory: $WebDir" -ForegroundColor Cyan

# Check if Go is installed
if (!(Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH" -ForegroundColor Red
    exit 1
}

# Check Go version
$GoVersion = (go version) -split ' ' | Select-Object -Index 2
Write-Host "Go version: $GoVersion" -ForegroundColor Cyan

# Set WASM environment variables
$env:GOOS = "js"
$env:GOARCH = "wasm"

Write-Host "Building WASM binary..." -ForegroundColor Yellow

# Change to web-wasm directory and build
Push-Location $WebDir

try {
    # Build the WASM binary
    go build -o ruea.wasm ./main.go
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "✓ WASM binary built successfully" -ForegroundColor Green
    } else {
        Write-Host "✗ Failed to build WASM binary" -ForegroundColor Red
        exit 1
    }

    # Copy wasm_exec.js from Go installation
    $GoRoot = go env GOROOT
    $WasmExecJs = Join-Path $GoRoot "misc\wasm\wasm_exec.js"

    if (Test-Path $WasmExecJs) {
        Copy-Item $WasmExecJs $OutputDir
        Write-Host "✓ Copied wasm_exec.js" -ForegroundColor Green
    } else {
        Write-Host "✗ Could not find wasm_exec.js in Go installation" -ForegroundColor Red
        Write-Host "You may need to download it manually from: https://github.com/golang/go/blob/master/misc/wasm/wasm_exec.js" -ForegroundColor Yellow
    }

    # Check file sizes
    Write-Host "File sizes:" -ForegroundColor Cyan
    Get-ChildItem "$OutputDir\ruea.wasm", "$OutputDir\index.html", "$OutputDir\wasm_exec.js" -ErrorAction SilentlyContinue | 
        Format-Table Name, @{Name="Size"; Expression={[math]::Round($_.Length/1MB, 2).ToString() + " MB"}} -AutoSize

    # Create a simple server script (PowerShell version)
    $ServerScript = @'
# Simple HTTP server for serving WebAssembly files
# Usage: .\serve.ps1

$Port = 8000
$LocalPath = Get-Location

Write-Host "Starting server at http://localhost:$Port" -ForegroundColor Green
Write-Host "Serving files from: $LocalPath" -ForegroundColor Cyan
Write-Host "Press Ctrl+C to stop the server" -ForegroundColor Yellow

try {
    # Use Python if available, otherwise provide instructions
    if (Get-Command python -ErrorAction SilentlyContinue) {
        python -m http.server $Port
    } elseif (Get-Command python3 -ErrorAction SilentlyContinue) {
        python3 -m http.server $Port
    } else {
        Write-Host "Python not found. Please install Python or use an alternative server:" -ForegroundColor Red
        Write-Host "- Node.js: npx serve ." -ForegroundColor Yellow
        Write-Host "- PHP: php -S localhost:$Port" -ForegroundColor Yellow
        Write-Host "- Any other HTTP server that supports WASM MIME types" -ForegroundColor Yellow
        exit 1
    }
} catch {
    Write-Host "Server stopped" -ForegroundColor Yellow
}
'@

    $ServerScript | Out-File -FilePath (Join-Path $OutputDir "serve.ps1") -Encoding UTF8
    
    Write-Host "✓ Created development server script (serve.ps1)" -ForegroundColor Green

    Write-Host "✓ Build complete!" -ForegroundColor Green
    Write-Host "Files created in: $OutputDir" -ForegroundColor Cyan
    Write-Host "To test locally, run:" -ForegroundColor Yellow
    Write-Host "  cd `"$OutputDir`" && .\serve.ps1" -ForegroundColor Green
    Write-Host "Then open: http://localhost:8000" -ForegroundColor Green

} finally {
    Pop-Location
}
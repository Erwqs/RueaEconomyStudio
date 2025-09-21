#!/usr/bin/env pwsh

# Cross-compile script for macOS ARM64
# This script sets up the environment for cross-compiling Go applications to macOS ARM64

Write-Host "Setting up cross-compilation environment for macOS ARM64..."

# Add osxcross tools to PATH
$env:PATH = "/home/aqts/etools/osxcross/target/bin:$($env:PATH)"

# Set Go cross-compilation environment variables
$env:GOOS = "darwin"
$env:GOARCH = "arm64" 
$env:CGO_ENABLED = "1"
$env:CC = "aarch64-apple-darwin20.4-clang"
$env:CXX = "aarch64-apple-darwin20.4-clang++"

Write-Host "Environment variables set:"
Write-Host "GOOS=$($env:GOOS)"
Write-Host "GOARCH=$($env:GOARCH)"
Write-Host "CGO_ENABLED=$($env:CGO_ENABLED)"
Write-Host "CC=$($env:CC)"
Write-Host "CXX=$($env:CXX)"

# Verify compiler is available
if (Get-Command $env:CC -ErrorAction SilentlyContinue) {
    Write-Host "✓ Cross-compiler found: $($env:CC)"
} else {
    Write-Host "✗ Cross-compiler not found: $($env:CC)"
    Write-Host "Make sure osxcross is properly set up in /home/aqts/etools/osxcross/"
    exit 1
}

Write-Host ""
Write-Host "Building RueaES for macOS ARM64..."

# Build the application
$outputFile = "RueaES-MacOS-arm64"
go build -o $outputFile main.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "✓ Build successful!"
    Write-Host "Output file: $outputFile"
    
    # Show file info
    $fileInfo = Get-ChildItem $outputFile -ErrorAction SilentlyContinue
    if ($fileInfo) {
        Write-Host "File size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB"
    }
    
    # Check architecture using file command if available
    $fileOutput = file $outputFile 2>$null
    if ($fileOutput) {
        Write-Host "Architecture: $fileOutput"
    }
} else {
    Write-Host "✗ Build failed!"
    exit 1
}

Write-Host ""
Write-Host "Cross-compilation complete!"
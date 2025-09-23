//go:build !js || !wasm
// +build !js !wasm

package app

import (
	"fmt"
	"os"

	"github.com/atotto/clipboard"
)

// WebSaveFile saves binary data using native file operations (fallback for non-web)
func WebSaveFile(filename string, data []byte) error {
	// Ensure filename has .lz4 extension for consistency
	if len(filename) < 4 || filename[len(filename)-4:] != ".lz4" {
		filename += ".lz4"
	}
	return os.WriteFile(filename, data, 0644)
}

// WebLoadFile loads binary data using native file operations (fallback for non-web)
func WebLoadFile(acceptTypes ...string) ([]byte, string, error) {
	// This is a fallback implementation for non-web builds
	// In practice, non-web builds should use the file system manager
	return nil, "", fmt.Errorf("WebLoadFile not implemented for non-web builds")
}

// WebWriteClipboard writes text to clipboard using native clipboard library
func WebWriteClipboard(text string) error {
	return clipboard.WriteAll(text)
}

// WebReadClipboard reads text from clipboard using native clipboard library
func WebReadClipboard() (string, error) {
	return clipboard.ReadAll()
}

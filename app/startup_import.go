package app

import (
	"os"
)

// startupImportPath holds an optional state file path supplied via CLI/OS association.
var startupImportPath string

// SetStartupImportPath records the file to import when the UI is ready.
func SetStartupImportPath(path string) {
	startupImportPath = path
}

// popStartupImportPath returns the pending path once and clears it.
func popStartupImportPath() string {
	path := startupImportPath
	startupImportPath = ""
	return path
}

// triggerStartupImportIfPresent shows the import modal for the pending path, if any.
func triggerStartupImportIfPresent() {
	path := popStartupImportPath()
	if path == "" {
		return
	}

	if _, err := os.Stat(path); err != nil {
		return
	}

	ShowStateImportModal(path)
}

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	dataDirOnce sync.Once
	dataDirPath string
)

// DataDir returns the platform-appropriate writable data directory and creates it if missing.
func DataDir() string {
	dataDirOnce.Do(func() {
		dataDirPath = resolveDataDir()
		_ = os.MkdirAll(dataDirPath, 0o755)
	})
	return dataDirPath
}

// DataFile joins the data directory with the provided relative name.
func DataFile(name string) string {
	return filepath.Join(DataDir(), name)
}

// ReadDataFile reads a file from the data directory, migrating from legacy working-directory files if present.
func ReadDataFile(name string) ([]byte, error) {
	primary := DataFile(name)
	data, err := os.ReadFile(primary)
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		return data, err
	}

	// Legacy fallback: check current working directory for old file name
	legacyData, legacyErr := os.ReadFile(name)
	if legacyErr != nil {
		return data, err // return original not-exist error
	}

	// Persist legacy data into the new location for future use
	_ = os.MkdirAll(filepath.Dir(primary), 0o755)
	_ = os.WriteFile(primary, legacyData, 0o644)
	return legacyData, nil
}

// WriteDataFile writes data to the data directory, ensuring the directory exists.
func WriteDataFile(name string, data []byte, perm os.FileMode) error {
	path := DataFile(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, perm)
}

func resolveDataDir() string {
	if custom := os.Getenv("RUEAES_DATA_DIR"); custom != "" {
		return custom
	}

	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("APPDATA"); base != "" {
			return filepath.Join(base, "RueaEconomyStudio")
		}
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "RueaEconomyStudio")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "RueaEconomyStudio")
		}
	default: // Linux and others
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			return filepath.Join(xdg, "RueaEconomyStudio")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "RueaEconomyStudio")
		}
	}

	// Final fallback: use current directory
	return "./RueaEconomyStudio"
}

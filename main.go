package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"RueaES/api"
	"RueaES/app"
	"RueaES/eruntime"
	_ "RueaES/eruntime"
	"RueaES/storage"

	"net/http"
	_ "net/http/pprof"

	// hideconsole
	_ "github.com/ebitengine/hideconsole"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.design/x/clipboard"
)

func init() {
	go func() {
		http.ListenAndServe(":6060", nil)
	}()
}

func main() {
	// Parse command line flags
	var headless bool
	var stateFilePath string
	flag.BoolVar(&headless, "headless", false, "Run in headless mode without GUI")
	flag.BoolVar(&headless, "h", false, "Run in headless mode without GUI (shorthand)")
	flag.StringVar(&stateFilePath, "file", "", "State file (.lz4/.ruea) to import on launch")
	flag.StringVar(&stateFilePath, "f", "", "State file (.lz4/.ruea) to import on launch (shorthand)")
	flag.Parse()

	// Support positional file argument so double-clicking a .lz4 passes the path through
	if stateFilePath == "" {
		if args := flag.Args(); len(args) > 0 {
			stateFilePath = args[0]
		}
	}

	if stateFilePath != "" {
		cleanPath := filepath.Clean(stateFilePath)
		if _, err := os.Stat(cleanPath); err != nil {
		} else {
			app.SetStartupImportPath(cleanPath)
			os.Setenv("RUEAES_SKIP_AUTOSAVE_LOAD", "1")
		}
	}

	lockPath := storage.DataFile(".rueaes.lock")
	lockFile, lockOwned, cleanupLock, err := prepareLock(lockPath)
	if err != nil {
		os.Exit(1)
	}

	_ = lockFile // retained to keep handle open for lifetime
	defer cleanupLock()

	if headless {
		if !lockOwned {
			os.Exit(1)
		}
		// Run in headless mode with shared memory server
		runHeadless()
		return
	}

	if !lockOwned {
		app.ScheduleLockWarning(lockPath,
			func() {
				// User chose to continue; close our handle and remove the existing lock file.
				cleanupLock()
				_ = os.Remove(lockPath)
			},
			func() {
				cleanupLock()
				os.Exit(0)
			},
		)
	}

	// Run with GUI
	runWithGUI(lockOwned, cleanupLock)
}

func prepareLock(lockPath string) (*os.File, bool, func(), error) {
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	owned := true
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			owned = false
			lockFile, err = os.OpenFile(lockPath, os.O_WRONLY, 0o644)
			if err != nil {
				return nil, false, nil, err
			}
		} else {
			return nil, false, nil, err
		}
	}

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			if lockFile != nil {
				_ = lockFile.Close()
			}
			if owned {
				os.Remove(lockPath)
			}
		})
	}

	return lockFile, owned, cleanup, nil
}

func runHeadless() {
	fmt.Println("Starting Wynncraft RueaES in headless mode...")

	// Start WebSocket API server
	go func() {
		fmt.Println("Starting WebSocket API server on port 42069...")
		api.StartWebSocketServer()
	}()

	// Initialize shared memory as server
	// if err := eruntime.InitializeSharedMemory(true); err != nil {
	// 	log.Fatalf("Failed to initialize shared memory server: %v", err)
	// }

	fmt.Println("WebSocket API is available at ws://localhost:42069/ws")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	eruntime.TriggerAutoSave()
	fmt.Println("Received shutdown signal. Cleaning up...")

	fmt.Println("Shutdown complete.")
}

func runWithGUI(lockOwned bool, cleanup func()) {
	if !lockOwned {
		fmt.Println("Lock file already existed; showing warning modal before continuing.")
	}
	// Start WebSocket API server for GUI mode as well
	go func() {
		fmt.Println("Starting WebSocket API server on port 42069...")
		api.StartWebSocketServer()
	}()
	fmt.Println("WebSocket API is available at ws://localhost:42069/ws")

	// Initialize clipboard
	// Disable clipboard initialization on WebAssembly (wasm)

	// Clipboard is only initialized on supported platforms
	if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
		initClipboard()
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		fmt.Println("Received shutdown signal. Cleaning up...")
		// trigger autosave
		eruntime.TriggerAutoSave()
		if cleanup != nil {
			cleanup()
		}
		os.Exit(0)
	}()

	// Initialize panic notification system
	app.InitPanicNotifier()
	app.InitToastManager()
	app.InitPauseNotificationManager()

	ebiten.SetWindowTitle("Ruea Economy Studio")
	ebiten.SetTPS(ebiten.SyncWithFPS) // Restore normal TPS since the issue is graphics, not game logic
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(true)
	ebiten.SetWindowSize(1600, 900)

	ebiten.SetWindowSize(1600, 900)

	ebiten.SetWindowPosition(0, 0)

	game := app.New()
	app.SetCurrentApp(game)
	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		X11ClassName:    "Ruea Economy Studio",
		X11InstanceName: "RueaES",
		SingleThread: false,
		ScreenTransparent: true,
	}); err != nil {
		panic(err)
	}
}

func initClipboard() error {
	return clipboard.Init()
}

package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"RueaES/api"
	"RueaES/app"
	_ "RueaES/eruntime"

	"net/http"
	_ "net/http/pprof"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.design/x/clipboard"
)

func init() {
	go func() {
		// fmt.Println("pprof listening on :6060")
		http.ListenAndServe(":6060", nil)
	}()
}

func main() {
	// Parse command line flags
	var headless bool
	flag.BoolVar(&headless, "headless", false, "Run in headless mode without GUI")
	flag.BoolVar(&headless, "h", false, "Run in headless mode without GUI (shorthand)")
	flag.Parse()

	// Check for session lock, if exists, exit
	if _, err := os.Stat(".rueaes.lock"); err == nil {
		// fmt.Println("Another instance of Ruea Economy Studio is already running.")
		// fmt.Println("If none is running, remove .rueaes.lock file to continue.")
		// fmt.Println("Exiting...")
		os.Exit(1)
	}

	// Create lock file
	lockFile, err := os.Create(".rueaes.lock")
	if err != nil {
		// fmt.Printf("Failed to create lock file: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := lockFile.Close(); err != nil {
			// fmt.Printf("Failed to close lock file: %v\n", err)
		}
		if err := os.Remove(".rueaes.lock"); err != nil {
			// fmt.Printf("Failed to remove lock file: %v\n", err)
		}
	}()

	if headless {
		// Run in headless mode with shared memory server
		runHeadless()
	} else {
		// Run with GUI
		runWithGUI()
	}
}

func runHeadless() {
	// fmt.Println("Starting Wynncraft RueaES in headless mode...")

	// Start WebSocket API server
	go func() {
		// fmt.Println("Starting WebSocket API server on port 42069...")
		api.StartWebSocketServer()
	}()

	// Initialize shared memory as server
	// if err := eruntime.InitializeSharedMemory(true); err != nil {
	// 	log.Fatalf("Failed to initialize shared memory server: %v", err)
	// }

	// fmt.Println("Shared memory server started. RueaES is ready for external connections.")
	// fmt.Println("WebSocket API is available at ws://localhost:42069/ws")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	// fmt.Println("Received shutdown signal. Cleaning up...")

	// Note: WebSocket server shutdown is handled automatically by the HTTP server
	// when the main goroutine exits

	// Clean up shared memory
	// if api, err := eruntime.GetSharedMemoryAPI(); err == nil {
	// 	api.Close()
	// }

	// fmt.Println("Shutdown complete.")
}

func runWithGUI() {
	// Start WebSocket API server for GUI mode as well
	go func() {
		// fmt.Println("Starting WebSocket API server on port 42069...")
		api.StartWebSocketServer()
	}()
	// fmt.Println("WebSocket API is available at ws://localhost:42069/ws")

	// Initialize clipboard
	// Disable clipboard initialization on WebAssembly (wasm)

	// Clipboard is only initialized on supported platforms
	if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
		err := initClipboard()
		if err != nil {
			// Clipboard initialization failed, but we can continue without it
			// The clipboard operations will simply not work
			// fmt.Printf("Warning: Clipboard initialization failed: %v\n", err)
		}
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-signalChan
		// fmt.Println("Received shutdown signal. Cleaning up...")
		// Delete lock file if exists
		if _, err := os.Stat(".rueaes.lock"); err == nil {
			if err := os.Remove(".rueaes.lock"); err != nil {
				// fmt.Printf("Failed to remove lock file: %v\n", err)
			} else {
				// fmt.Println("Lock file removed successfully.")
			}
		} else {
			// fmt.Println("No lock file found, nothing to remove.")
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
	}); err != nil {
		panic(err)
	}
}

func initClipboard() error {
	return clipboard.Init()
}

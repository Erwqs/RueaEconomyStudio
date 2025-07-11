package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"etools/app"
	_ "etools/eruntime"

	// _ "net/http/pprof"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.design/x/clipboard"
)

// func init() {
// 	go func() {
// 		fmt.Println("pprof listening on :6060")
// 		http.ListenAndServe(":6060", nil)
// 	}()
// }

func main() {
	// Parse command line flags
	var headless bool
	flag.BoolVar(&headless, "headless", false, "Run in headless mode without GUI")
	flag.BoolVar(&headless, "h", false, "Run in headless mode without GUI (shorthand)")
	flag.Parse()

	if headless {
		// Run in headless mode with shared memory server
		runHeadless()
	} else {
		// Run with GUI
		runWithGUI()
	}
}

func runHeadless() {
	fmt.Println("Starting Wynncraft ETools in headless mode...")

	// Initialize shared memory as server
	// if err := eruntime.InitializeSharedMemory(true); err != nil {
	// 	log.Fatalf("Failed to initialize shared memory server: %v", err)
	// }

	fmt.Println("Shared memory server started. ETools is ready for external connections.")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down...")

	// Clean up shared memory
	// if api, err := eruntime.GetSharedMemoryAPI(); err == nil {
	// 	api.Close()
	// }

	fmt.Println("Shutdown complete.")
}

func runWithGUI() {
	// Initialize clipboard
	// Disable clipboard initialization on WebAssembly (wasm)

	// Clipboard is only initialized on supported platforms
	if runtime.GOARCH != "wasm" && runtime.GOOS != "js" {
		err := initClipboard()
		if err != nil {
			// Clipboard initialization failed, but we can continue without it
			// The clipboard operations will simply not work
			fmt.Printf("Warning: Clipboard initialization failed: %v\n", err)
		}
	}

	// Initialize panic notification system
	app.InitPanicNotifier()
	app.InitToastManager()

	ebiten.SetWindowTitle("Ruea Economy Studio")
	ebiten.SetTPS(ebiten.SyncWithFPS) // Restore normal TPS since the issue is graphics, not game logic
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowDecorated(true)
	ebiten.SetWindowSize(1600, 900)

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


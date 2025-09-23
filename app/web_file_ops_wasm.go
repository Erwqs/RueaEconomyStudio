//go:build js && wasm
// +build js,wasm

package app

import (
	"fmt"
	"syscall/js"
)

// WebSaveFile saves data to a file using the browser's download functionality
func WebSaveFile(filename string, data []byte) error {
	// Ensure filename has .lz4 extension for compatibility with desktop version
	if len(filename) < 4 || filename[len(filename)-4:] != ".lz4" {
		filename += ".lz4"
	}

	// Data is already compressed by eruntime.SaveStateToBytes(), don't compress again
	// fmt.Printf("[WEB_FILE] Saving %s with %d bytes (already LZ4 compressed)\n", filename, len(data))

	// Convert to JavaScript Uint8Array
	jsArray := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsArray, data)

	// Call JavaScript function to trigger download
	js.Global().Call("downloadFile", filename, jsArray)

	return nil
}

// WebLoadFile loads a file using the browser's file picker
func WebLoadFile(acceptTypes ...string) ([]byte, string, error) {
	// Default to .lz4 files if no extension specified
	fileTypes := ".lz4"
	if len(acceptTypes) > 0 {
		fileTypes = acceptTypes[0]
	}

	// Create a channel to wait for the file result
	resultChan := make(chan fileResult)

	// Create a callback function that will be called from JavaScript
	callback := js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- fileResult{nil, "", fmt.Errorf("callback panic: %v", r)}
			}
		}()

		if len(args) < 3 {
			resultChan <- fileResult{nil, "", fmt.Errorf("invalid callback arguments")}
			return nil
		}

		// Get the result from JavaScript
		success := args[0].Bool()
		filename := args[1].String()

		if !success {
			resultChan <- fileResult{nil, "", fmt.Errorf("file picker cancelled or failed")}
			return nil
		}

		// Get the file data as Uint8Array
		jsData := args[2]
		dataLength := jsData.Get("length").Int()
		data := make([]byte, dataLength)
		js.CopyBytesToGo(data, jsData)

		// Data is already LZ4 compressed, pass it directly to eruntime.LoadStateFromBytes()
		// which will handle the decompression
		// fmt.Printf("[WEB_FILE] Loaded %s with %d bytes (LZ4 compressed)\n", filename, len(data))

		resultChan <- fileResult{data, filename, nil}
		return nil
	})

	defer callback.Release()

	// Call JavaScript file picker with specified file types filter
	js.Global().Call("pickFile", fileTypes, callback)

	// Wait for result
	result := <-resultChan
	return result.data, result.filename, result.err
}

// WebWriteClipboard writes text to clipboard using browser API
func WebWriteClipboard(text string) error {
	promise := js.Global().Call("writeClipboard", text)

	// Wait for the promise to resolve
	resultChan := make(chan bool, 1)

	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			resultChan <- args[0].Bool()
		} else {
			resultChan <- false
		}
		return nil
	}))

	success := <-resultChan
	if !success {
		return fmt.Errorf("failed to write to clipboard")
	}

	return nil
}

// WebReadClipboard reads text from clipboard using browser API
func WebReadClipboard() (string, error) {
	promise := js.Global().Call("readClipboard")

	// Wait for the promise to resolve
	resultChan := make(chan string, 1)

	promise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) > 0 {
			resultChan <- args[0].String()
		} else {
			resultChan <- ""
		}
		return nil
	}))

	result := <-resultChan
	return result, nil
}

// Helper struct for file operation results
type fileResult struct {
	data     []byte
	filename string
	err      error
}

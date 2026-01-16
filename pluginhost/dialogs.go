//go:build cgo
// +build cgo

package pluginhost

/*
#include <stdlib.h>
#include "../sdk/RueaES-SDK.h"

static inline void ruea_call_file_accept(ruea_file_dialog_accept_cb cb, const char* path, void* user_data) {
	if (cb) cb(path, user_data);
}

static inline void ruea_call_file_cancel(ruea_file_dialog_cancel_cb cb, void* user_data) {
	if (cb) cb(user_data);
}

static inline void ruea_call_color_accept(ruea_color_picker_accept_cb cb, uint8_t r, uint8_t g, uint8_t b, void* user_data) {
	if (cb) cb(r, g, b, user_data);
}

static inline void ruea_call_color_cancel(ruea_color_picker_cancel_cb cb, void* user_data) {
	if (cb) cb(user_data);
}

static inline void ruea_call_territory_accept(ruea_territory_selector_accept_cb cb, const char* const* names, size_t count, void* user_data) {
	if (cb) cb(names, count, user_data);
}

static inline void ruea_call_territory_cancel(ruea_territory_selector_cancel_cb cb, void* user_data) {
	if (cb) cb(user_data);
}
*/
import "C"

import (
	"image/color"
	"strings"
	"sync"
	"unsafe"
)

// FileDialogMode mirrors the C enum for file dialogue types.
type FileDialogMode int

const (
	FileDialogOpen FileDialogMode = iota
	FileDialogSave
)

// FileDialogSpec carries the information needed for the app layer to render a file chooser.
type FileDialogSpec struct {
	PluginID    string
	Title       string
	DefaultPath string
	Filters     []string
	Mode        FileDialogMode
}

type fileDialogRequest struct {
	spec     FileDialogSpec
	onAccept C.ruea_file_dialog_accept_cb
	onCancel C.ruea_file_dialog_cancel_cb
	userData unsafe.Pointer
}

type ColorPickerSpec struct {
	PluginID string
	Initial  color.RGBA
}

type colorPickerRequest struct {
	spec     ColorPickerSpec
	onAccept C.ruea_color_picker_accept_cb
	onCancel C.ruea_color_picker_cancel_cb
	userData unsafe.Pointer
}

// TerritorySelectorSpec carries parameters for the host-driven territory selector.
type TerritorySelectorSpec struct {
	PluginID    string
	Title       string
	Preselect   []string
	MultiSelect bool
}

type territorySelectorRequest struct {
	spec     TerritorySelectorSpec
	onAccept C.ruea_territory_selector_accept_cb
	onCancel C.ruea_territory_selector_cancel_cb
	userData unsafe.Pointer
}

var (
	fileDialogMu      sync.Mutex
	fileDialogHandler func(FileDialogSpec)
	activeFileDialogs = make(map[string]fileDialogRequest)

	colorPickerMu      sync.Mutex
	colorPickerHandler func(ColorPickerSpec)
	activeColorPickers = make(map[string]colorPickerRequest)

	territorySelectorMu      sync.Mutex
	territorySelectorHandler func(TerritorySelectorSpec)
	activeTerritorySelectors = make(map[string]territorySelectorRequest)
)

// RegisterFileDialogHandler allows the app to provide a renderer for plugin file dialogs.
func RegisterFileDialogHandler(openFn func(FileDialogSpec)) {
	fileDialogMu.Lock()
	fileDialogHandler = openFn
	fileDialogMu.Unlock()
}

// RegisterColorPickerHandler allows the app to provide a renderer for plugin color pickers.
func RegisterColorPickerHandler(openFn func(ColorPickerSpec)) {
	colorPickerMu.Lock()
	colorPickerHandler = openFn
	colorPickerMu.Unlock()
}

// RegisterTerritorySelectorHandler allows the app to render a territory selector UI for plugins.
func RegisterTerritorySelectorHandler(openFn func(TerritorySelectorSpec)) {
	territorySelectorMu.Lock()
	territorySelectorHandler = openFn
	territorySelectorMu.Unlock()
}

func normalizeExtensions(filters []string) []string {
	if len(filters) == 0 {
		return nil
	}
	out := make([]string, 0, len(filters))
	for _, f := range filters {
		trimmed := strings.TrimSpace(f)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func openPluginFileDialog(spec FileDialogSpec, onAccept C.ruea_file_dialog_accept_cb, onCancel C.ruea_file_dialog_cancel_cb, userData unsafe.Pointer) int {
	if spec.PluginID == "" || onAccept == nil || onCancel == nil {
		return hostErrBadArgument
	}
	if spec.Mode != FileDialogOpen && spec.Mode != FileDialogSave {
		return hostErrBadArgument
	}

	fileDialogMu.Lock()
	openFn := fileDialogHandler
	if openFn == nil {
		fileDialogMu.Unlock()
		return hostErrUnsupported
	}
	if len(activeFileDialogs) > 0 {
		fileDialogMu.Unlock()
		return hostErrUnsupported
	}

	cleaned := spec
	cleaned.Filters = normalizeExtensions(spec.Filters)
	activeFileDialogs[spec.PluginID] = fileDialogRequest{spec: cleaned, onAccept: onAccept, onCancel: onCancel, userData: userData}
	fileDialogMu.Unlock()

	openFn(cleaned)
	return hostOK
}

// CompleteFileDialogAccept resolves an outstanding file dialog with an accepted path.
func CompleteFileDialogAccept(pluginID, path string) {
	fileDialogMu.Lock()
	req, ok := activeFileDialogs[pluginID]
	if ok {
		delete(activeFileDialogs, pluginID)
	}
	fileDialogMu.Unlock()
	if !ok {
		return
	}
	if path == "" {
		if req.onCancel != nil {
			C.ruea_call_file_cancel(req.onCancel, req.userData)
		}
		return
	}
	cpath := C.CString(path)
	if cpath != nil {
		C.ruea_call_file_accept(req.onAccept, cpath, req.userData)
		C.free(unsafe.Pointer(cpath))
	}
}

// CompleteFileDialogCancel resolves an outstanding file dialog as cancelled.
func CompleteFileDialogCancel(pluginID string) {
	fileDialogMu.Lock()
	req, ok := activeFileDialogs[pluginID]
	if ok {
		delete(activeFileDialogs, pluginID)
	}
	fileDialogMu.Unlock()
	if !ok {
		return
	}
	if req.onCancel != nil {
		C.ruea_call_file_cancel(req.onCancel, req.userData)
	}
}

// clearFileDialogsForPlugin removes pending dialogs when a plugin unloads.
func clearFileDialogsForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}
	fileDialogMu.Lock()
	delete(activeFileDialogs, pluginID)
	fileDialogMu.Unlock()
}

func openPluginColorPicker(spec ColorPickerSpec, onAccept C.ruea_color_picker_accept_cb, onCancel C.ruea_color_picker_cancel_cb, userData unsafe.Pointer) int {
	if spec.PluginID == "" || onAccept == nil || onCancel == nil {
		return hostErrBadArgument
	}

	colorPickerMu.Lock()
	openFn := colorPickerHandler
	if openFn == nil {
		colorPickerMu.Unlock()
		return hostErrUnsupported
	}
	if len(activeColorPickers) > 0 {
		colorPickerMu.Unlock()
		return hostErrUnsupported
	}

	activeColorPickers[spec.PluginID] = colorPickerRequest{spec: spec, onAccept: onAccept, onCancel: onCancel, userData: userData}
	colorPickerMu.Unlock()

	openFn(spec)
	return hostOK
}

// CompleteColorPickerAccept resolves an outstanding color picker with the chosen color.
func CompleteColorPickerAccept(pluginID string, c color.RGBA) {
	colorPickerMu.Lock()
	req, ok := activeColorPickers[pluginID]
	if ok {
		delete(activeColorPickers, pluginID)
	}
	colorPickerMu.Unlock()
	if !ok {
		return
	}
	C.ruea_call_color_accept(req.onAccept, C.uint8_t(c.R), C.uint8_t(c.G), C.uint8_t(c.B), req.userData)
}

// CompleteColorPickerCancel resolves an outstanding color picker as cancelled.
func CompleteColorPickerCancel(pluginID string) {
	colorPickerMu.Lock()
	req, ok := activeColorPickers[pluginID]
	if ok {
		delete(activeColorPickers, pluginID)
	}
	colorPickerMu.Unlock()
	if !ok {
		return
	}
	if req.onCancel != nil {
		C.ruea_call_color_cancel(req.onCancel, req.userData)
	}
}

// clearColorPickersForPlugin removes pending color picker requests when a plugin unloads.
func clearColorPickersForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}
	colorPickerMu.Lock()
	delete(activeColorPickers, pluginID)
	colorPickerMu.Unlock()
}

func openPluginTerritorySelector(spec TerritorySelectorSpec, onAccept C.ruea_territory_selector_accept_cb, onCancel C.ruea_territory_selector_cancel_cb, userData unsafe.Pointer) int {
	if spec.PluginID == "" || onAccept == nil || onCancel == nil {
		return hostErrBadArgument
	}

	territorySelectorMu.Lock()
	openFn := territorySelectorHandler
	if openFn == nil {
		territorySelectorMu.Unlock()
		return hostErrUnsupported
	}
	if len(activeTerritorySelectors) > 0 {
		territorySelectorMu.Unlock()
		return hostErrUnsupported
	}

	activeTerritorySelectors[spec.PluginID] = territorySelectorRequest{spec: spec, onAccept: onAccept, onCancel: onCancel, userData: userData}
	territorySelectorMu.Unlock()

	openFn(spec)
	return hostOK
}

// CompleteTerritorySelectorAccept resolves an outstanding territory selector with the chosen names.
func CompleteTerritorySelectorAccept(pluginID string, territories []string) {
	territorySelectorMu.Lock()
	req, ok := activeTerritorySelectors[pluginID]
	if ok {
		delete(activeTerritorySelectors, pluginID)
	}
	territorySelectorMu.Unlock()
	if !ok {
		return
	}

	if len(territories) == 0 {
		if req.onCancel != nil {
			C.ruea_call_territory_cancel(req.onCancel, req.userData)
		}
		return
	}

	// Build C strings for the selection.
	cNames := make([]*C.char, 0, len(territories))
	for _, name := range territories {
		if name == "" {
			continue
		}
		cstr := C.CString(name)
		if cstr != nil {
			cNames = append(cNames, cstr)
		}
	}
	if len(cNames) == 0 {
		if req.onCancel != nil {
			C.ruea_call_territory_cancel(req.onCancel, req.userData)
		}
		return
	}

	// Ensure C strings are freed after callback.
	defer func() {
		for _, cstr := range cNames {
			C.free(unsafe.Pointer(cstr))
		}
	}()

	C.ruea_call_territory_accept(req.onAccept, (**C.char)(unsafe.Pointer(&cNames[0])), C.size_t(len(cNames)), req.userData)
}

// CompleteTerritorySelectorCancel resolves an outstanding territory selector as cancelled.
func CompleteTerritorySelectorCancel(pluginID string) {
	territorySelectorMu.Lock()
	req, ok := activeTerritorySelectors[pluginID]
	if ok {
		delete(activeTerritorySelectors, pluginID)
	}
	territorySelectorMu.Unlock()
	if !ok {
		return
	}
	if req.onCancel != nil {
		C.ruea_call_territory_cancel(req.onCancel, req.userData)
	}
}

// clearTerritorySelectorsForPlugin removes pending selectors when a plugin unloads.
func clearTerritorySelectorsForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}
	territorySelectorMu.Lock()
	delete(activeTerritorySelectors, pluginID)
	territorySelectorMu.Unlock()
}

//go:build cgo
// +build cgo

package pluginhost

/*
#cgo CFLAGS: -I../sdk
#include "../sdk/RueaES-SDK.h"

static inline void ruea_call_keybind(ruea_keybind_cb cb, void* user_data) {
	if (cb) cb(user_data);
}
*/
import "C"

import (
	"sync"
	"unsafe"

	"RueaES/eruntime"
	"RueaES/typedef"
)

type pluginKeybind struct {
	PluginID       string
	ID             string
	Label          string
	DefaultBinding string
	Callback       C.ruea_keybind_cb
	UserData       unsafe.Pointer
}

// PluginKeybindInfo describes a registered plugin keybind for UI surfaces.
type PluginKeybindInfo struct {
	PluginID       string
	ID             string
	Label          string
	DefaultBinding string
}

var (
	pkMu   sync.RWMutex
	pkRegs = make(map[string]map[string]pluginKeybind) // pluginID -> bindID -> keybind
)

func pluginKey(pluginID, bindID string) string { return pluginID + "::" + bindID }

// registerPluginKeybind records a plugin keybind and ensures the runtime options capture its binding.
func registerPluginKeybind(pluginID, bindID, label, defaultBinding string, cb C.ruea_keybind_cb, userData unsafe.Pointer) int {
	if pluginID == "" || bindID == "" || label == "" || cb == nil {
		return hostErrBadArgument
	}
	canon, ok := typedef.CanonicalizeBinding(defaultBinding)
	if !ok {
		return hostErrBadArgument
	}

	entry := pluginKeybind{PluginID: pluginID, ID: bindID, Label: label, DefaultBinding: canon, Callback: cb, UserData: userData}

	pkMu.Lock()
	if _, ok := pkRegs[pluginID]; !ok {
		pkRegs[pluginID] = make(map[string]pluginKeybind)
	}
	pkRegs[pluginID][bindID] = entry
	pkMu.Unlock()

	// Seed runtime options with the default if missing.
	opts := eruntime.GetRuntimeOptions()
	if opts.PluginKeybinds == nil {
		opts.PluginKeybinds = make(map[string]string)
	}
	key := pluginKey(pluginID, bindID)
	if _, exists := opts.PluginKeybinds[key]; !exists {
		opts.PluginKeybinds[key] = canon
		eruntime.SetRuntimeOptions(opts)
	}
	return hostOK
}

// unregisterKeybindsForPlugin removes all keybinds for a plugin and clears persisted bindings.
func unregisterKeybindsForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}

	pkMu.Lock()
	binds := pkRegs[pluginID]
	delete(pkRegs, pluginID)
	pkMu.Unlock()

	if len(binds) == 0 {
		return
	}

	opts := eruntime.GetRuntimeOptions()
	if opts.PluginKeybinds != nil {
		changed := false
		for bindID := range binds {
			key := pluginKey(pluginID, bindID)
			if _, ok := opts.PluginKeybinds[key]; ok {
				delete(opts.PluginKeybinds, key)
				changed = true
			}
		}
		if changed {
			eruntime.SetRuntimeOptions(opts)
		}
	}
}

// ListPluginKeybinds returns a shallow copy of plugin keybind metadata.
func ListPluginKeybinds() []PluginKeybindInfo {
	pkMu.RLock()
	out := make([]PluginKeybindInfo, 0)
	for pid, entries := range pkRegs {
		for bid, kb := range entries {
			_ = bid
			out = append(out, PluginKeybindInfo{
				PluginID:       pid,
				ID:             kb.ID,
				Label:          kb.Label,
				DefaultBinding: kb.DefaultBinding,
			})
		}
	}
	pkMu.RUnlock()
	return out
}

// CurrentBinding returns the current binding string for a plugin keybind (empty if none).
func CurrentBinding(pluginID, bindID string) string {
	opts := eruntime.GetRuntimeOptions()
	key := pluginKey(pluginID, bindID)
	if opts.PluginKeybinds != nil {
		return opts.PluginKeybinds[key]
	}
	return ""
}

// TriggerPluginKeybind invokes the registered callback for a plugin keybind.
func TriggerPluginKeybind(pluginID, bindID string) bool {
	pkMu.RLock()
	pluginEntries, ok := pkRegs[pluginID]
	if !ok {
		pkMu.RUnlock()
		return false
	}
	entry, ok := pluginEntries[bindID]
	pkMu.RUnlock()
	if !ok || entry.Callback == nil {
		return false
	}
	C.ruea_call_keybind(entry.Callback, entry.UserData)
	return true
}

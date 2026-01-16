//go:build cgo
// +build cgo

package pluginhost

import (
	"sync"
)

// ModalSpec describes a plugin-requested modal including controls and initial values.
type ModalSpec struct {
	PluginID      string
	ModalID       string
	Title         string
	Controls      []UIControl
	InitialValues map[string]any
}

var (
	modalMu           sync.RWMutex
	modalOpenHandler  func(ModalSpec)
	modalCloseHandler func(pluginID, modalID string)
	openModals        = make(map[string]map[string]ModalSpec) // pluginID -> modalID -> spec
)

// RegisterModalHandlers lets the app render and dismiss plugin-driven modals.
func RegisterModalHandlers(openFn func(ModalSpec), closeFn func(pluginID, modalID string)) {
	modalMu.Lock()
	modalOpenHandler = openFn
	modalCloseHandler = closeFn
	modalMu.Unlock()
}

func openPluginModal(spec ModalSpec) int {
	if spec.PluginID == "" || spec.ModalID == "" {
		return hostErrBadArgument
	}

	modalMu.Lock()
	if _, ok := openModals[spec.PluginID]; !ok {
		openModals[spec.PluginID] = make(map[string]ModalSpec)
	}
	openModals[spec.PluginID][spec.ModalID] = spec
	openFn := modalOpenHandler
	modalMu.Unlock()

	if openFn != nil {
		openFn(spec)
	}
	return hostOK
}

func closePluginModal(pluginID, modalID string) int {
	if pluginID == "" || modalID == "" {
		return hostErrBadArgument
	}

	modalMu.Lock()
	pluginModals, ok := openModals[pluginID]
	if ok {
		delete(pluginModals, modalID)
		if len(pluginModals) == 0 {
			delete(openModals, pluginID)
		}
	}
	closeFn := modalCloseHandler
	modalMu.Unlock()

	if closeFn != nil {
		closeFn(pluginID, modalID)
	}
	return hostOK
}

// closeModalsForPlugin clears all open modals for a plugin (called on unload).
func closeModalsForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}

	modalMu.Lock()
	modals := openModals[pluginID]
	delete(openModals, pluginID)
	closeFn := modalCloseHandler
	modalMu.Unlock()

	if closeFn != nil {
		for modalID := range modals {
			closeFn(pluginID, modalID)
		}
	}
}

// CurrentModalSpec returns the stored spec for a plugin modal.
func CurrentModalSpec(pluginID, modalID string) (ModalSpec, bool) {
	modalMu.RLock()
	defer modalMu.RUnlock()
	if m, ok := openModals[pluginID]; ok {
		if spec, ok2 := m[modalID]; ok2 {
			return spec, true
		}
	}
	return ModalSpec{}, false
}

//go:build cgo
// +build cgo

package pluginhost

// #include <stdlib.h>
// #include <string.h>
// #include "../sdk/RueaES-SDK.h"
import "C"
import (
	"encoding/json"
	"fmt"
	"sync"
	"unsafe"

	"RueaES/typedef"
)

// Host manages native plugin instances and their lifecycles.
type Host struct {
	mu      sync.Mutex
	plugins map[string]*Instance // keyed by absolute path
}

// NewHost creates a new plugin host.
func NewHost() *Host {
	return &Host{plugins: make(map[string]*Instance)}
}

// Instance represents a loaded plugin and its callbacks.
type Instance struct {
	state    typedef.PluginState
	p        *platformPlugin
	ui       C.ruea_plugin_ui
	controls []UIControl
}

func (inst *Instance) applyMetadata() {
	if inst.p == nil || inst.p.binding.get_metadata == nil {
		return
	}
	var meta C.ruea_metadata
	switch rc := C.ruea_call_get_metadata(inst.p.binding.get_metadata, &meta); rc {
	case C.RUEA_OK:
		// continue
	case C.RUEA_ERR_UNSUPPORTED:
		return
	default:
		inst.state.LastError = fmt.Sprintf("metadata error: code %d", int(rc))
		return
	}

	assign := func(dst *string, cstr *C.char) {
		if cstr == nil {
			return
		}
		if val := C.GoString(cstr); val != "" {
			*dst = val
		}
	}

	assign(&inst.state.Name, meta.name)
	assign(&inst.state.Author, meta.author)
	assign(&inst.state.Description, meta.description)
	assign(&inst.state.Version, meta.version)
}

// Load loads (or reloads) a plugin from disk and initializes it.
func (h *Host) Load(state *typedef.PluginState) (*Instance, error) {
	p, err := openPlatformPlugin(state.Path)
	if err != nil {
		return nil, fmt.Errorf("load plugin: %w", err)
	}

	inst := &Instance{state: *state, p: p}
	inst.applyMetadata()

	cCfg := toCConfig(&inst.state.Config)
	hostAPI := defaultHostAPI()
	var ui C.ruea_plugin_ui

	initSettings, cleanupSettings, err := mapToCSettings(inst.state.Config.UserSettings)
	if err != nil {
		p.close()
		return nil, fmt.Errorf("prepare settings: %w", err)
	}
	defer cleanupSettings()

	var initSettingsPtr *C.ruea_settings
	if initSettings != nil {
		initSettingsPtr = initSettings
	}

	if p.binding.init != nil {
		if rc := C.ruea_call_init(p.binding.init, &cCfg, hostAPI, &ui, initSettingsPtr); rc != C.RUEA_OK {
			p.close()
			return nil, fmt.Errorf("plugin init failed: code %d", int(rc))
		}
	}

	inst.ui = ui

	// Describe UI once per load.
	if p.binding.describe_ui != nil {
		if ctrls, err := describeUI(p); err == nil {
			inst.controls = ctrls
		}
	}

	// Restore state after init so plugins can assume host is ready.
	if p.binding.set_state != nil && len(inst.state.StateBlob) > 0 {
		cState, cleanupState := blobToCSettings(inst.state.StateBlob)
		defer cleanupState()
		if rc := C.ruea_call_set_state(p.binding.set_state, cState); rc != C.RUEA_OK {
			p.close()
			return nil, fmt.Errorf("plugin state restore failed: code %d", int(rc))
		}
	}

	// Sync settings explicitly if provided.
	if p.binding.set_settings != nil && len(inst.state.Config.UserSettings) > 0 {
		cSettings, cleanup, err := mapToCSettings(inst.state.Config.UserSettings)
		if err != nil {
			p.close()
			return nil, fmt.Errorf("prepare settings: %w", err)
		}
		defer cleanup()
		if rc := C.ruea_call_set_settings(p.binding.set_settings, cSettings); rc != C.RUEA_OK {
			p.close()
			return nil, fmt.Errorf("plugin settings restore failed: code %d", int(rc))
		}
	}

	h.mu.Lock()
	h.plugins[state.Path] = inst
	h.mu.Unlock()

	return inst, nil
}

// Disable shuts down and removes a plugin if it is loaded.
func (h *Host) Disable(path string) {
	h.mu.Lock()
	inst, ok := h.plugins[path]
	if ok {
		delete(h.plugins, path)
	}
	h.mu.Unlock()

	if ok {
		inst.shutdown()
	}
}

// Tick invokes TickFunc on all loaded plugins.
func (h *Host) Tick() {
	h.mu.Lock()
	instances := make([]*Instance, 0, len(h.plugins))
	for _, inst := range h.plugins {
		instances = append(instances, inst)
	}
	h.mu.Unlock()

	for _, inst := range instances {
		inst.tick()
	}
}

// Snapshot returns a copy of the plugin state (including runtime-sourced settings/state).
func (h *Host) Snapshot(path string) (typedef.PluginState, bool) {
	h.mu.Lock()
	inst, ok := h.plugins[path]
	h.mu.Unlock()
	if !ok {
		return typedef.PluginState{}, false
	}
	return inst.snapshot(), true
}

// UIControls returns the UI controls advertised by the plugin, if loaded.
func (h *Host) UIControls(path string) ([]UIControl, bool) {
	h.mu.Lock()
	inst, ok := h.plugins[path]
	h.mu.Unlock()
	if !ok {
		return nil, false
	}
	return inst.controls, true
}

// UpdateSettings pushes user settings into a loaded plugin, if supported, and caches them in state.
func (h *Host) UpdateSettings(path string, settings map[string]any) error {
	h.mu.Lock()
	inst, ok := h.plugins[path]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin not loaded")
	}
	inst.state.Config.UserSettings = settings
	if inst.p != nil && inst.p.binding.set_settings != nil {
		cSettings, cleanup, err := mapToCSettings(settings)
		if err != nil {
			return err
		}
		defer cleanup()
		if rc := C.ruea_call_set_settings(inst.p.binding.set_settings, cSettings); rc != C.RUEA_OK {
			return fmt.Errorf("set_settings error: code %d", int(rc))
		}
	}
	return nil
}

// SendUIEvent dispatches a UI event to the plugin, if supported.
func (h *Host) SendUIEvent(path string, ev UIEvent) error {
	h.mu.Lock()
	inst, ok := h.plugins[path]
	h.mu.Unlock()
	if !ok {
		return fmt.Errorf("plugin not loaded")
	}
	return inst.sendUIEvent(ev)
}

// snapshotAll returns snapshots of all loaded plugins.
func (h *Host) snapshotAll() []typedef.PluginState {
	h.mu.Lock()
	instances := make([]*Instance, 0, len(h.plugins))
	for _, inst := range h.plugins {
		instances = append(instances, inst)
	}
	h.mu.Unlock()

	out := make([]typedef.PluginState, 0, len(instances))
	for _, inst := range instances {
		out = append(out, inst.snapshot())
	}
	return out
}

func (inst *Instance) tick() {
	if inst.p == nil || inst.p.binding.tick == nil {
		return
	}
	if rc := C.ruea_call_tick(inst.p.binding.tick); rc != C.RUEA_OK {
		inst.state.LastError = fmt.Sprintf("tick error: code %d", int(rc))
		inst.state.Enabled = false
	}
}

func (inst *Instance) shutdown() {
	if inst.p != nil && inst.p.binding.shutdown != nil {
		_ = C.ruea_call_shutdown(inst.p.binding.shutdown)
	}
	if inst.p != nil {
		inst.p.close()
	}
}

func (inst *Instance) sendUIEvent(ev UIEvent) error {
	if inst.p == nil || inst.ui.on_ui_event == nil {
		return fmt.Errorf("ui events not supported")
	}
	cEv := toCUIEvent(ev)
	defer freeCUIEvent(&cEv)
	if rc := C.ruea_call_on_ui_event(inst.ui.on_ui_event, &cEv); rc != C.RUEA_OK {
		return fmt.Errorf("ui event error: code %d", int(rc))
	}
	return nil
}

func (inst *Instance) snapshot() typedef.PluginState {
	snapshot := inst.state

	if inst.p != nil && inst.p.binding.get_state != nil {
		var cState C.ruea_settings
		cState.version = C.RUEA_ABI_VERSION
		if rc := C.ruea_call_get_state(inst.p.binding.get_state, &cState); rc == C.RUEA_OK {
			if blob, ok := settingsToBlob(&cState); ok {
				snapshot.StateBlob = blob
			}
		} else {
			snapshot.LastError = fmt.Sprintf("get_state error: code %d", int(rc))
		}
	}

	if inst.p != nil && inst.p.binding.get_settings != nil {
		var cSettings C.ruea_settings
		cSettings.version = C.RUEA_ABI_VERSION
		if rc := C.ruea_call_get_settings(inst.p.binding.get_settings, &cSettings); rc == C.RUEA_OK {
			if m, err := settingsToMap(&cSettings); err == nil {
				snapshot.Config.UserSettings = m
			} else {
				snapshot.LastError = err.Error()
			}
		} else {
			snapshot.LastError = fmt.Sprintf("get_settings error: code %d", int(rc))
		}
	}

	return snapshot
}

func toCConfig(cfg *typedef.PluginConfig) C.ruea_config {
	if cfg == nil {
		return C.ruea_config{version: C.RUEA_ABI_VERSION}
	}
	return C.ruea_config{
		version:            C.RUEA_ABI_VERSION,
		allow_filesystem:   boolToUint8(cfg.AllowFileSystem),
		allow_network:      boolToUint8(cfg.AllowNetwork),
		allow_cpu:          boolToUint8(cfg.AllowCPU),
		allow_time:         boolToUint8(cfg.AllowTime),
		allow_state_access: boolToUint8(cfg.AllowStateAccess),
	}
}

func boolToUint8(v bool) C.uint8_t {
	if v {
		return 1
	}
	return 0
}

// mapToCSettings converts a map into a C ruea_settings struct and returns a cleanup func.
func mapToCSettings(m map[string]any) (*C.ruea_settings, func(), error) {
	cleanup := func() {}
	if len(m) == 0 {
		return nil, cleanup, nil
	}

	kvCount := len(m)
	mem := C.malloc(C.size_t(kvCount) * C.size_t(C.sizeof_ruea_kv))
	if mem == nil {
		return nil, cleanup, fmt.Errorf("malloc kvs failed")
	}
	cleanupFns := []func(){}
	cleanup = func() {
		for _, fn := range cleanupFns {
			fn()
		}
		C.free(mem)
	}

	kvs := unsafe.Slice((*C.ruea_kv)(mem), kvCount)
	idx := 0
	for k, v := range m {
		ckey := C.CString(k)
		cleanupFns = append(cleanupFns, func() { C.free(unsafe.Pointer(ckey)) })
		kvs[idx].key = ckey
		switch val := v.(type) {
		case int:
			C.ruea_kv_set_i64(&kvs[idx], C.int64_t(val))
		case int32:
			C.ruea_kv_set_i64(&kvs[idx], C.int64_t(val))
		case int64:
			C.ruea_kv_set_i64(&kvs[idx], C.int64_t(val))
		case uint:
			C.ruea_kv_set_i64(&kvs[idx], C.int64_t(val))
		case uint64:
			C.ruea_kv_set_i64(&kvs[idx], C.int64_t(val))
		case float32:
			C.ruea_kv_set_f64(&kvs[idx], C.double(val))
		case float64:
			C.ruea_kv_set_f64(&kvs[idx], C.double(val))
		case bool:
			if val {
				C.ruea_kv_set_i64(&kvs[idx], C.int64_t(1))
			} else {
				C.ruea_kv_set_i64(&kvs[idx], C.int64_t(0))
			}
		case string:
			cstr := C.CString(val)
			cleanupFns = append(cleanupFns, func() { C.free(unsafe.Pointer(cstr)) })
			C.ruea_kv_set_str(&kvs[idx], cstr, C.size_t(len(val)))
		case []byte:
			if len(val) > 0 {
				cbin := C.malloc(C.size_t(len(val)))
				if cbin == nil {
					cleanup()
					return nil, func() {}, fmt.Errorf("malloc bin failed")
				}
				C.memcpy(cbin, unsafe.Pointer(&val[0]), C.size_t(len(val)))
				cleanupFns = append(cleanupFns, func() { C.free(cbin) })
				C.ruea_kv_set_bin(&kvs[idx], cbin, C.size_t(len(val)))
			} else {
				C.ruea_kv_set_bin(&kvs[idx], nil, 0)
			}
		default:
			C.ruea_kv_set_none(&kvs[idx])
		}
		idx++
	}

	settings := &C.ruea_settings{
		version: C.RUEA_ABI_VERSION,
		items:   (*C.ruea_kv)(mem),
		count:   C.size_t(kvCount),
	}
	return settings, cleanup, nil
}

// blobToCSettings wraps a binary blob into a single BIN entry keyed by stateBlobKey.
func blobToCSettings(blob []byte) (*C.ruea_settings, func()) {
	m := map[string]any{}
	m[stateBlobKey] = blob
	settings, cleanup, _ := mapToCSettings(m)
	return settings, cleanup
}

func settingsToMap(s *C.ruea_settings) (map[string]any, error) {
	out := make(map[string]any)
	if s == nil || s.items == nil || s.count == 0 {
		return out, nil
	}
	kvs := unsafe.Slice(s.items, s.count)
	for _, kv := range kvs {
		if kv.key == nil {
			continue
		}
		// Skip obviously bogus pointers to avoid crashing GoString.
		if uintptr(unsafe.Pointer(kv.key)) < 0x10000 {
			continue
		}
		key := C.GoString(kv.key)
		switch kv._type {
		case C.RUEA_VAL_I64:
			out[key] = int64(C.ruea_kv_get_i64(&kv))
		case C.RUEA_VAL_F64:
			out[key] = float64(C.ruea_kv_get_f64(&kv))
		case C.RUEA_VAL_STR:
			var l C.size_t
			ptr := C.ruea_kv_get_str(&kv, &l)
			if ptr != nil && l > 0 {
				out[key] = C.GoStringN(ptr, C.int(l))
			} else {
				out[key] = ""
			}
		case C.RUEA_VAL_BIN:
			var l C.size_t
			ptr := C.ruea_kv_get_bin(&kv, &l)
			if ptr != nil && l > 0 {
				out[key] = C.GoBytes(unsafe.Pointer(ptr), C.int(l))
			} else {
				out[key] = []byte{}
			}
		default:
			// ignore
		}
	}
	return out, nil
}

func settingsToBlob(s *C.ruea_settings) ([]byte, bool) {
	if s == nil || s.items == nil || s.count == 0 {
		return nil, false
	}
	kvs := unsafe.Slice(s.items, s.count)
	for _, kv := range kvs {
		if kv.key == nil {
			continue
		}
		key := C.GoString(kv.key)
		if key != stateBlobKey {
			continue
		}
		if kv._type != C.RUEA_VAL_BIN {
			continue
		}
		var l C.size_t
		ptr := C.ruea_kv_get_bin(&kv, &l)
		if ptr == nil || l == 0 {
			return []byte{}, true
		}
		return C.GoBytes(unsafe.Pointer(ptr), C.int(l)), true
	}

	// Fallback: if the plugin returned other kvs, serialize to JSON for persistence.
	if m, err := settingsToMap(s); err == nil && len(m) > 0 {
		if data, jerr := json.Marshal(m); jerr == nil {
			return data, true
		}
	}
	return nil, false
}

// UIControlKind mirrors the C enum for UI controls.
type UIControlKind int

const (
	UIControlButton UIControlKind = iota
	UIControlSlider
	UIControlCheckbox
	UIControlText
	UIControlSelect
)

// UIControl is a Go representation of a plugin-advertised control.
type UIControl struct {
	ID       string
	Label    string
	Kind     UIControlKind
	MinValue float64
	MaxValue float64
	Step     float64
	Options  []string
}

// UIEvent is a host-side UI interaction to send back to the plugin.
type UIEvent struct {
	ControlID string
	ModalID   string
	Kind      UIControlKind
	F64       float64
	I64       int64
	Str       string
}

func describeUI(p *platformPlugin) ([]UIControl, error) {
	if p == nil || p.binding.describe_ui == nil {
		return nil, nil
	}
	var desc C.ruea_ui_desc
	desc.version = C.RUEA_ABI_VERSION
	if rc := C.ruea_call_describe_ui(p.binding.describe_ui, &desc); rc != C.RUEA_OK {
		return nil, fmt.Errorf("describe_ui error: code %d", int(rc))
	}
	if desc.controls == nil || desc.count == 0 {
		return nil, nil
	}
	ctrls := unsafe.Slice(desc.controls, desc.count)
	out := make([]UIControl, 0, desc.count)
	for _, c := range ctrls {
		out = append(out, UIControl{
			ID:       C.GoString(c.id),
			Label:    C.GoString(c.label),
			Kind:     UIControlKind(c.kind),
			MinValue: float64(c.min_value),
			MaxValue: float64(c.max_value),
			Step:     float64(c.step),
			Options:  goStrings(c.options, c.option_count),
		})
	}
	return out, nil
}

func goStrings(arr **C.char, count C.size_t) []string {
	if arr == nil || count == 0 {
		return nil
	}
	slice := unsafe.Slice(arr, count)
	out := make([]string, 0, count)
	for _, s := range slice {
		if s != nil {
			out = append(out, C.GoString(s))
		}
	}
	return out
}

func toCUIEvent(ev UIEvent) C.ruea_ui_event {
	var cEv C.ruea_ui_event
	cEv.control_id = C.CString(ev.ControlID)
	if ev.ModalID != "" {
		cEv.modal_id = C.CString(ev.ModalID)
	}
	cEv.kind = C.ruea_ctrl_kind(ev.Kind)
	switch ev.Kind {
	case UIControlSlider:
		C.ruea_ui_event_set_f64(&cEv, C.double(ev.F64))
	case UIControlCheckbox, UIControlSelect:
		C.ruea_ui_event_set_i64(&cEv, C.int64_t(ev.I64))
	case UIControlText:
		cstr := C.CString(ev.Str)
		C.ruea_ui_event_set_str(&cEv, cstr, C.size_t(len(ev.Str)))
	case UIControlButton:
		// no payload
	}
	return cEv
}

func freeCUIEvent(ev *C.ruea_ui_event) {
	if ev == nil {
		return
	}
	if ev.control_id != nil {
		C.free(unsafe.Pointer(ev.control_id))
		ev.control_id = nil
	}
	if ev.modal_id != nil {
		C.free(unsafe.Pointer(ev.modal_id))
		ev.modal_id = nil
	}
	if ev.kind == C.RUEA_CTRL_TEXT {
		ptr := C.ruea_ui_event_str_ptr(ev)
		if ptr != nil {
			C.free(unsafe.Pointer(ptr))
		}
		C.ruea_ui_event_clear_str(ev)
	}
}

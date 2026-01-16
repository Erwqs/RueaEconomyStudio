//go:build cgo
// +build cgo

package pluginhost

/*
#cgo CFLAGS: -I../sdk
#include <stdlib.h>
#include "../sdk/RueaES-SDK.h"
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"RueaES/eruntime"
)

type pathfinderProvider struct {
	PluginID string
	Name     string
	Fn       C.ruea_pathfinder_f
}

type calculatorProvider struct {
	PluginID string
	Fn       C.ruea_calculator_f
}

type costProvider struct {
	PluginID string
	Payload  map[string]any
}

// CostProviderInfo describes a registered cost provider.
type CostProviderInfo struct {
	PluginID string
}

var (
	pfMu   sync.RWMutex
	pfRegs = make(map[string]pathfinderProvider) // key = pluginID::displayName

	calcMu   sync.RWMutex
	calcRegs = make(map[string]calculatorProvider)

	costMu   sync.RWMutex
	costRegs = make(map[string]costProvider)
)

func pathfinderKey(pluginID, name string) string { return pluginID + "::" + name }

// registerPathfinderProvider stores a plugin-supplied pathfinder callback.
func registerPathfinderProvider(pluginID, name string, fn C.ruea_pathfinder_f) int {
	if pluginID == "" || name == "" || fn == nil {
		return hostErrBadArgument
	}
	pfMu.Lock()
	pfRegs[pathfinderKey(pluginID, name)] = pathfinderProvider{PluginID: pluginID, Name: name, Fn: fn}
	pfMu.Unlock()
	eruntime.SetPathfinderResolver(resolvePathfinderViaPlugin)
	return hostOK
}

// registerCalculatorProvider stores a plugin-supplied calculation callback.
func registerCalculatorProvider(pluginID string, fn C.ruea_calculator_f) int {
	if pluginID == "" || fn == nil {
		return hostErrBadArgument
	}
	calcMu.Lock()
	calcRegs[pluginID] = calculatorProvider{PluginID: pluginID, Fn: fn}
	calcMu.Unlock()

	// External calculators must run sequentially; signal runtime when any are active.
	eruntime.SetExternalCalculatorActive(true)
	return hostOK
}

// registerCostProvider caches plugin-provided cost tables as a generic map.
func registerCostProvider(pluginID string, payload map[string]any) int {
	if pluginID == "" {
		return hostErrBadArgument
	}
	costMu.Lock()
	costRegs[pluginID] = costProvider{PluginID: pluginID, Payload: payload}
	costMu.Unlock()
	return hostOK
}

// unregisterProvidersForPlugin removes all provider registrations for a plugin.
func unregisterProvidersForPlugin(pluginID string) {
	if pluginID == "" {
		return
	}
	pfMu.Lock()
	for k, v := range pfRegs {
		if v.PluginID == pluginID {
			delete(pfRegs, k)
		}
	}
	pfRemaining := len(pfRegs)
	pfMu.Unlock()
	clearSelectedPathfinderIfMissing()
	calcMu.Lock()
	delete(calcRegs, pluginID)
	calcMu.Unlock()
	costMu.Lock()
	delete(costRegs, pluginID)
	costMu.Unlock()
	if pfRemaining == 0 {
		eruntime.ClearPathfinderResolver()
	}

	calcMu.RLock()
	remaining := len(calcRegs)
	calcMu.RUnlock()
	if remaining == 0 {
		eruntime.SetExternalCalculatorActive(false)
	}
}

// clearSelectedPathfinderIfMissing resets runtime selection if the chosen provider no longer exists.
func clearSelectedPathfinderIfMissing() {
	opts := eruntime.GetRuntimeOptions()
	if opts.PathfinderProvider == "" {
		return
	}
	pfMu.RLock()
	_, ok := pfRegs[opts.PathfinderProvider]
	pfMu.RUnlock()
	if !ok {
		opts.PathfinderProvider = ""
		eruntime.SetRuntimeOptions(opts)
	}
}

// PathfinderProviderInfo describes a registered pathfinder.
type PathfinderProviderInfo struct {
	PluginID string
	Name     string
	Key      string
}

// ListPathfinderProviders returns shallow copy of providers.
func ListPathfinderProviders() []PathfinderProviderInfo {
	pfMu.RLock()
	out := make([]PathfinderProviderInfo, 0, len(pfRegs))
	for k, p := range pfRegs {
		out = append(out, PathfinderProviderInfo{PluginID: p.PluginID, Name: p.Name, Key: k})
	}
	pfMu.RUnlock()
	return out
}

// CalculatorProviderInfo describes a registered calculator.
type CalculatorProviderInfo struct {
	PluginID string
}

// ListCalculatorProviders returns shallow copy of calculator providers.
func ListCalculatorProviders() []CalculatorProviderInfo {
	calcMu.RLock()
	out := make([]CalculatorProviderInfo, 0, len(calcRegs))
	for _, p := range calcRegs {
		out = append(out, CalculatorProviderInfo{PluginID: p.PluginID})
	}
	calcMu.RUnlock()
	return out
}

// CostProviderPayload returns the stored cost payload for a plugin, if any.
func CostProviderPayload(pluginID string) (map[string]any, bool) {
	costMu.RLock()
	entry, ok := costRegs[pluginID]
	costMu.RUnlock()
	if !ok {
		return nil, false
	}
	return entry.Payload, true
}

// ListCostProviders returns shallow copy of cost providers.
func ListCostProviders() []CostProviderInfo {
	costMu.RLock()
	out := make([]CostProviderInfo, 0, len(costRegs))
	for _, p := range costRegs {
		out = append(out, CostProviderInfo{PluginID: p.PluginID})
	}
	costMu.RUnlock()
	return out
}

func pathfinderProviderByKey(key string) (pathfinderProvider, bool) {
	pfMu.RLock()
	defer pfMu.RUnlock()
	if key != "" {
		p, ok := pfRegs[key]
		return p, ok
	}
	for _, p := range pfRegs {
		return p, true
	}
	return pathfinderProvider{}, false
}

// resolvePathfinderViaPlugin adapts the plugin call to the runtime resolver contract.
func resolvePathfinderViaPlugin(graph eruntime.PathfinderGraph, src, dst string) ([]string, error) {
	selectedKey := eruntime.GetRuntimeOptions().PathfinderProvider
	provider, ok := pathfinderProviderByKey(selectedKey)
	if !ok {
		// Selection is invalid; revert to built-in for future calls.
		opts := eruntime.GetRuntimeOptions()
		opts.PathfinderProvider = ""
		eruntime.SetRuntimeOptions(opts)
		return nil, errors.New("pathfinder provider not found; reverted to built-in")
	}
	resp, rc := callPathfinder(provider, graph, src, dst)
	if rc != hostOK {
		return nil, fmt.Errorf("pathfinder call failed: %d", rc)
	}
	route, err := extractRoute(resp)
	if err != nil {
		return nil, err
	}
	return route, nil
}

func extractRoute(payload map[string]any) ([]string, error) {
	raw, ok := payload["route"]
	if !ok {
		return nil, errors.New("pathfinder response missing route")
	}
	switch v := raw.(type) {
	case []string:
		return v, nil
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, errors.New("route entries must be strings")
			}
			out = append(out, s)
		}
		return out, nil
	case string:
		var arr []string
		if err := json.Unmarshal([]byte(v), &arr); err != nil {
			return nil, errors.New("route string is not valid JSON array")
		}
		return arr, nil
	case []byte:
		var arr []string
		if err := json.Unmarshal(v, &arr); err != nil {
			return nil, errors.New("route bytes are not valid JSON array")
		}
		return arr, nil
	default:
		return nil, errors.New("route field is not an array")
	}
}

// buildCPathfinderGraph converts a Go graph into a C ruea_settings payload containing a ruea_path_graph.
func buildCPathfinderGraph(graph eruntime.PathfinderGraph) (*C.ruea_settings, func(), error) {
	nodeCount := len(graph.Territories)
	if nodeCount == 0 {
		return nil, func() {}, errors.New("empty graph")
	}

	nodesMem := C.malloc(C.size_t(nodeCount) * C.size_t(C.sizeof_ruea_path_node))
	if nodesMem == nil {
		return nil, func() {}, errors.New("malloc nodes failed")
	}
	nodes := unsafe.Slice((*C.ruea_path_node)(nodesMem), nodeCount)

	graphMem := C.malloc(C.size_t(C.sizeof_ruea_path_graph))
	if graphMem == nil {
		C.free(nodesMem)
		return nil, func() {}, errors.New("malloc graph failed")
	}
	graphPtr := (*C.ruea_path_graph)(graphMem)
	graphPtr.nodes = (*C.ruea_path_node)(nodesMem)
	graphPtr.node_count = C.size_t(nodeCount)

	cstrs := []unsafe.Pointer{nodesMem, graphMem}
	cleanup := func() {
		for _, p := range cstrs {
			C.free(p)
		}
	}

	i := 0
	for name, terr := range graph.Territories {
		n := &nodes[i]
		n.name = C.CString(name)
		n.id = C.CString(terr.ID)
		n.guild_tag = C.CString(terr.GuildTag)
		n.routing_mode = C.int8_t(terr.RoutingMode)
		cstrs = append(cstrs, unsafe.Pointer(n.name), unsafe.Pointer(n.id), unsafe.Pointer(n.guild_tag))

		linksCount := len(terr.Links)
		if linksCount > 0 {
			linksMem := C.malloc(C.size_t(linksCount) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
			if linksMem == nil {
				cleanup()
				return nil, func() {}, errors.New("malloc links failed")
			}
			links := unsafe.Slice((**C.char)(linksMem), linksCount)
			for idx, link := range terr.Links {
				links[idx] = C.CString(link)
				cstrs = append(cstrs, unsafe.Pointer(links[idx]))
			}
			n.links = (**C.char)(linksMem)
			n.link_count = C.size_t(linksCount)
			cstrs = append(cstrs, linksMem)
		}
		i++
	}

	// Build ruea_settings manually with a single BIN entry pointing to the graph struct.
	mem := C.malloc(C.size_t(C.sizeof_ruea_kv))
	if mem == nil {
		cleanup()
		return nil, func() {}, errors.New("malloc kv failed")
	}
	kv := (*C.ruea_kv)(mem)
	kv.key = C.CString("graph")
	C.ruea_kv_set_bin(kv, graphMem, C.size_t(C.sizeof_ruea_path_graph))

	settings := &C.ruea_settings{
		version: C.RUEA_ABI_VERSION,
		items:   kv,
		count:   1,
	}

	cleanupWithKV := func() {
		cleanup()
		C.free(unsafe.Pointer(kv.key))
		C.free(mem)
	}

	return settings, cleanupWithKV, nil
}

// callPathfinder invokes the plugin pathfinder with a struct graph payload.
func callPathfinder(provider pathfinderProvider, graph eruntime.PathfinderGraph, src, dst string) (map[string]any, int) {
	graphSettings, cleanup, err := buildCPathfinderGraph(graph)
	if err != nil {
		return nil, hostErrBadArgument
	}
	defer cleanup()

	cSrc := C.CString(src)
	defer C.free(unsafe.Pointer(cSrc))
	cDst := C.CString(dst)
	defer C.free(unsafe.Pointer(cDst))

	var out C.ruea_settings
	out.version = C.RUEA_ABI_VERSION
	rc := C.ruea_call_pathfinder(provider.Fn, graphSettings, cSrc, cDst, &out)
	if rc != C.RUEA_OK {
		return nil, int(rc)
	}
	resp, convErr := settingsToMap(&out)
	if convErr != nil {
		return nil, hostErrInternal
	}
	return resp, hostOK
}

// callCalculator invokes the plugin calculator with a JSON snapshot payload.
func callCalculator(pluginID string, snapshot any) (map[string]any, int) {
	calcMu.RLock()
	provider, ok := calcRegs[pluginID]
	calcMu.RUnlock()
	if !ok {
		return nil, hostErrUnsupported
	}
	blob, err := json.Marshal(snapshot)
	if err != nil {
		return nil, hostErrBadArgument
	}
	cCtx, cleanup, err := mapToCSettings(map[string]any{"json": blob})
	if err != nil || cCtx == nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, hostErrBadArgument
	}
	defer cleanup()
	var out C.ruea_settings
	out.version = C.RUEA_ABI_VERSION
	rc := C.ruea_call_calculator(provider.Fn, cCtx, &out)
	if rc != C.RUEA_OK {
		return nil, int(rc)
	}
	resp, convErr := settingsToMap(&out)
	if convErr != nil {
		return nil, hostErrInternal
	}
	return resp, hostOK
}

// registerCostProviderFromSettings converts a ruea_settings payload to map and registers it.
func registerCostProviderFromSettings(pluginID string, costs *C.ruea_settings) int {
	if pluginID == "" || costs == nil {
		return hostErrBadArgument
	}
	payload, err := settingsToMap(costs)
	if err != nil {
		return hostErrBadArgument
	}
	return registerCostProvider(pluginID, payload)
}

// CalculatorSnapshot is an opaque snapshot envelope for calculators.
type CalculatorSnapshot struct {
	State any `json:"state"`
}

// BuildCalculatorSnapshot wraps state into an envelope.
func BuildCalculatorSnapshot(state any) CalculatorSnapshot {
	return CalculatorSnapshot{State: state}
}

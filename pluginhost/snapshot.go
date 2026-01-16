//go:build cgo
// +build cgo

package pluginhost

/*
#cgo CFLAGS: -I../sdk
#include "../sdk/RueaES-SDK.h"
*/
import "C"

import (
	"encoding/json"

	"RueaES/eruntime"
	"RueaES/typedef"
)

type snapshotEnvelope struct {
	Territories map[string]*eruntime.TerritoryStats `json:"territories"`
	System      *eruntime.SystemStats               `json:"system"`
	Runtime     typedef.RuntimeOptions              `json:"runtime_options"`
}

func buildSnapshot() (snapshotEnvelope, error) {
	terr := eruntime.GetAllTerritoryStats()
	sys := eruntime.GetSystemStats()
	runtimeOpts := eruntime.GetRuntimeOptions()
	return snapshotEnvelope{Territories: terr, System: sys, Runtime: runtimeOpts}, nil
}

// fillStateSnapshot writes a snapshot into outState as a single JSON blob setting.
// Note: memory allocated for the ruea_settings is intentionally retained for plugin consumption.
func fillStateSnapshot(outState *C.ruea_settings) int {
	if outState == nil {
		return hostErrBadArgument
	}
	payload, err := buildSnapshot()
	if err != nil {
		return hostErrInternal
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return hostErrInternal
	}
	cSettings, cleanup, err := mapToCSettings(map[string]any{"json": blob})
	if err != nil || cSettings == nil {
		if cleanup != nil {
			cleanup()
		}
		return hostErrBadArgument
	}
	// Intentionally leak the allocated settings to keep memory valid for plugin use.
	*outState = *cSettings
	return hostOK
}

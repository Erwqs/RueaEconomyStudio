//go:build cgo
// +build cgo

package pluginhost

/*
#cgo CFLAGS: -I../sdk
#include "../sdk/RueaES-SDK.h"
#include <stdlib.h>
*/
import "C"
import (
	"image/color"
	"unsafe"

	"RueaES/eruntime"
	"RueaES/typedef"
)

//export ruea_host_set_overlay_color
func ruea_host_set_overlay_color(territory *C.char, r C.uint8_t, g C.uint8_t, b C.uint8_t, a C.uint8_t) C.int {
	if territory == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	name := C.GoString(territory)
	setOverlayColor(name, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)})
	return C.int(C.RUEA_OK)
}

//export ruea_host_clear_overlay_color
func ruea_host_clear_overlay_color(territory *C.char) C.int {
	if territory == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	name := C.GoString(territory)
	clearOverlay(name)
	return C.int(C.RUEA_OK)
}

//export ruea_host_clear_overlay_colors
func ruea_host_clear_overlay_colors() C.int {
	clearOverlays()
	return C.int(C.RUEA_OK)
}

//export ruea_host_get_overlay_color
func ruea_host_get_overlay_color(territory *C.char, r *C.uint8_t, g *C.uint8_t, b *C.uint8_t, a *C.uint8_t, found *C.uint8_t) C.int {
	if territory == nil || r == nil || g == nil || b == nil || a == nil || found == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	name := C.GoString(territory)
	if c, ok := GetOverlayColor(name); ok {
		*r = C.uint8_t(c.R)
		*g = C.uint8_t(c.G)
		*b = C.uint8_t(c.B)
		*a = C.uint8_t(c.A)
		*found = 1
		return C.int(C.RUEA_OK)
	}
	*found = 0
	return C.int(C.RUEA_OK)
}

//export ruea_host_toast
func ruea_host_toast(message *C.char, r C.uint8_t, g C.uint8_t, b C.uint8_t, a C.uint8_t) C.int {
	if message == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	msg := C.GoString(message)
	showToast(msg, color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)})
	return C.int(C.RUEA_OK)
}

//export ruea_host_command
func ruea_host_command(verb *C.char, args *C.ruea_settings, outResponse *C.ruea_settings) C.int {
	if verb == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goVerb := C.GoString(verb)
	goArgs, err := settingsToMap(args)
	if err != nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	if outResponse != nil {
		outResponse.version = C.RUEA_ABI_VERSION
		outResponse.items = nil
		outResponse.count = 0
	}
	switch rc, resp := runCommand(goVerb, goArgs); rc {
	case hostOK:
		if outResponse != nil && resp != nil {
			if cResp, cleanup, err := mapToCSettings(resp); err == nil && cResp != nil {
				*outResponse = *cResp
				defer cleanup()
			}
		}
		return C.int(C.RUEA_OK)
	case hostErrBadArgument:
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	case hostErrUnsupported:
		return C.int(C.RUEA_ERR_UNSUPPORTED)
	case hostErrInternal:
		return C.int(C.RUEA_ERR_INTERNAL)
	default:
		return C.int(rc)
	}
}

//export ruea_host_get_state_snapshot
func ruea_host_get_state_snapshot(outState *C.ruea_settings) C.int {
	if outState == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	return C.int(fillStateSnapshot(outState))
}

//export ruea_host_register_pathfinder
func ruea_host_register_pathfinder(pluginID *C.char, displayName *C.char, fn C.ruea_pathfinder_f) C.int {
	if pluginID == nil || displayName == nil || fn == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	goName := C.GoString(displayName)
	if rc := registerPathfinderProvider(goPlugin, goName, fn); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_register_costs
func ruea_host_register_costs(pluginID *C.char, costs *C.ruea_settings) C.int {
	if pluginID == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	if rc := registerCostProviderFromSettings(goPlugin, costs); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_register_calculator
func ruea_host_register_calculator(pluginID *C.char, fn C.ruea_calculator_f) C.int {
	if pluginID == nil || fn == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	if rc := registerCalculatorProvider(goPlugin, fn); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_register_keybind
func ruea_host_register_keybind(pluginID *C.char, bindID *C.char, label *C.char, defaultBinding *C.char, cb C.ruea_keybind_cb, userData unsafe.Pointer) C.int {
	if pluginID == nil || bindID == nil || label == nil || cb == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	goBind := C.GoString(bindID)
	goLabel := C.GoString(label)
	goDefault := ""
	if defaultBinding != nil {
		goDefault = C.GoString(defaultBinding)
	}
	if rc := registerPluginKeybind(goPlugin, goBind, goLabel, goDefault, cb, userData); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_open_modal
func ruea_host_open_modal(pluginID *C.char, spec *C.ruea_modal_spec, initialValues *C.ruea_settings) C.int {
	if pluginID == nil || spec == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	goModalID := C.GoString(spec.modal_id)
	goTitle := C.GoString(spec.title)
	controls := controlsFromC(spec.controls, spec.control_count)
	vals, _ := settingsToMap(initialValues)
	ms := ModalSpec{PluginID: goPlugin, ModalID: goModalID, Title: goTitle, Controls: controls, InitialValues: vals}
	if rc := openPluginModal(ms); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

func fillCTerritory(out *C.ruea_territory, snap typedef.TerritorySnapshot) {
	if out == nil {
		return
	}
	*out = C.ruea_territory{}
	copyCString(out.name[:], snap.Name)
	copyCString(out.id[:], snap.ID)
	copyCString(out.guild_name[:], snap.GuildName)
	copyCString(out.guild_tag[:], snap.GuildTag)
	if snap.HQ {
		out.hq = 1
	}
	out.routing_mode = C.int8_t(snap.RoutingMode)
	out.border = C.int8_t(snap.Border)
	out.treasury = C.double(snap.Treasury)
	out.route_tax = C.double(snap.RouteTax)
	out.generation_bonus = C.double(snap.GenerationBonus)
	out.resources.emeralds = C.double(snap.Resources.Emeralds)
	out.resources.ores = C.double(snap.Resources.Ores)
	out.resources.wood = C.double(snap.Resources.Wood)
	out.resources.fish = C.double(snap.Resources.Fish)
	out.resources.crops = C.double(snap.Resources.Crops)
	out.location.start[0] = C.int32_t(snap.Location.Start[0])
	out.location.start[1] = C.int32_t(snap.Location.Start[1])
	out.location.end[0] = C.int32_t(snap.Location.End[0])
	out.location.end[1] = C.int32_t(snap.Location.End[1])
}

func fillCTransitResource(out *C.ruea_transit_resource, snap eruntime.TransitSnapshot) {
	if out == nil {
		return
	}
	*out = C.ruea_transit_resource{}

	copyCString(out.id[:], snap.ID)
	copyCString(out.origin_id[:], snap.OriginID)
	copyCString(out.origin_name[:], snap.OriginName)
	copyCString(out.destination_id[:], snap.DestinationID)
	copyCString(out.destination_name[:], snap.DestinationName)
	copyCString(out.next_id[:], snap.NextID)
	copyCString(out.next_name[:], snap.NextName)

	out.resources.emeralds = C.double(snap.Resources.Emeralds)
	out.resources.ores = C.double(snap.Resources.Ores)
	out.resources.wood = C.double(snap.Resources.Wood)
	out.resources.fish = C.double(snap.Resources.Fish)
	out.resources.crops = C.double(snap.Resources.Crops)

	out.next_tax = C.double(snap.NextTax)
	out.route_index = C.int32_t(snap.RouteIndex)
	out.created_at = C.uint64_t(snap.CreatedAt)
	if snap.Moved {
		out.moved = 1
	}

	maxRoute := int(C.RUEA_MAX_TRANSIT_ROUTE)
	count := len(snap.Route)
	if count > maxRoute {
		count = maxRoute
		out.route_truncated = 1
	}
	out.route_len = C.size_t(count)
	for i := 0; i < count; i++ {
		copyCString(out.route_ids[i][:], snap.Route[i])
	}
}

func copyCString(buf []C.char, s string) {
	n := len(buf)
	if n == 0 {
		return
	}
	b := []byte(s)
	if len(b) >= n {
		b = b[:n-1]
	}
	for i := range buf {
		buf[i] = 0
	}
	for i, v := range b {
		buf[i] = C.char(v)
	}
}

//export ruea_host_close_modal
func ruea_host_close_modal(pluginID *C.char, modalID *C.char) C.int {
	if pluginID == nil || modalID == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goPlugin := C.GoString(pluginID)
	goModal := C.GoString(modalID)
	if rc := closePluginModal(goPlugin, goModal); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_unregister_plugin
func ruea_host_unregister_plugin(pluginID *C.char) C.int {
	if pluginID == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goID := C.GoString(pluginID)
	unregisterProvidersForPlugin(goID)
	unregisterKeybindsForPlugin(goID)
	closeModalsForPlugin(goID)
	clearFileDialogsForPlugin(goID)
	clearColorPickersForPlugin(goID)
	clearTerritorySelectorsForPlugin(goID)
	return C.int(C.RUEA_OK)
}

//export ruea_host_open_file_dialog
func ruea_host_open_file_dialog(pluginID *C.char, spec *C.ruea_file_dialog_spec, onAccept C.ruea_file_dialog_accept_cb, onCancel C.ruea_file_dialog_cancel_cb, userData unsafe.Pointer) C.int {
	if pluginID == nil || spec == nil || onAccept == nil || onCancel == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goSpec := FileDialogSpec{
		PluginID: C.GoString(pluginID),
		Mode:     FileDialogMode(spec.mode),
	}
	if spec.title != nil {
		goSpec.Title = C.GoString(spec.title)
	}
	if spec.default_path != nil {
		goSpec.DefaultPath = C.GoString(spec.default_path)
	}
	goSpec.Filters = goStrings(spec.filters, spec.filter_count)
	if rc := openPluginFileDialog(goSpec, onAccept, onCancel, userData); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_open_color_picker
func ruea_host_open_color_picker(pluginID *C.char, r C.uint8_t, g C.uint8_t, b C.uint8_t, onAccept C.ruea_color_picker_accept_cb, onCancel C.ruea_color_picker_cancel_cb, userData unsafe.Pointer) C.int {
	if pluginID == nil || onAccept == nil || onCancel == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goSpec := ColorPickerSpec{
		PluginID: C.GoString(pluginID),
		Initial:  color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255},
	}
	if rc := openPluginColorPicker(goSpec, onAccept, onCancel, userData); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_open_territory_selector
func ruea_host_open_territory_selector(pluginID *C.char, spec *C.ruea_territory_selector_spec, onAccept C.ruea_territory_selector_accept_cb, onCancel C.ruea_territory_selector_cancel_cb, userData unsafe.Pointer) C.int {
	if pluginID == nil || spec == nil || onAccept == nil || onCancel == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	goSpec := TerritorySelectorSpec{
		PluginID:    C.GoString(pluginID),
		Title:       "",
		MultiSelect: spec.multi_select != 0,
	}
	if spec.title != nil {
		goSpec.Title = C.GoString(spec.title)
	}
	goSpec.Preselect = goStrings(spec.preselect, spec.preselect_count)
	if rc := openPluginTerritorySelector(goSpec, onAccept, onCancel, userData); rc != hostOK {
		return C.int(rc)
	}
	return C.int(C.RUEA_OK)
}

//export ruea_host_get_territory
func ruea_host_get_territory(territory *C.char, out *C.ruea_territory) C.int {
	if territory == nil || out == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	name := C.GoString(territory)
	snap, ok := eruntime.GetTerritorySnapshot(name)
	if !ok {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	fillCTerritory(out, snap)
	return C.int(C.RUEA_OK)
}

//export ruea_host_get_territories
func ruea_host_get_territories(names **C.char, count C.size_t, outArr *C.ruea_territory, outCap C.size_t, written *C.size_t) C.int {
	if outArr == nil || written == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	*written = 0

	goNames := goStrings(names, count)
	if len(goNames) == 0 {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}

	snaps := eruntime.GetTerritorySnapshots(goNames)
	max := int(outCap)
	if max <= 0 {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	if len(snaps) > max {
		snaps = snaps[:max]
	}
	outSlice := unsafe.Slice(outArr, len(snaps))
	for i, snap := range snaps {
		fillCTerritory(&outSlice[i], snap)
	}
	*written = C.size_t(len(snaps))
	return C.int(C.RUEA_OK)
}

//export ruea_host_get_transit_resources
func ruea_host_get_transit_resources(territory *C.char, outArr *C.ruea_transit_resource, outCap C.size_t, written *C.size_t) C.int {
	if territory == nil || outArr == nil || written == nil {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	*written = 0

	terrName := C.GoString(territory)
	snaps := eruntime.GetTransitSnapshotsForTerritory(terrName)
	if len(snaps) == 0 {
		return C.int(C.RUEA_OK)
	}

	max := int(outCap)
	if max <= 0 {
		return C.int(C.RUEA_ERR_BAD_ARGUMENT)
	}
	if len(snaps) > max {
		snaps = snaps[:max]
	}

	outSlice := unsafe.Slice(outArr, len(snaps))
	for i, snap := range snaps {
		fillCTransitResource(&outSlice[i], snap)
	}

	*written = C.size_t(len(snaps))
	return C.int(C.RUEA_OK)
}

// controlsFromC mirrors describeUI conversion for plugin-supplied controls.
func controlsFromC(arr *C.ruea_control, count C.size_t) []UIControl {
	if arr == nil || count == 0 {
		return nil
	}
	slice := unsafe.Slice(arr, count)
	out := make([]UIControl, 0, count)
	for _, c := range slice {
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
	return out
}

// ensure references to avoid unused errors
var _ unsafe.Pointer

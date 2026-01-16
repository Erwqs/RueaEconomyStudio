//go:build cgo && windows
// +build cgo,windows

package pluginhost

/*
#cgo CFLAGS: -I../sdk
#include <windows.h>
#include <stdlib.h>
#include "../sdk/RueaES-SDK.h"

typedef int (*ruea_init_f)(const ruea_config*, const ruea_host_api*, ruea_plugin_ui*, const ruea_settings*);
typedef int (*ruea_tick_f)(void);
typedef int (*ruea_shutdown_f)(void);
typedef int (*ruea_get_state_f)(ruea_settings*);
typedef int (*ruea_set_state_f)(const ruea_settings*);
typedef int (*ruea_get_settings_f)(ruea_settings*);
typedef int (*ruea_set_settings_f)(const ruea_settings*);
typedef int (*ruea_get_metadata_f)(ruea_metadata*);
typedef int (*ruea_describe_ui_f)(ruea_ui_desc*);
typedef void (*ruea_free_f)(void*);

typedef struct {
	HMODULE handle;
	ruea_init_f init;
	ruea_tick_f tick;
	ruea_shutdown_f shutdown;
	ruea_get_state_f get_state;
	ruea_set_state_f set_state;
	ruea_get_settings_f get_settings;
	ruea_set_settings_f set_settings;
	ruea_get_metadata_f get_metadata;
	ruea_describe_ui_f describe_ui;
	ruea_free_f free_fn;
} ruea_binding;

static HMODULE ruea_open(const char* path) { return LoadLibraryA(path); }
static void ruea_close(HMODULE h) { if (h) FreeLibrary(h); }
static FARPROC ruea_sym(HMODULE h, const char* name) { return GetProcAddress(h, name); }

static int ruea_bind(HMODULE h, ruea_binding* out) {
	if (!h || !out) return RUEA_ERR_BAD_ARGUMENT;
	out->handle = h;
	out->init = (ruea_init_f)ruea_sym(h, "Ruea_Init");
	out->tick = (ruea_tick_f)ruea_sym(h, "Ruea_Tick");
	out->shutdown = (ruea_shutdown_f)ruea_sym(h, "Ruea_Shutdown");
	out->get_state = (ruea_get_state_f)ruea_sym(h, "Ruea_GetState");
	out->set_state = (ruea_set_state_f)ruea_sym(h, "Ruea_SetState");
	out->get_settings = (ruea_get_settings_f)ruea_sym(h, "Ruea_GetSettings");
	out->set_settings = (ruea_set_settings_f)ruea_sym(h, "Ruea_SetSettings");
	out->get_metadata = (ruea_get_metadata_f)ruea_sym(h, "Ruea_GetMetadata");
	out->describe_ui = (ruea_describe_ui_f)ruea_sym(h, "Ruea_DescribeUI");
	out->free_fn = (ruea_free_f)ruea_sym(h, "Ruea_Free");
	return RUEA_OK;
}

// Forward declarations of Go-implemented host API functions.
int ruea_host_set_overlay_color(const char* territory, uint8_t r, uint8_t g, uint8_t b, uint8_t a);
int ruea_host_clear_overlay_color(const char* territory);
int ruea_host_clear_overlay_colors(void);
int ruea_host_get_overlay_color(const char* territory, uint8_t* r, uint8_t* g, uint8_t* b, uint8_t* a, uint8_t* found);
int ruea_host_toast(const char* message, uint8_t r, uint8_t g, uint8_t b, uint8_t a);
int ruea_host_command(const char* verb, const ruea_settings* args, ruea_settings* out_response);
int ruea_host_get_state_snapshot(ruea_settings* out_state);
int ruea_host_register_pathfinder(const char* plugin_id, const char* display_name, ruea_pathfinder_f fn);
int ruea_host_register_costs(const char* plugin_id, const ruea_settings* costs);
int ruea_host_register_calculator(const char* plugin_id, ruea_calculator_f fn);
int ruea_host_register_keybind(const char* plugin_id, const char* bind_id, const char* label, const char* default_binding, ruea_keybind_cb cb, void* user_data);
int ruea_host_open_modal(const char* plugin_id, const ruea_modal_spec* spec, const ruea_settings* initial_values);
int ruea_host_close_modal(const char* plugin_id, const char* modal_id);
int ruea_host_unregister_plugin(const char* plugin_id);
int ruea_host_open_file_dialog(const char* plugin_id, const ruea_file_dialog_spec* spec, ruea_file_dialog_accept_cb on_accept, ruea_file_dialog_cancel_cb on_cancel, void* user_data);
int ruea_host_open_color_picker(const char* plugin_id, uint8_t r, uint8_t g, uint8_t b, ruea_color_picker_accept_cb on_accept, ruea_color_picker_cancel_cb on_cancel, void* user_data);
int ruea_host_open_territory_selector(const char* plugin_id, const ruea_territory_selector_spec* spec, ruea_territory_selector_accept_cb on_accept, ruea_territory_selector_cancel_cb on_cancel, void* user_data);
int ruea_host_get_territory(const char* territory, ruea_territory* out);
int ruea_host_get_territories(const char* const* territories, size_t count, ruea_territory* out_array, size_t out_capacity, size_t* written);
int ruea_host_get_transit_resources(const char* territory, ruea_transit_resource* out_array, size_t out_capacity, size_t* written);

static const ruea_host_api default_host_api = {
	ruea_host_set_overlay_color,
	ruea_host_clear_overlay_color,
	ruea_host_clear_overlay_colors,
	ruea_host_get_overlay_color,
	ruea_host_toast,
	ruea_host_command,
	ruea_host_get_state_snapshot,
	ruea_host_register_pathfinder,
	ruea_host_register_costs,
	ruea_host_register_calculator,
	ruea_host_register_keybind,
	ruea_host_open_modal,
	ruea_host_close_modal,
	ruea_host_unregister_plugin,
	ruea_host_open_file_dialog,
	ruea_host_open_color_picker,
	ruea_host_open_territory_selector,
	ruea_host_get_territory,
	ruea_host_get_territories,
	ruea_host_get_transit_resources
};

static const ruea_host_api* ruea_default_host_api() { return &default_host_api; }
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// platformPlugin wraps the resolved function pointers for a shared library.
type platformPlugin struct {
	binding C.ruea_binding
}

func openPlatformPlugin(path string) (*platformPlugin, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))

	h := C.ruea_open(cpath)
	if h == nil {
		return nil, fmt.Errorf("LoadLibrary failed for %s", path)
	}
	var b C.ruea_binding
	if rc := C.ruea_bind(h, &b); rc != C.RUEA_OK {
		C.ruea_close(h)
		return nil, fmt.Errorf("bind failed for %s (code %d)", path, int(rc))
	}
	return &platformPlugin{binding: b}, nil
}

func (p *platformPlugin) close() {
	if p == nil || p.binding.handle == nil {
		return
	}
	C.ruea_close(p.binding.handle)
	p.binding.handle = nil
}

func defaultHostAPI() *C.ruea_host_api {
	return (*C.ruea_host_api)(C.ruea_default_host_api())
}

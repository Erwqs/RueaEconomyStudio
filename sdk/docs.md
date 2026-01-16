# RueaES Extension SDK (ABI v6)

Native extensions are plain shared libraries (`.so` / `.dylib` / `.dll`) that export the Ruea ABI from `sdk/RueaES-SDK.h`. The host loads them via cgo (no Go `plugin` package) and exchanges data through C structs and function pointers.

## Lifecycle Exports
- `Ruea_Init(cfg, host_api, out_ui, initial_settings)` — capture `host_api`, wire UI callbacks, restore state/settings. Return `RUEA_OK` on success.
- `Ruea_Tick()` — called each frame while the plugin is enabled.
- `Ruea_Shutdown()` — cleanup before unload, free everything and host will call dlclose()
- Optional: `Ruea_GetState` / `Ruea_SetState`, `Ruea_GetSettings` / `Ruea_SetSettings`, `Ruea_DescribeUI`, `Ruea_GetMetadata`, `Ruea_Free`.
- Return codes: `RUEA_OK`, `RUEA_ERR_BAD_ARGUMENT`, `RUEA_ERR_UNSUPPORTED`, `RUEA_ERR_INTERNAL`, `RUEA_ERR_NO_MEMORY` (see header for full list).

## Memory & Ownership
- Arguments the host passes to you are **read-only** and only valid for the duration of that call. Copy anything you keep.
- Arrays/strings you pass to the host must stay valid until the call returns. The host copies per call.
- If you allocate a buffer for the host to own (e.g., a `STR` or `BIN` value you want the host to free), return it and ensure `Ruea_Free(void *ptr)` can free it. The host only calls `Ruea_Free` on plugin-allocated pointers.
- `get_state_snapshot` returns host-owned `ruea_settings` containing a `BIN` entry `json` with a full simulator snapshot; it is intentionally kept alive by the host—treat it as read-only and copy what you need.
- Async callbacks (`open_file_dialog`, `open_color_picker`, `open_territory_selector`, `register_keybind`) may run after your call returns; keep any `user_data` or captured pointers valid until the callback fires.
- `ruea_territory` structs filled by the host during `get_territory` / `get_territories` are valid for that call only.
- `get_transit_resources` fills caller-provided `ruea_transit_resource` arrays; data is valid for the call and copies route IDs inline (no JSON involved).
- Booleans travel as `I64` 0/1. Use UTF-8 for strings.

## Data Shapes (selected)
- `ruea_settings`: `{key, type, union}` array; `version` must be `RUEA_ABI_VERSION`.
- `ruea_control`: button, slider, checkbox, text, select. Stable `id`; select options via `options` + `option_count`.
- `ruea_ui_event`: host → plugin when a control changes (`f64`, `i64`, or `str` payload).
- `ruea_modal_spec`: modal id/title + controls array for host-rendered modals.
- `ruea_territory`: snapshot fields (name/id/tag/HQ/border/routing/resources/location, etc.).
- `ruea_transit_resource`: in-transit packet (`id`, `origin/destination/next` ids + names, `resources`, `route_index`, `route_ids[]`, `route_len`, `route_truncated`, `next_tax`, `created_at`, `moved`). Route IDs are capped at `RUEA_MAX_TRANSIT_ROUTE`.

## Host API Table (`ruea_host_api`)
- `set_overlay_color(territory, r,g,b,a)` / `clear_overlay_color(name)` / `clear_overlay_colors()` / `get_overlay_color(name, r,g,b,a, found)` — cached overlay colors used in the Extension map view.
- `toast(message, r,g,b,a)` — UI toast.
- `command(verb, args, out_response)` — generic simulator verbs (see Command Verbs below). `args`/`out_response` are `ruea_settings` (host copies).
- `get_state_snapshot(out_state)` — fills with `{json: <BIN>}` snapshot; read-only.
- `get_transit_resources(territory, out_array, out_capacity, written)` — writes current transits at a territory into caller-provided `ruea_transit_resource` structs. Set `written` to learn how many entries fit; routes are truncated when longer than `RUEA_MAX_TRANSIT_ROUTE`.
- Provider registration: `register_pathfinder(plugin_id, display_name, fn)`, `register_costs(plugin_id, costs)`, `register_calculator(plugin_id, fn)`. Graph/context payloads are host-owned for the call; return results via `ruea_settings` (e.g., pathfinder must return `route` as `[]string`/JSON array).
- Input hooks: `register_keybind(plugin_id, bind_id, label, default_binding, cb, user_data)` — keep `user_data` valid until unregister.
- UI surfaces: `open_modal(plugin_id, spec, initial_values)`, `close_modal(plugin_id, modal_id)`, `unregister_plugin(plugin_id)` (clears providers, modals, dialogs, keybinds).
- Async helpers: `open_file_dialog`, `open_color_picker`, `open_territory_selector` (all require accept + cancel callbacks and stable `user_data`).
- Territory snapshots: `get_territory(name, out)` (one) and `get_territories(names[], count, out_array, out_capacity, written)` (many).

## Command Verbs (via `command`)
| Verb | Args (`ruea_settings` keys) | Response payload | Notes |
| --- | --- | --- | --- |
| `ping` | none | none | Health check.
| `highlight_territory` | `territory` (string) | none | Selects territory on the map.
| `clear_highlight` | none | none | Clears map selection.
| `set_overlay_color` | `territory`, `r`, `g`, `b`, optional `a` (0-255 numbers) | none | Writes overlay cache.
| `clear_overlays` | none | none | Clears all overlay cache entries.
| `clear_overlay` | `territory` | none | Removes a single overlay entry.
| `set_territory_routing_mode` | `territory`, `routing_mode` (0=cheapest, 1=fastest) | none | Updates routing mode and recomputes routes.
| `set_territory_border` | `territory`, `border` (0=closed, 1=open) | none | Sets border status.
| `set_territory_tax` | `territory`, `tax`, `ally_tax` | none | Updates tax rates.
| `set_territory_hq` | `territory`, `hq` (bool as 0/1) | none | Marks/unmarks HQ; triggers route rebuilds.
| `set_territory_upgrade` | `territory`, `upgrade` (damage/attack/health/defence), `level` | none | Applies a single tower upgrade.
| `set_territory_bonus` | `territory`, `bonus` (strongerMinions, towerMultiAttack, towerAura, towerVolley, gatheringExperience, mobExperience, mobDamage, pvpDamage, xpSeeking, tomeSeeking, emeraldSeeking, largerResourceStorage, largerEmeraldStorage, efficientResource, efficientEmerald, resourceRate, emeraldRate), `level` | none | Applies a single bonus (runtime will clamp/enforce per guild limits).
| `set_territory_storage` | `territory`, `storage` (map with `emeralds`,`ores`,`wood`,`fish`,`crops`) | none | Overwrites per-hour storage values.
| `get_overlays` | none | `{overlays: {name: {r,g,b,a}}}` | Returns current cache.
| `get_overlay` | `territory` | `{overlay: {r,g,b,a} | null}` | Single overlay lookup.
| `get_territories` | none | `{territories: [{name, guild_name, guild_tag, hq, location:{start,end}, resources:{emeralds, ore, crops, fish, wood}, color:{r,g,b,a|null}}]}` | `color` is guild color when available.
| `get_guilds` | none | `{guilds: [{name, tag, color, show}]}` | Guild list.
| `get_loadouts` | none | `{loadouts: [{name}]}` | Loadout names.
| `get_state` | none | `{selected_territory, overlay_count, guilds, loadouts, active_tributes, territories, tick}` | Summary counters.
| `get_tribute` | none | `{tributes: [{id, from_guild, to_guild, interval_minutes, active, last_transfer, next_transfer_minutes, next_transfer_ticks, amount_per_hour:{emeralds, ores, wood, fish, crops}}]}` | Active tributes with ETA.
| `create_tribute` | `amount` (resources map), `interval_minutes` (>0), optional `from_guild`, `to_guild` | `{id}` | Adds a tribute.
| `update_tribute` | `id`, optional `amount`, optional `interval_minutes` (>0) | none | At least one of amount/interval required.
| `delete_tribute` | `id` | none | Removes tribute by id.
| `set_tribute_active` | `id`, `active` (bool as 0/1) | none | Enable/disable tribute.


## Example (minimal C plugin)
```c
#include "RueaES-SDK.h"
static const ruea_host_api *g_host;

static int highlight(const char *name) {
    if (!g_host || !g_host->command) return RUEA_ERR_UNSUPPORTED;
    ruea_kv kv[1];
    kv[0].key = "territory";
    ruea_kv_set_str(&kv[0], name, name ? strlen(name) : 0);
    ruea_settings args = { .version = RUEA_ABI_VERSION, .items = kv, .count = 1 };
    return g_host->command("highlight_territory", &args, NULL);
}

RUEA_EXPORT int Ruea_Init(const ruea_config *cfg, const ruea_host_api *host_api, ruea_plugin_ui *out_ui, const ruea_settings *initial_settings) {
    (void)cfg; (void)initial_settings;
    g_host = host_api;
    if (out_ui) out_ui->on_ui_event = NULL;
    if (g_host && g_host->toast) g_host->toast("plugin loaded", 80, 180, 255, 255);
    if (g_host && g_host->set_overlay_color) g_host->set_overlay_color("Detlas", 50, 200, 120, 200);
    highlight("Detlas");
    return RUEA_OK;
}

RUEA_EXPORT int Ruea_Tick(void) {
    // periodic work; return non-zero on error
    return RUEA_OK;
}

RUEA_EXPORT int Ruea_Shutdown(void) { return RUEA_OK; }

RUEA_EXPORT int Ruea_GetMetadata(ruea_metadata *out) {
    if (!out) return RUEA_ERR_BAD_ARGUMENT;
    out->name = "Example Plugin";
    out->author = "you";
    out->description = "Shows overlays and highlights a territory";
    out->version = "0.1.0";
    return RUEA_OK;
}

RUEA_EXPORT void Ruea_Free(void *ptr) { free(ptr); }
```

## Build (quick)
- Linux/macOS: `cc -shared -fPIC -o myplugin.so plugin.c`
- Windows (MSVC): `cl /LD plugin.c /Fe:myplugin.dll`
- Windows (mingw): `x86_64-w64-mingw32-gcc -shared -o myplugin.dll plugin.c`

Copy the resulting shared library into your plugin directory and load it via the Extensions menu (F8).

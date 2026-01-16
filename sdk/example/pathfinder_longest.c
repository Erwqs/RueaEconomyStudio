#include "../RueaES-SDK.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// string vec 
static const char **make_str_array(size_t n) {
    return (const char **)malloc(sizeof(char *) * n);
}

static void free_route(const char **route, size_t n) {
    if (!route) return;
    for (size_t i = 0; i < n; i++) {
        if (route[i]) free((void *)route[i]);
    }
    free(route);
}

// node by name
static ssize_t find_node(const ruea_path_graph *g, const char *name) {
    if (!g || !name) return -1;
    for (size_t i = 0; i < g->node_count; i++) {
        const ruea_path_node *n = &g->nodes[i];
        if (n->name && strcmp(n->name, name) == 0) return (ssize_t)i;
    }
    return -1;
}

// dfs to find longest path
// it doesnt work btw since itll lock up the thread 
// but its for demonstration of pathfinder provider
static void dfs_longest(const ruea_path_graph *g, ssize_t cur, ssize_t dst,
                        int *visited, const char **stack, size_t depth,
                        const char ***best_route, size_t *best_len) {
    if (cur < 0 || (size_t)cur >= g->node_count) return;
    visited[cur] = 1;
    stack[depth] = g->nodes[cur].name;
    depth++;

    if (cur == dst) {
        if (depth > *best_len) {
            // copy stack into best_route
            free_route((const char **)*best_route, *best_len);
            const char **route = make_str_array(depth);
            for (size_t i = 0; i < depth; i++) {
                size_t len = strlen(stack[i]);
                char *cpy = (char *)malloc(len + 1);
                memcpy(cpy, stack[i], len + 1);
                route[i] = cpy;
            }
            *best_route = route;
            *best_len = depth;
        }
    } else {
        const ruea_path_node *n = &g->nodes[cur];
        for (size_t i = 0; i < n->link_count; i++) {
            const char *nbr_name = n->links[i];
            ssize_t nbr_idx = find_node(g, nbr_name);
            if (nbr_idx >= 0 && !visited[nbr_idx]) {
                dfs_longest(g, nbr_idx, dst, visited, stack, depth, best_route, best_len);
            }
        }
    }

    visited[cur] = 0;
}

static int build_route_settings(const char **route, size_t route_len, ruea_settings *out) {
    if (!out) return RUEA_ERR_BAD_ARGUMENT;
    out->version = RUEA_ABI_VERSION;
    if (route_len == 0 || route == NULL) {
        out->items = NULL;
        out->count = 0;
        return RUEA_OK;
    }

    // serialize
    size_t buf_cap = 2; // []
    for (size_t i = 0; i < route_len; i++) {
        buf_cap += strlen(route[i]) + 4; // quotes + comma
    }
    char *json = (char *)malloc(buf_cap + 1);
    if (!json) return RUEA_ERR_NO_MEMORY;
    size_t pos = 0;
    json[pos++] = '[';
    for (size_t i = 0; i < route_len; i++) {
        json[pos++] = '"';
        size_t len = strlen(route[i]);
        memcpy(json + pos, route[i], len);
        pos += len;
        json[pos++] = '"';
        if (i + 1 < route_len) json[pos++] = ',';
    }
    json[pos++] = ']';
    json[pos] = '\0';

    ruea_kv *kv = (ruea_kv *)malloc(sizeof(ruea_kv));
    if (!kv) {
        free(json);
        return RUEA_ERR_NO_MEMORY;
    }
    kv->key = "route";
    kv->type = RUEA_VAL_STR;
    kv->v.str.ptr = json;
    kv->v.str.len = strlen(json);

    out->items = kv;
    out->count = 1;
    return RUEA_OK;
}

// pathfinding callback for the host
static int on_pathfind(const ruea_settings *graph_settings, const char *src, const char *dst, ruea_settings *out_result) {
    if (!graph_settings || !src || !dst || !out_result) return RUEA_ERR_BAD_ARGUMENT;

    // find graph from settings
    const ruea_path_graph *g = NULL;
    if (graph_settings->items && graph_settings->count > 0) {
        const ruea_kv *kvs = graph_settings->items;
        for (size_t i = 0; i < graph_settings->count; i++) {
            const ruea_kv *kv = &kvs[i];
            if (!kv->key || kv->type != RUEA_VAL_BIN) continue;
            if (strcmp(kv->key, "graph") != 0) continue;
            if (kv->v.bin.ptr == NULL || kv->v.bin.len < sizeof(ruea_path_graph)) continue;
            g = (const ruea_path_graph *)kv->v.bin.ptr;
            break;
        }
    }
    if (!g || !g->nodes || g->node_count == 0) return RUEA_ERR_BAD_ARGUMENT;

    ssize_t src_idx = find_node(g, src);
    ssize_t dst_idx = find_node(g, dst);
    if (src_idx < 0 || dst_idx < 0) return RUEA_ERR_BAD_ARGUMENT;

    int *visited = (int *)calloc(g->node_count, sizeof(int));
    const char **stack = make_str_array(g->node_count);
    const char **best_route = NULL;
    size_t best_len = 0;

    dfs_longest(g, src_idx, dst_idx, visited, stack, 0, &best_route, &best_len);

    free(stack);
    free(visited);

    int rc = build_route_settings(best_route, best_len, out_result);
    free_route(best_route, best_len);
    return rc;
}

// no op hook
static int on_init(const ruea_config *cfg, const ruea_host_api *host_api, ruea_plugin_ui *out_ui, const ruea_settings *initial_settings) {
    (void)cfg;
    (void)out_ui;
    (void)initial_settings;
    if (!host_api || !host_api->register_pathfinder) return RUEA_ERR_BAD_ARGUMENT;
    // register pathfinder provider so it show up in state management menu
    return host_api->register_pathfinder("sample-longest", "LongestPath", on_pathfind);
}

static int on_tick(void) { return RUEA_OK; }
static int on_shutdown(void) { return RUEA_OK; }

// exports
int Ruea_Init(const ruea_config *cfg, const ruea_host_api *host_api, ruea_plugin_ui *out_ui, const ruea_settings *initial_settings) {
    return on_init(cfg, host_api, out_ui, initial_settings);
}

int Ruea_Tick(void) { return on_tick(); }
int Ruea_Shutdown(void) { return on_shutdown(); }

int Ruea_GetState(ruea_settings *out_state) { (void)out_state; return RUEA_ERR_UNSUPPORTED; }
int Ruea_SetState(const ruea_settings *state) { (void)state; return RUEA_ERR_UNSUPPORTED; }
int Ruea_GetSettings(ruea_settings *out_settings) { (void)out_settings; return RUEA_ERR_UNSUPPORTED; }
int Ruea_SetSettings(const ruea_settings *settings) { (void)settings; return RUEA_ERR_UNSUPPORTED; }
int Ruea_DescribeUI(ruea_ui_desc *out_ui) { (void)out_ui; return RUEA_ERR_UNSUPPORTED; }

void Ruea_Free(void *ptr) {
    free(ptr);
}

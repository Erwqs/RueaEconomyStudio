package typedef

// PluginConfig defines the resource guardrails for a native plugin.
type PluginConfig struct {
	AllowFileSystem  bool           `json:"allowFileSystem"`
	AllowNetwork     bool           `json:"allowNetwork"`
	AllowCPU         bool           `json:"allowCPU"`
	AllowTime        bool           `json:"allowTime"`
	AllowStateAccess bool           `json:"allowStateAccess"`
	UserSettings     map[string]any `json:"userSettings,omitempty"`
}

// PluginState captures the persisted metadata and state for a plugin.
type PluginState struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Version     string       `json:"version,omitempty"`
	Author      string       `json:"author,omitempty"`
	Description string       `json:"description,omitempty"`
	Path        string       `json:"path"`
	Enabled     bool         `json:"enabled"`
	Config      PluginConfig `json:"config"`
	StateBlob   []byte       `json:"stateBlob,omitempty"`
	Missing     bool         `json:"missing,omitempty"`
	LastError   string       `json:"lastError,omitempty"`
}

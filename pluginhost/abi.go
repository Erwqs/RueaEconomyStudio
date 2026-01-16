package pluginhost

// stateBlobKey is the synthetic key used when mapping binary plugin state to a
// ruea_settings payload. The host stores the returned binary blob into
// typedef.PluginState.StateBlob and feeds it back during SetState calls.
const stateBlobKey = "__state_blob"

package app

// Global MapView instance for singleton access
var globalMapViewInstance *MapView

// GetMapView returns the singleton MapView instance
func GetMapView() *MapView {
	return globalMapViewInstance
}

// SetMapView sets the singleton MapView instance
func SetMapView(mapView *MapView) {
	globalMapViewInstance = mapView
}

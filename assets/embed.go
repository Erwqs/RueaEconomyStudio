package assets

import (
	"embed"
)

//go:embed main-map.png territories.json upgrades.json bg.png hq.png
var AssetFiles embed.FS

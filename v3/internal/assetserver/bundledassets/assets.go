package bundledassets

import _ "embed"

//go:embed importmap.js
var ImportmapJS []byte

//go:embed es-module-shims.js
var ESModuleShims []byte

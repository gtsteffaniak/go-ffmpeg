package platform

import _ "embed"

// embeddedDXGIAdapters is the bundled DXGI enumeration script (Windows only).
//
//go:embed dxgi-adapters.ps1
var embeddedDXGIAdapters string

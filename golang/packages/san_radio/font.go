package san_radio

import _ "embed"

// monoFont is DejaVu Sans Mono (fonts/LICENSE), the terminal's font unless
// Config.Font names another. Built in, it looks the same on every machine.
//
//go:embed fonts/DejaVuSansMono.ttf
var monoFont []byte

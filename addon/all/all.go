// Package all registers all builtin addons with the default registry.
// To build with a subset, don't import this package; instead hand-pick
// the individual addon imports you want.
package all

import (
	_ "go.senan.xyz/wrtag/addon/lyrics"
	_ "go.senan.xyz/wrtag/addon/musicdesc"
	_ "go.senan.xyz/wrtag/addon/replaygain"
	_ "go.senan.xyz/wrtag/addon/subproc"
)

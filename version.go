package wrtag

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var version string
var Version = strings.TrimSpace(version)

var Name = "wrtag"

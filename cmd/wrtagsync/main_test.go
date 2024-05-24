package main

import (
	"os"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/cmd/internal/testing/testcmds"
)

func TestMain(m *testing.M) {
	testcmds.RegisterTransport()

	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtagsync": func() int { main(); return 0 },
		"tag":       func() int { testcmds.Tag(); return 0 },
		"find":      func() int { testcmds.Find(); return 0 },
		"touch":     func() int { testcmds.Touch(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
	})
}

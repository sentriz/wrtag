package main

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
	"go.senan.xyz/wrtag/cmd/internal/testing/testcmds"
)

func TestMain(m *testing.M) {
	testcmds.RegisterTransport()

	os.Exit(testscript.RunMain(m, map[string]func() int{
		"wrtag":    func() int { main(); return 0 },
		"tag":      func() int { testcmds.Tag(); return 0 },
		"find":     func() int { testcmds.Find(); return 0 },
		"touch":    func() int { testcmds.Touch(); return 0 },
		"mime":     func() int { testcmds.MIME(); return 0 },
		"mod-time": func() int { testcmds.ModTime(); return 0 },
		"rand":     func() int { testcmds.Rand(); return 0 },
	}))
}

func TestScripts(t *testing.T) {
	t.Parallel()

	testscript.Run(t, testscript.Params{
		Dir:                 "testdata/scripts",
		RequireExplicitExec: true,
		Condition: func(cond string) (bool, error) {
			switch cond {
			case "ci":
				v, _ := strconv.ParseBool(os.Getenv("CI"))
				return v, nil
			}
			return false, fmt.Errorf("unknown cond")
		},
	})
}

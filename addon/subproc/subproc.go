package subproc

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"go.senan.xyz/wrtag/addon"
)

func init() {
	addon.Register("subproc", NewSubprocAddon)
}

type SubprocAddon struct {
	command string
	args    []string
}

func NewSubprocAddon(conf string) (SubprocAddon, error) {
	var a SubprocAddon
	parts, err := shlex.Split(conf)
	if err != nil {
		return SubprocAddon{}, err
	}
	if len(parts) == 0 {
		return SubprocAddon{}, errors.New("no command provided")
	}
	a.command = parts[0]
	a.args = parts[1:]
	return a, nil
}

const (
	markerFiles = "<files>"
)

func (s SubprocAddon) ProcessRelease(ctx context.Context, paths []string) error {
	var args []string
	for _, arg := range s.args {
		switch arg {
		case markerFiles:
			args = append(args, paths...)
		default:
			args = append(args, arg)
		}
	}

	cmd := exec.CommandContext(ctx, s.command, args...) //nolint:gosec // command name is from user config
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run cmd: %w", err)
	}
	return nil
}

func (s SubprocAddon) String() string {
	args := fmt.Sprintf("%q", append([]string{s.command}, s.args...))
	args = strings.TrimPrefix(args, "[")
	args = strings.TrimSuffix(args, "]")
	return fmt.Sprintf("subproc (%s)", args)
}

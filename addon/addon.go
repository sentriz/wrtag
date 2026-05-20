package addon

import (
	"context"
	"fmt"
)

type Addon interface {
	Check() error
	ProcessRelease(ctx context.Context, cover string, paths []string) error
}

var registry = map[string]func(conf string) (Addon, error){}

func Register[A Addon](name string, addn func(conf string) (A, error)) {
	if _, ok := registry[name]; ok {
		panic(fmt.Errorf("addon %q already registered", name))
	}
	registry[name] = func(conf string) (Addon, error) {
		return addn(conf)
	}
}

func New(name, conf string) (Addon, error) {
	newAddon, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("addon %q not found", name)
	}
	return newAddon(conf)
}

package addon

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Addon represents a plugin that can process files after the main import operation.
type Addon interface {
	// Check validates that the addon has everything it needs to work properly.
	Check() error
	// ProcessRelease is called with the paths of the processed files after a successful import.
	ProcessRelease(ctx context.Context, cover string, paths []string) error
}

var registry = map[string]func(conf string) (Addon, error){}
var registryMu sync.Mutex

// Register adds an addon to the global addon registry.
func Register[A Addon](name string, addn func(conf string) (A, error)) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, ok := registry[name]; ok {
		panic(fmt.Errorf("addon %q already registered", name))
	}

	registry[name] = func(conf string) (Addon, error) {
		return addn(conf)
	}
}

// New initialises a new addon from the registry with the provided conf.
func New(name string, conf string) (Addon, error) {
	registryMu.Lock()
	newAddon, ok := registry[name]
	registryMu.Unlock()

	if !ok {
		return nil, errors.New("addon not found")
	}

	addn, err := newAddon(conf)
	if err != nil {
		return nil, err
	}
	return addn, nil
}

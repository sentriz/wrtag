package addon

import (
	"context"
	"fmt"
	"sync"
)

// Addon represents a plugin that can process files after the main import operation.
type Addon interface {
	// ProcessRelease is called with the paths of the processed files after a successful import.
	ProcessRelease(context.Context, []string) error
}

var registry = map[string]func(conf string) (Addon, error){}
var registryMu sync.Mutex

// Register adds an addon to the global addon registry
func Register(name string, addn func(conf string) (Addon, error)) {
	registryMu.Lock()
	defer registryMu.Unlock()

	if _, ok := registry[name]; ok {
		panic(fmt.Errorf("addon %q already registered", name))
	}

	registry[name] = addn
}

// New initialises a new addon from the registry with the provided conf.
func New(name string, conf string) (Addon, error) {
	registryMu.Lock()
	newAddon, ok := registry[name]
	registryMu.Unlock()

	if !ok {
		return nil, fmt.Errorf("addon not found")
	}

	addn, err := newAddon(conf)
	if err != nil {
		return nil, err
	}
	return addn, nil
}

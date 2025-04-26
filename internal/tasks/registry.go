package tasks

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrTaskAlreadyRegistered indicates that a task with the same name is already registered.
	ErrTaskAlreadyRegistered = errors.New("task already registered")
	// ErrTaskNotFound indicates that a task with the given name was not found in the registry.
	ErrTaskNotFound = errors.New("task not found in registry")
)

// registry stores registered TaskRunner implementations by name.
var (
	registry = make(map[string]TaskRunner)
	regMux   sync.RWMutex // Mutex to protect concurrent access to the registry
)

// RegisterTask adds a TaskRunner to the registry.
func RegisterTask(name string, runner TaskRunner) error {
	regMux.Lock()
	defer regMux.Unlock()

	if _, exists := registry[name]; exists {
		return fmt.Errorf("%w: %s", ErrTaskAlreadyRegistered, name)
	}
	registry[name] = runner
	return nil
}

// GetTask retrieves a TaskRunner from the registry by name.
func GetTask(name string) (TaskRunner, error) {
	regMux.RLock()
	defer regMux.RUnlock()

	runner, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("task '%s': %w", name, ErrTaskNotFound)
	}
	return runner, nil
}

// ListTasks returns a list of registered task names.
func ListTasks() []string {
	regMux.RLock()
	defer regMux.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

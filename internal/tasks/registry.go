package tasks

import (
	"fmt"
	"sync"
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
		return fmt.Errorf("task with name '%s' already registered", name)
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
		return nil, fmt.Errorf("task with name '%s' not found", name)
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

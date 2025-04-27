package tasks

import (
	"errors"
	"fmt"
	"sync"

	"retro/internal/types"
)

var (
	// ErrTaskConstructorAlreadyRegistered indicates that a task constructor with the same name is already registered.
	ErrTaskConstructorAlreadyRegistered = errors.New("task constructor already registered")
	// ErrTaskConstructorNotFound indicates that a task constructor with the given name was not found in the registry.
	ErrTaskConstructorNotFound = errors.New("task constructor not found in registry")
)

// TaskConstructor defines the function signature for creating a TaskRunner instance.
type TaskConstructor func() TaskRunner

// registry stores registered TaskConstructor functions by name (using types.TaskName as key).
var (
	constructors    = make(map[types.TaskName]TaskConstructor)
	constructorsMux sync.RWMutex // Mutex to protect concurrent access to the constructors map
)

// MustRegisterConstructor adds a TaskConstructor to the registry using types.TaskName.
// It panics if a constructor with the same name is already registered.
func MustRegisterConstructor(name types.TaskName, constructor TaskConstructor) {
	constructorsMux.Lock()
	defer constructorsMux.Unlock()

	if _, exists := constructors[name]; exists {
		panic(fmt.Sprintf("task constructor with name '%s' already registered", name))
	}
	constructors[name] = constructor
}

// NewTask creates a new TaskRunner instance using the registered constructor for the given types.TaskName.
func NewTask(name types.TaskName) (TaskRunner, error) {
	constructorsMux.RLock()
	constructor, exists := constructors[name]
	constructorsMux.RUnlock() // Unlock after reading

	if !exists {
		return nil, fmt.Errorf("task constructor for '%s' not found: %w", name, ErrTaskConstructorNotFound)
	}
	return constructor(), nil // Call the constructor to get a new instance
}

// ListTasks returns a list of registered task constructor names (types.TaskName).
func ListTasks() []types.TaskName {
	constructorsMux.RLock()
	defer constructorsMux.RUnlock()

	names := make([]types.TaskName, 0, len(constructors))
	for name := range constructors {
		names = append(names, name)
	}
	return names
}

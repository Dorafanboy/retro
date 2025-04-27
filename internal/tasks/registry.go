package tasks

import (
	"errors"
	"fmt"
	"sync"

	"retro/internal/logger"
	"retro/internal/types"
)

var (
	// ErrTaskConstructorAlreadyRegistered indicates that a task constructor with the same name is already registered.
	ErrTaskConstructorAlreadyRegistered = errors.New("task constructor already registered")
	// ErrTaskConstructorNotFound indicates that a task constructor with the given name was not found in the registry.
	ErrTaskConstructorNotFound = errors.New("task constructor not found in registry")
)

// TaskConstructor defines the function signature for creating task runners.
// It now accepts a logger instance.
type TaskConstructor func(log logger.Logger) TaskRunner

// registry stores registered TaskConstructor functions by name (using types.TaskName as key).
var (
	constructors    = make(map[types.TaskName]TaskConstructor)
	constructorsMux sync.RWMutex // Mutex to protect concurrent access to the constructors map
)

// MustRegisterConstructor registers a task constructor or panics if the name is already registered.
// The constructor must now conform to the TaskConstructor type, accepting a logger.
func MustRegisterConstructor(name types.TaskName, constructor TaskConstructor) {
	constructorsMux.Lock()
	defer constructorsMux.Unlock()

	if _, exists := constructors[name]; exists {
		panic(fmt.Sprintf("task constructor already registered: %s", name))
	}
	constructors[name] = constructor
}

// NewTask creates a new task runner instance by its name.
// It now requires a logger to pass to the task constructor.
// It returns an error if the task name is not registered.
func NewTask(name types.TaskName, log logger.Logger) (TaskRunner, error) {
	constructorsMux.RLock()
	defer constructorsMux.RUnlock()

	constructor, exists := constructors[name]
	if !exists {
		return nil, fmt.Errorf("unknown task name: %s", name)
	}
	// Передаем логгер конструктору
	return constructor(log), nil
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

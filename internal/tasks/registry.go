package tasks

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"retro/internal/logger"
	"retro/internal/types"
)

var ErrTaskConstructorNotFound = errors.New("task constructor not found in registry")

// TaskConstructor defines the function signature for creating task runners.
type TaskConstructor func(log logger.Logger) TaskRunner

var (
	constructors sync.Map
	taskCount    atomic.Int64
)

// MustRegisterConstructor registers a task constructor or panics if the name is already registered.
func MustRegisterConstructor(name types.TaskName, constructor TaskConstructor) {
	if _, loaded := constructors.LoadOrStore(name, constructor); loaded {
		panic(fmt.Sprintf("task constructor already registered: %s", name))
	} else {
		taskCount.Add(1)
	}
}

// NewTask creates a new task runner instance by its name.
func NewTask(name types.TaskName, log logger.Logger) (TaskRunner, error) {
	value, ok := constructors.Load(name)
	if !ok {
		return nil, fmt.Errorf("unknown task name: %s: %w", name, ErrTaskConstructorNotFound)
	}

	constructor, ok := value.(TaskConstructor)
	if !ok {
		return nil, fmt.Errorf("internal error: stored value for task '%s' is not a TaskConstructor", name)
	}

	return constructor(log), nil
}

// ListTasks returns a list of registered task constructor names (types.TaskName).
func ListTasks() []types.TaskName {
	count := int(taskCount.Load())
	names := make([]types.TaskName, 0, count)

	constructors.Range(func(key, value interface{}) bool {
		if taskName, ok := key.(types.TaskName); ok {
			names = append(names, taskName)
		}
		return true
	})
	return names
}

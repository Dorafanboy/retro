package app

import (
	"math/rand"
	"sort"

	"retro_template/internal/config"
	"retro_template/internal/logger"
	"retro_template/internal/types"
	"retro_template/internal/utils"
)

// TaskSelector is responsible for the logic of selecting tasks to execute.
type TaskSelector struct {
	cfg                 *config.Config
	registeredTaskNames []string
}

// newTaskSelector creates a new TaskSelector instance.
func newTaskSelector(cfg *config.Config, registeredTaskNames []string) *TaskSelector {
	return &TaskSelector{
		cfg:                 cfg,
		registeredTaskNames: registeredTaskNames,
	}
}

// SelectTasks selects tasks to run based on configuration.
func (ts *TaskSelector) SelectTasks() ([]config.TaskConfigEntry, error) {
	if len(ts.cfg.Actions.ExplicitTaskSequence) > 0 {
		logger.Debug("Используется режим явной последовательности задач")
		var selected []config.TaskConfigEntry
		taskMap := make(map[string]config.TaskConfigEntry)
		for _, taskCfg := range ts.cfg.Tasks {
			if taskCfg.Enabled {
				taskMap[taskCfg.Name] = taskCfg
			}
		}

		for _, taskName := range ts.cfg.Actions.ExplicitTaskSequence {
			taskCfg, exists := taskMap[taskName]
			if !exists {
				logger.Warn("Задача из явной последовательности не найдена/отключена", "task", taskName)
				continue
			}
			if !isTaskRegistered(taskName, ts.registeredTaskNames) {
				logger.Warn("Задача из явной последовательности не зарегистрирована", "task", taskName)
				continue
			}
			selected = append(selected, taskCfg)
		}
		if len(selected) == 0 {
			return nil, ErrNoValidTasksSelected
		}
		logger.Info("Выбраны задачи из явной последовательности", "count", len(selected))
		return selected, nil
	}

	logger.Debug("Используется режим случайного выбора задач")

	availableTasks := make([]config.TaskConfigEntry, 0)
	for _, taskCfg := range ts.cfg.Tasks {
		if taskCfg.Enabled && isTaskRegistered(taskCfg.Name, ts.registeredTaskNames) {
			availableTasks = append(availableTasks, taskCfg)
		}
	}

	if len(availableTasks) == 0 {
		return nil, ErrNoValidTasksSelected
	}

	minTasks := ts.cfg.Actions.ActionsPerAccount.Min
	maxTasks := ts.cfg.Actions.ActionsPerAccount.Max
	if maxTasks <= 0 || maxTasks > len(availableTasks) {
		maxTasks = len(availableTasks)
	}
	if minTasks <= 0 {
		minTasks = 1
	}
	if minTasks > maxTasks {
		minTasks = maxTasks
	}
	numTasksToSelect := utils.RandomIntInRange(minTasks, maxTasks)
	logger.Info("Будет выбрано задач", "count", numTasksToSelect, "min", minTasks, "max", maxTasks)

	rand.Shuffle(len(availableTasks), func(i, j int) {
		availableTasks[i], availableTasks[j] = availableTasks[j], availableTasks[i]
	})

	selected := availableTasks[:numTasksToSelect]

	if ts.cfg.Actions.TaskOrder == types.TaskOrderSequential {
		logger.Debug("Сортировка выбранных задач по порядку из конфига")
		originalIndex := make(map[string]int)
		for idx, taskCfg := range ts.cfg.Tasks {
			originalIndex[taskCfg.Name] = idx
		}
		sort.SliceStable(selected, func(i, j int) bool {
			return originalIndex[selected[i].Name] < originalIndex[selected[j].Name]
		})
	} else {
		logger.Debug("Порядок выполнения задач - случайный")
	}

	return selected, nil
}

// isTaskRegistered checks if the task name is in the list of registered ones.
func isTaskRegistered(name string, registeredNames []string) bool {
	for _, registeredName := range registeredNames {
		if name == registeredName {
			return true
		}
	}
	return false
}

// getTaskNames - helper for logging task names.
func getTaskNames(tasks []config.TaskConfigEntry) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = task.Name
	}
	return names
}

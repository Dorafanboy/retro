package selector

import (
	"errors"
	"math/rand"
	"sort"

	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/types"
	"retro/internal/utils"
)

// ErrNoValidTasksSelected сигнализирует, что не было выбрано ни одной валидной задачи.
var ErrNoValidTasksSelected = errors.New("selector: no valid and active tasks selected")

// Selector is responsible for the logic of selecting tasks to execute.
type Selector struct {
	cfg *config.Config
}

// NewSelector creates a new Selector instance.
func NewSelector(cfg *config.Config) *Selector {
	return &Selector{
		cfg: cfg,
	}
}

// SelectTasks selects tasks to run based on configuration.
func (s *Selector) SelectTasks() ([]config.TaskConfigEntry, error) {
	if len(s.cfg.Actions.ExplicitTaskSequence) > 0 {
		logger.Debug("Используется режим явной последовательности задач")
		var selected []config.TaskConfigEntry
		taskMap := make(map[types.TaskName]config.TaskConfigEntry)
		for _, taskCfg := range s.cfg.Tasks {
			if taskCfg.Enabled {
				taskMap[taskCfg.Name] = taskCfg
			}
		}

		for _, taskNameStr := range s.cfg.Actions.ExplicitTaskSequence {
			taskName := types.TaskName(taskNameStr)
			taskCfg, exists := taskMap[taskName]
			if !exists {
				logger.Warn("Задача из явной последовательности не найдена/отключена", "task", taskName)
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
	for _, taskCfg := range s.cfg.Tasks {
		if taskCfg.Enabled {
			availableTasks = append(availableTasks, taskCfg)
		}
	}

	if len(availableTasks) == 0 {
		return nil, ErrNoValidTasksSelected
	}

	minTasks := s.cfg.Actions.ActionsPerAccount.Min
	maxTasks := s.cfg.Actions.ActionsPerAccount.Max
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

	if s.cfg.Actions.TaskOrder == types.TaskOrderSequential {
		logger.Debug("Сортировка выбранных задач по порядку из конфига")
		originalIndex := make(map[types.TaskName]int)
		for idx, taskCfg := range s.cfg.Tasks {
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

// getTaskNames - helper for logging task names.
func getTaskNames(tasks []config.TaskConfigEntry) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = string(task.Name)
	}
	return names
}

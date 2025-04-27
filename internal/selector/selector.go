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
	log logger.Logger
}

// NewSelector creates a new Selector instance.
func NewSelector(cfg *config.Config, log logger.Logger) *Selector {
	return &Selector{
		cfg: cfg,
		log: log,
	}
}

// SelectTasks selects tasks to run based on configuration.
func (s *Selector) SelectTasks() ([]config.TaskConfigEntry, error) {
	if len(s.cfg.Actions.ExplicitTaskSequence) > 0 {
		s.log.Debug("Используется режим явной последовательности задач")
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
				s.log.Warn("Задача из явной последовательности не найдена/отключена", "task", taskName)
				continue
			}
			selected = append(selected, taskCfg)
		}
		if len(selected) == 0 {
			return nil, ErrNoValidTasksSelected
		}
		s.log.Info("Выбраны задачи из явной последовательности", "count", len(selected))
		return selected, nil
	}

	s.log.Debug("Используется режим случайного выбора задач")

	availableTasks := make([]config.TaskConfigEntry, 0)
	for _, taskCfg := range s.cfg.Tasks {
		if taskCfg.Enabled {
			availableTasks = append(availableTasks, taskCfg)
		}
	}

	if len(availableTasks) == 0 {
		return nil, ErrNoValidTasksSelected
	}

	// Получаем запрошенные min/max
	minRequested := s.cfg.Actions.ActionsPerAccount.Min
	maxRequested := s.cfg.Actions.ActionsPerAccount.Max

	// Корректируем только отрицательные значения
	if minRequested < 0 {
		s.log.Warn("Запрошенное min значение меньше 0, используется 0.", "requested_min", minRequested)
		minRequested = 0
	}
	if maxRequested < 0 {
		s.log.Warn("Запрошенное max значение меньше 0, используется 0.", "requested_max", maxRequested)
		maxRequested = 0 // Если max отрицательный, разумно сделать его 0
	}

	// Гарантируем, что min <= max
	if minRequested > maxRequested {
		s.log.Warn("Запрошенное min значение больше max, используется min как max.", "requested_min", minRequested, "requested_max", maxRequested)
		maxRequested = minRequested
	}

	// Определяем количество задач на основе запрошенных min/max
	numTasksToSelect := 0
	if maxRequested > 0 || minRequested > 0 {
		numTasksToSelect = utils.RandomIntInRange(minRequested, maxRequested)
	}
	s.log.Info("Выбор количества задач", "requested_min", s.cfg.Actions.ActionsPerAccount.Min, "requested_max", s.cfg.Actions.ActionsPerAccount.Max, "tasks_to_select", numTasksToSelect)

	// Если выбрано 0 задач, возвращаем пустой слайс
	if numTasksToSelect == 0 {
		return []config.TaskConfigEntry{}, nil
	}

	// --- Логика выбора с повторениями (остается без изменений) ---
	selectedTasks := make([]config.TaskConfigEntry, 0, numTasksToSelect)
	for i := 0; i < numTasksToSelect; i++ {
		randomIndex := rand.Intn(len(availableTasks)) // Выбираем из доступных
		selectedTasks = append(selectedTasks, availableTasks[randomIndex])
	}
	// --- Конец логики выбора ---

	if s.cfg.Actions.TaskOrder == types.TaskOrderSequential {
		s.log.Debug("Сортировка выбранных задач по порядку из конфига")
		originalIndex := make(map[types.TaskName]int)
		for idx, taskCfg := range s.cfg.Tasks {
			originalIndex[taskCfg.Name] = idx
		}
		// Сортируем новый слайс selectedTasks
		sort.SliceStable(selectedTasks, func(i, j int) bool {
			return originalIndex[selectedTasks[i].Name] < originalIndex[selectedTasks[j].Name]
		})
	} else {
		s.log.Debug("Порядок выполнения задач - случайный (выбраны случайно с повторениями)")
	}

	return selectedTasks, nil
}

// getTaskNames - helper for logging task names.
func getTaskNames(tasks []config.TaskConfigEntry) []string {
	names := make([]string, len(tasks))
	for i, task := range tasks {
		names[i] = string(task.Name)
	}
	return names
}

package bootstrap

import (
	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/types"

	dummytask "retro/internal/tasks/dummy" // Импорт для конструктора dummy
	// Импортировать другие пакеты задач по мере добавления
)

// Карта всех известных конструкторов задач в проекте.
// Ключ - константа types.TaskName, значение - функция-конструктор.
var allTaskConstructors = map[types.TaskName]tasks.TaskConstructor{
	types.TaskNameLogBalance: tasks.NewLogBalanceTask, // Конструктор из пакета tasks
	types.TaskNameDummy:      dummytask.NewTask,       // Конструктор из пакета dummy
	// Добавлять сюда новые задачи
}

// RegisterTasksFromConfig выполняет явную регистрацию задач на основе config.yml.
// Регистрируются только те задачи, которые включены (Enabled: true) в конфиге
// и для которых есть запись в карте allTaskConstructors.
func RegisterTasksFromConfig(cfg *config.Config) {
	logger.Info("Регистрация задач из конфигурации...")
	registeredCount := 0
	for _, taskCfg := range cfg.Tasks {
		if taskCfg.Enabled {
			// taskCfg.Name уже имеет тип types.TaskName после изменений в config.go
			constructor, ok := allTaskConstructors[taskCfg.Name]
			if ok {
				// Регистрируем, используя имя из конфига (типа types.TaskName)
				tasks.MustRegisterConstructor(taskCfg.Name, constructor)
				registeredCount++
			} else {
				// Имя задачи в логе уже будет types.TaskName
				logger.Warn("Задача из config.yml включена, но не найдена среди известных конструкторов", "task", taskCfg.Name)
			}
		}
	}
	logger.Info("Задачи, зарегистрированные для выполнения", "count", registeredCount, "tasks", tasks.ListTasks())
}

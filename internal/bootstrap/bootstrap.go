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

// RegisterTasksFromConfig registers task constructors found in the config and the local map
// into the central task registry.
func RegisterTasksFromConfig(cfg *config.Config, log logger.Logger) {
	log.Info("Регистрация задач из конфигурации в центральном реестре...")
	registeredCount := 0
	for _, taskCfg := range cfg.Tasks {
		if taskCfg.Enabled {
			constructor, ok := allTaskConstructors[taskCfg.Name] // Получаем конструктор
			if ok {
				log.Debug("Регистрация конструктора в центральном реестре", "task", taskCfg.Name)
				// Регистрируем конструктор в центральном реестре
				tasks.MustRegisterConstructor(taskCfg.Name, constructor)
				registeredCount++ // Считаем успешно зарегистрированные
			} else {
				log.Warn("Задача из config.yml включена, но не найдена среди известных конструкторов в bootstrap", "task", taskCfg.Name)
			}
		}
	}
	// Логируем список задач, которые теперь действительно доступны через tasks.NewTask
	log.Info("Задачи, зарегистрированные и доступные для выполнения", "count", registeredCount, "tasks", tasks.ListTasks())
}

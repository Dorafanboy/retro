package bootstrap

import (
	"retro/internal/config"
	"retro/internal/logger"
	"retro/internal/tasks"
	"retro/internal/types"

	dummytask "retro/internal/tasks/dummy"
)

var allTask = map[types.TaskName]tasks.TaskConstructor{
	types.TaskNameLogBalance: tasks.NewLogBalanceTask,
	types.TaskNameDummy:      dummytask.NewTask,
}

// RegisterTasksFromConfig registers task constructors found in the config and the local map
func RegisterTasksFromConfig(cfg *config.Config, log logger.Logger) {
	log.Info("Регистрация задач из конфигурации в центральном реестре...")
	registeredCount := 0
	for _, taskCfg := range cfg.Tasks {
		if taskCfg.Enabled {
			constructor, ok := allTask[taskCfg.Name]
			if ok {
				log.Debug("Регистрация конструктора в центральном реестре", "task", taskCfg.Name)
				tasks.MustRegisterConstructor(taskCfg.Name, constructor)
				registeredCount++
			} else {
				log.Warn("Задача из config.yml включена, но не найдена среди известных конструкторов в bootstrap",
					"task", taskCfg.Name)
			}
		}
	}
	log.Info("Задачи, зарегистрированные и доступные для выполнения",
		"count", registeredCount, "tasks", tasks.ListTasks())
}

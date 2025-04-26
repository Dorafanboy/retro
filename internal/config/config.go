package config

import (
	"errors"
	"fmt"
	"os"

	"retro_template/internal/types"

	"gopkg.in/yaml.v3"
)

var (
	// ErrConfigNotFound indicates that the configuration file was not found at the specified path.
	ErrConfigNotFound = errors.New("config file not found")
	// ErrConfigReadFailed indicates that there was an error reading the configuration file.
	ErrConfigReadFailed = errors.New("failed to read config file")
	// ErrConfigParseFailed indicates that the configuration file content is not valid YAML.
	ErrConfigParseFailed = errors.New("failed to parse config file (invalid YAML)")
)

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Type             types.DBType `yaml:"type,omitempty"`
	ConnectionString string       `yaml:"connection_string,omitempty"`
	PoolMaxConns     string       `yaml:"pool_max_conns,omitempty"`
}

// TaskConfigEntry defines the structure for a single task entry in the config.
type TaskConfigEntry struct {
	Name    string                 `yaml:"name"`
	Network string                 `yaml:"network"`
	Enabled bool                   `yaml:"enabled"`
	Params  map[string]interface{} `yaml:"params"`
}

// Config corresponds to the structure of config.yml
type Config struct {
	LogFilePath string              `yaml:"log_file_path,omitempty"`
	RPCNodes    map[string][]string `yaml:"rpc_nodes"`
	Concurrency ConcurrencyConfig   `yaml:"concurrency"`
	Wallets     WalletsConfig       `yaml:"wallets"`
	Delay       DelayConfig         `yaml:"delay"`
	Actions     ActionsConfig       `yaml:"actions"`
	Tasks       []TaskConfigEntry   `yaml:"tasks"`
	Database    DatabaseConfig      `yaml:"database"`
}

// ConcurrencyConfig holds settings related to parallel execution
type ConcurrencyConfig struct {
	MaxParallelWallets int `yaml:"max_parallel_wallets"`
}

// WalletsConfig holds settings related to wallet processing
type WalletsConfig struct {
	ProcessOrder types.WalletProcessOrder `yaml:"process_order"`
}

// DelayConfig holds settings for various delays
type DelayConfig struct {
	BetweenAccounts DelayRange `yaml:"between_accounts"`
	BetweenActions  DelayRange `yaml:"between_actions"`
	AfterError      DelayRange `yaml:"after_error"`
	BetweenRetries  RetryDelay `yaml:"between_retries"`
}

// ActionsConfig holds settings related to task execution strategy
type ActionsConfig struct {
	ActionsPerAccount    MinMax          `yaml:"actions_per_account"`
	TaskOrder            types.TaskOrder `yaml:"task_order"`
	ExplicitTaskSequence []string        `yaml:"explicit_task_sequence"`
}

// DelayRange represents a min/max delay with units
type DelayRange struct {
	Min  int            `yaml:"min"`
	Max  int            `yaml:"max"`
	Unit types.TimeUnit `yaml:"unit"`
}

// RetryDelay includes the delay range and number of attempts for retries
type RetryDelay struct {
	Delay    DelayRange `yaml:"delay"`
	Attempts int        `yaml:"attempts"`
}

// MinMax represents a min/max integer range
type MinMax struct {
	Min int `yaml:"min"`
	Max int `yaml:"max"`
}

// LoadConfig reads the configuration file from the given path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("файл конфигурации '%s': %w", path, ErrConfigNotFound)
		}
		return nil, fmt.Errorf("чтение файла '%s': %w: %w", path, ErrConfigReadFailed, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("парсинг YAML из '%s': %w: %w", path, ErrConfigParseFailed, err)
	}

	if dbTypeEnv := os.Getenv("DB_TYPE"); dbTypeEnv != "" {
		cfg.Database.Type = types.DBType(dbTypeEnv)
	}
	if dbConnStr := os.Getenv("DB_CONNECTION_STRING"); dbConnStr != "" {
		cfg.Database.ConnectionString = dbConnStr
	}
	if dbPoolMax := os.Getenv("DB_POOL_MAX_CONNS"); dbPoolMax != "" {
		cfg.Database.PoolMaxConns = dbPoolMax
	}

	return &cfg, nil
}

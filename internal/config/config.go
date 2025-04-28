package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"retro/internal/types"
)

var (
	ErrConfigNotFound    = errors.New("config file not found")
	ErrConfigReadFailed  = errors.New("failed to read config file")
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
	Name    types.TaskName         `yaml:"name"`
	Network string                 `yaml:"network"`
	Enabled bool                   `yaml:"enabled"`
	Params  map[string]interface{} `yaml:"params"`
}

// StateConfig holds configuration related to application state persistence.
type StateConfig struct {
	ResumeEnabled bool `yaml:"resume_enabled"`
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
	State       StateConfig         `yaml:"state"`
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

// LoadConfig reads configuration from the specified file path.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, path)
		}
		return nil, fmt.Errorf("%w reading %s: %w", ErrConfigReadFailed, path, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigParseFailed, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
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

// Validate performs basic validation on the loaded configuration.
func (c *Config) Validate() error {
	// ... (существующий код Validate)
	// Добавить валидацию для State, если нужно
	return nil
}

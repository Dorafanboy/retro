package config

import (
	"errors"
	"fmt"
	"os"

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
// Fields are tagged with omitempty as they can be overridden by environment variables.
type DatabaseConfig struct {
	Type             string `yaml:"type,omitempty"`              // Database type (e.g., "postgres", "sqlite", "none"). Env: DB_TYPE
	ConnectionString string `yaml:"connection_string,omitempty"` // Database connection string. Env: DB_CONNECTION_STRING
	PoolMaxConns     string `yaml:"pool_max_conns,omitempty"`    // Max connections for the pool (used by postgres). Env: DB_POOL_MAX_CONNS
}

// TaskConfigEntry defines the structure for a single task entry in the config.
type TaskConfigEntry struct {
	Name    string                 `yaml:"name"`    // Name corresponding to a registered TaskRunner
	Network string                 `yaml:"network"` // Network name (key in RPCNodes map)
	Enabled bool                   `yaml:"enabled"` // Whether this task configuration is active
	Params  map[string]interface{} `yaml:"params"`  // Task-specific parameters
}

// Config corresponds to the structure of config.yml
// Use yaml tags to map the fields.
type Config struct {
	RPCNodes    map[string][]string `yaml:"rpc_nodes"`
	Concurrency ConcurrencyConfig   `yaml:"concurrency"`
	Wallets     WalletsConfig       `yaml:"wallets"`
	Delay       DelayConfig         `yaml:"delay"`
	Actions     ActionsConfig       `yaml:"actions"`
	Tasks       []TaskConfigEntry   `yaml:"tasks"`
	Database    DatabaseConfig      `yaml:"database"` // Added Database config section
}

// ConcurrencyConfig holds settings related to parallel execution
type ConcurrencyConfig struct {
	MaxParallelWallets int `yaml:"max_parallel_wallets"`
}

// WalletsConfig holds settings related to wallet processing
type WalletsConfig struct {
	ProcessOrder string `yaml:"process_order"`
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
	ActionsPerAccount    MinMax   `yaml:"actions_per_account"`
	TaskOrder            string   `yaml:"task_order"`
	ExplicitTaskSequence []string `yaml:"explicit_task_sequence"` // Added for specific task sequences
}

// DelayRange represents a min/max delay with units
type DelayRange struct {
	Min  int    `yaml:"min"`
	Max  int    `yaml:"max"`
	Unit string `yaml:"unit"`
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

// LoadConfig reads the configuration file from the given path and overrides
// specific fields with environment variables if they are set.
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

	// Override database settings with environment variables if they exist
	if dbType := os.Getenv("DB_TYPE"); dbType != "" {
		cfg.Database.Type = dbType
	}
	if dbConnStr := os.Getenv("DB_CONNECTION_STRING"); dbConnStr != "" {
		cfg.Database.ConnectionString = dbConnStr
	}
	if dbPoolMax := os.Getenv("DB_POOL_MAX_CONNS"); dbPoolMax != "" {
		cfg.Database.PoolMaxConns = dbPoolMax
	}

	return &cfg, nil
}

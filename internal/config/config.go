package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

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

// LoadConfig reads the configuration file from the given path
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data from %s: %w", path, err)
	}

	return &cfg, nil
}

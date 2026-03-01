package sidekiq

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents Sidekiq configuration
type Config struct {
	Concurrency int                 `yaml:"concurrency"`
	Queues      []QueueConfig       `yaml:"queues"`
	Timeout     int                 `yaml:"timeout"`
	Verbose     bool                `yaml:"verbose"`
	Redis       RedisConfig         `yaml:"redis"`
}

// QueueConfig represents queue priority configuration
// Can be parsed as [name, weight] array or {name: name, weight: weight} map
type QueueConfig struct {
	Name     string `yaml:"name"`
	Weight   int    `yaml:"weight"`
	Priority int    `yaml:"priority"`
}

// UnmarshalYAML implements custom YAML unmarshaling for queue config
func (qc *QueueConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as array [name, weight]
	var arr []interface{}
	if err := unmarshal(&arr); err == nil && len(arr) >= 1 {
		if name, ok := arr[0].(string); ok {
			qc.Name = name
			if len(arr) >= 2 {
				if weight, ok := arr[1].(int); ok {
					qc.Weight = weight
				} else if weight, ok := arr[1].(float64); ok {
					qc.Weight = int(weight)
				}
			} else {
				qc.Weight = 1 // default weight
			}
			return nil
		}
	}

	// Fall back to map format
	var m map[string]interface{}
	if err := unmarshal(&m); err != nil {
		return err
	}

	if name, ok := m["name"].(string); ok {
		qc.Name = name
	}
	if weight, ok := m["weight"].(int); ok {
		qc.Weight = weight
	} else if weight, ok := m["weight"].(float64); ok {
		qc.Weight = int(weight)
	}
	if priority, ok := m["priority"].(int); ok {
		qc.Priority = priority
	} else if priority, ok := m["priority"].(float64); ok {
		qc.Priority = int(priority)
	}

	// Default weight if not set
	if qc.Weight == 0 {
		qc.Weight = 1
	}

	return nil
}

// RedisConfig represents Redis connection configuration
type RedisConfig struct {
	URL            string `yaml:"url"`
	NetworkTimeout int    `yaml:"network_timeout"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if config.Concurrency == 0 {
		config.Concurrency = 10
	}
	if config.Timeout == 0 {
		config.Timeout = 8
	}
	if config.Redis.URL == "" {
		config.Redis.URL = os.Getenv("REDIS_URL")
		if config.Redis.URL == "" {
			config.Redis.URL = "redis://localhost:6379/0"
		}
	}
	if config.Redis.NetworkTimeout == 0 {
		config.Redis.NetworkTimeout = 5
	}

	return &config, nil
}

// GetTimeout returns timeout as duration
func (c *Config) GetTimeout() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

// GetNetworkTimeout returns network timeout as duration
func (c *RedisConfig) GetNetworkTimeout() time.Duration {
	return time.Duration(c.NetworkTimeout) * time.Second
}


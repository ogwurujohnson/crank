package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents Sidekiq configuration
type Config struct {
	Concurrency int           `yaml:"concurrency"`
	Queues      []QueueConfig  `yaml:"queues"`
	Timeout     int            `yaml:"timeout"`
	Verbose     bool           `yaml:"verbose"`
	Redis       RedisConfig    `yaml:"redis"`
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
				qc.Weight = 1
			}
			return nil
		}
	}

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
	if qc.Weight == 0 {
		qc.Weight = 1
	}
	return nil
}

// RedisConfig represents Redis connection configuration
type RedisConfig struct {
	URL                   string `yaml:"url"`
	NetworkTimeout        int    `yaml:"network_timeout"`
	UseTLS                bool   `yaml:"use_tls"`
	TLSInsecureSkipVerify bool   `yaml:"tls_insecure_skip_verify"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Concurrency == 0 {
		cfg.Concurrency = 10
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 8
	}
	if cfg.Redis.URL == "" {
		cfg.Redis.URL = os.Getenv("REDIS_URL")
		if cfg.Redis.URL == "" {
			cfg.Redis.URL = "redis://localhost:6379/0"
		}
	}
	if cfg.Redis.NetworkTimeout == 0 {
		cfg.Redis.NetworkTimeout = 5
	}

	return &cfg, nil
}

// GetTimeout returns timeout as duration
func (c *Config) GetTimeout() time.Duration {
	return time.Duration(c.Timeout) * time.Second
}

// GetNetworkTimeout returns network timeout as duration
func (c *RedisConfig) GetNetworkTimeout() time.Duration {
	return time.Duration(c.NetworkTimeout) * time.Second
}

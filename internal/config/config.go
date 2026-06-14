package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Checker CheckerConfig `yaml:"checker"`
	Memory  MemoryConfig  `yaml:"memory"`
}

type ServerConfig struct {
	Port        int    `yaml:"port"`
	Mode        string `yaml:"mode"`
	CORSOrigins string `yaml:"cors_origins"`
}

type CheckerConfig struct {
	DefaultConcurrency int `yaml:"default_concurrency"`
	Timeout            int `yaml:"timeout"`
}

type PlatformRateConfig struct {
	Enabled              bool `json:"enabled"`
	Concurrency          int  `json:"concurrency"`
	RequestDelayMs       int  `json:"request_delay_ms"`
	MaxRequestsPerSecond int  `json:"max_requests_per_second"`
	CacheTTLHours        int  `json:"cache_ttl_hours"`
}

type MemoryConfig struct {
	HistoryTTLMinutes      int `yaml:"history_ttl_minutes" json:"history_ttl_minutes"`
	CleanupIntervalMinutes int `yaml:"cleanup_interval_minutes" json:"cleanup_interval_minutes"`
}

var AppConfig *Config

func Load(configPath string) error {
	setDefaults()

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}

		AppConfig = &Config{}
		if err := yaml.Unmarshal(data, AppConfig); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	} else {
		AppConfig = &Config{}
	}

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if port := viper.GetInt("SERVER_PORT"); port > 0 {
		AppConfig.Server.Port = port
	}
	if mode := viper.GetString("SERVER_MODE"); mode != "" {
		AppConfig.Server.Mode = mode
	}
	if corsOrigins := viper.GetString("SERVER_CORS_ORIGINS"); corsOrigins != "" {
		AppConfig.Server.CORSOrigins = corsOrigins
	}

	if concurrency := viper.GetInt("CHECKER_DEFAULT_CONCURRENCY"); concurrency > 0 {
		AppConfig.Checker.DefaultConcurrency = concurrency
	}
	if timeout := viper.GetInt("CHECKER_TIMEOUT"); timeout > 0 {
		AppConfig.Checker.Timeout = timeout
	}

	if historyTTLMinutes := viper.GetInt("MEMORY_HISTORY_TTL_MINUTES"); historyTTLMinutes > 0 {
		AppConfig.Memory.HistoryTTLMinutes = historyTTLMinutes
	}
	if cleanupIntervalMinutes := viper.GetInt("MEMORY_CLEANUP_INTERVAL_MINUTES"); cleanupIntervalMinutes > 0 {
		AppConfig.Memory.CleanupIntervalMinutes = cleanupIntervalMinutes
	}

	if AppConfig.Server.Port == 0 {
		AppConfig.Server.Port = 6080
	}
	if AppConfig.Server.Mode == "" {
		AppConfig.Server.Mode = "debug"
	}
	if AppConfig.Server.CORSOrigins == "" {
		AppConfig.Server.CORSOrigins = "*"
	}
	if AppConfig.Checker.DefaultConcurrency == 0 {
		AppConfig.Checker.DefaultConcurrency = 5
	}
	if AppConfig.Checker.Timeout == 0 {
		AppConfig.Checker.Timeout = 30
	}
	if AppConfig.Memory.HistoryTTLMinutes == 0 {
		AppConfig.Memory.HistoryTTLMinutes = 2880
	}
	if AppConfig.Memory.CleanupIntervalMinutes == 0 {
		AppConfig.Memory.CleanupIntervalMinutes = 10
	}

	return nil
}

func setDefaults() {
	viper.SetDefault("SERVER_PORT", 6080)
	viper.SetDefault("SERVER_MODE", "debug")
	viper.SetDefault("SERVER_CORS_ORIGINS", "*")
	viper.SetDefault("CHECKER_DEFAULT_CONCURRENCY", 5)
	viper.SetDefault("CHECKER_TIMEOUT", 30)
	viper.SetDefault("MEMORY_HISTORY_TTL_MINUTES", 2880)
	viper.SetDefault("MEMORY_CLEANUP_INTERVAL_MINUTES", 10)
}

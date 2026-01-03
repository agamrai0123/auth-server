package auth

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type (
	// Logging configuration
	logging struct {
		Level      int    `mapstructure:"level,omitempty"`
		Path       string `mapstructure:"path,omitempty"`
		MaxSizeMB  int    `mapstructure:"max_size_mb,omitempty"`
		MaxBackups int    `mapstructure:"max_backups,omitempty"`
		MaxAgeDays int    `mapstructure:"max_age_days,omitempty"`
		Compress   bool   `mapstructure:"compress,omitempty"`
	}

	// Database configuration (for future rqlite integration)
	database struct {
		Host    string `mapstructure:"host,omitempty"`
		Port    int    `mapstructure:"port,omitempty"`
		Timeout int    `mapstructure:"timeout_seconds,omitempty"`
	}

	// JWT configuration
	jwtConfig struct {
		SecretKey       string `mapstructure:"secret_key,omitempty"`
		AccessDuration  int    `mapstructure:"access_duration_minutes,omitempty"`
		RefreshDuration int    `mapstructure:"refresh_duration_hours,omitempty"`
	}

	// Server configuration
	configuration struct {
		Version     string    `mapstructure:"version,omitempty"`
		Logging     logging   `mapstructure:"logging"`
		ServerPort  string    `mapstructure:"server_port"`
		MetricPort  int       `mapstructure:"metric_port"`
		Database    database  `mapstructure:"database"`
		JWT         jwtConfig `mapstructure:"jwt"`
		Environment string    `mapstructure:"environment,omitempty"`
	}
)

var (
	AppConfig configuration
)

// ReadConfiguration reads the configuration from file
func ReadConfiguration() error {
	viper.SetConfigName("auth-server-config")
	viper.SetConfigType("json")

	// Try multiple config paths
	configPaths := []string{
		"./config",
		"../config",
		"../../config",
	}

	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}

	// Try to read config file
	err := viper.ReadInConfig()
	if err != nil {
		// Config file is optional, set defaults
		log.Warn().Err(err).Msg("configuration file not found, using defaults")
		setDefaults()
	}

	// Unmarshal configuration
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	// Validate required fields
	if err := validateConfiguration(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Set defaults for optional fields
	if err := applyDefaults(); err != nil {
		return err
	}

	return nil
}

func setDefaults() {
	viper.SetDefault("version", "1.0.0")
	viper.SetDefault("server_port", "8080")
	viper.SetDefault("metric_port", 9090)
	viper.SetDefault("environment", "development")
	viper.SetDefault("logging.level", -1)
	viper.SetDefault("logging.path", "./logs/auth-server.log")
	viper.SetDefault("logging.max_size_mb", 100)
	viper.SetDefault("logging.max_backups", 10)
	viper.SetDefault("logging.max_age_days", 14)
	viper.SetDefault("logging.compress", true)
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 1521)
	viper.SetDefault("database.timeout_seconds", 30)
	viper.SetDefault("jwt.access_duration_minutes", 15)
	viper.SetDefault("jwt.refresh_duration_hours", 24)
}

func validateConfiguration() error {
	if AppConfig.ServerPort == "" {
		return errors.New("server_port is required in configuration")
	}

	if AppConfig.Logging.Path == "" {
		return errors.New("logging.path is required in configuration")
	}

	if AppConfig.Logging.MaxSizeMB <= 0 {
		return errors.New("logging.max_size_mb must be greater than 0")
	}

	return nil
}

func applyDefaults() error {
	// Create logs directory if it doesn't exist
	if AppConfig.Logging.Path != "" {
		logDir := filepath.Dir(AppConfig.Logging.Path)
		if logDir != "." && logDir != "" {
			if err := createDirIfNotExists(logDir); err != nil {
				return fmt.Errorf("failed to create log directory: %w", err)
			}
		}
	}

	// Apply logging defaults
	if AppConfig.Logging.MaxBackups == 0 {
		AppConfig.Logging.MaxBackups = 10
	}
	if AppConfig.Logging.MaxAgeDays == 0 {
		AppConfig.Logging.MaxAgeDays = 14
	}

	// Apply database defaults
	if AppConfig.Database.Host == "" {
		AppConfig.Database.Host = "localhost"
	}
	if AppConfig.Database.Port == 0 {
		AppConfig.Database.Port = 1521
	}
	if AppConfig.Database.Timeout == 0 {
		AppConfig.Database.Timeout = 30
	}

	// Apply JWT defaults
	if AppConfig.JWT.AccessDuration == 0 {
		AppConfig.JWT.AccessDuration = 15
	}
	if AppConfig.JWT.RefreshDuration == 0 {
		AppConfig.JWT.RefreshDuration = 24
	}

	return nil
}

func createDirIfNotExists(dir string) error {
	if _, err := filepath.Abs(dir); err != nil {
		return err
	}

	// Directory creation will be handled by the logger's lumberjack
	return nil
}

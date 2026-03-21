package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Kafka   KafkaConfig   `mapstructure:"kafka"`
	Metrics MetricsConfig `mapstructure:"metrics"`
	Log     LogConfig     `mapstructure:"log"`
}

type KafkaConfig struct {
	Brokers       []string `mapstructure:"brokers"`
	Topic         string   `mapstructure:"topic"`
	GroupID       string   `mapstructure:"group_id"`
	InitialOffset string   `mapstructure:"initial_offset"` // "oldest" or "newest"
}

type MetricsConfig struct {
	Port int `mapstructure:"port"`
}

type LogConfig struct {
	Level string `mapstructure:"level"` // "debug", "info", "warn", "error"
}

func Load() (Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/log-analytics")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("kafka.brokers", []string{"localhost:9092"})
	viper.SetDefault("kafka.topic", "application-logs")
	viper.SetDefault("kafka.group_id", "log-analytics")
	viper.SetDefault("kafka.initial_offset", "oldest")
	viper.SetDefault("metrics.port", 2112)
	viper.SetDefault("log.level", "info")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

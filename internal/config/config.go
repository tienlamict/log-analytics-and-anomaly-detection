package config

import (
	"fmt"

	"github.com/log-analytics/server/internal/detection"
	"github.com/spf13/viper"
)

type Config struct {
	Kafka     KafkaConfig              `mapstructure:"kafka"`
	Metrics   MetricsConfig            `mapstructure:"metrics"`
	Log       LogConfig                `mapstructure:"log"`
	Detection detection.DetectionConfig `mapstructure:"detection"`
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

	// Detection defaults
	viper.SetDefault("detection.window_duration", "5m")
	viper.SetDefault("detection.eviction_interval", "1m")
	viper.SetDefault("detection.cooldown_duration", "15m")
	viper.SetDefault("detection.warmup_multiplier", 2)

	// Error rate rule
	viper.SetDefault("detection.rules.error_rate.enabled", true)
	viper.SetDefault("detection.rules.error_rate.threshold", 10)
	viper.SetDefault("detection.rules.error_rate.window", "5m")
	viper.SetDefault("detection.rules.error_rate.severity", "high")

	// Latency rule
	viper.SetDefault("detection.rules.latency.enabled", true)
	viper.SetDefault("detection.rules.latency.threshold_ms", 500)
	viper.SetDefault("detection.rules.latency.breach_rate_percent", 20)
	viper.SetDefault("detection.rules.latency.window", "5m")
	viper.SetDefault("detection.rules.latency.severity", "medium")

	// Repeated failure rule
	viper.SetDefault("detection.rules.repeated_failure.enabled", true)
	viper.SetDefault("detection.rules.repeated_failure.threshold", 5)
	viper.SetDefault("detection.rules.repeated_failure.window", "5m")
	viper.SetDefault("detection.rules.repeated_failure.severity", "medium")

	// Auth burst rule
	viper.SetDefault("detection.rules.auth_burst.enabled", true)
	viper.SetDefault("detection.rules.auth_burst.threshold", 10)
	viper.SetDefault("detection.rules.auth_burst.window", "5m")
	viper.SetDefault("detection.rules.auth_burst.ip_field", "source_ip")
	viper.SetDefault("detection.rules.auth_burst.user_field", "username")
	viper.SetDefault("detection.rules.auth_burst.severity", "high")

	// Off-hours rule
	viper.SetDefault("detection.rules.off_hours.enabled", true)
	viper.SetDefault("detection.rules.off_hours.business_hours_start", 9)
	viper.SetDefault("detection.rules.off_hours.business_hours_end", 17)
	viper.SetDefault("detection.rules.off_hours.timezone", "UTC")
	viper.SetDefault("detection.rules.off_hours.sensitive_paths", []string{"/admin", "/api/v1/users", "/internal"})
	viper.SetDefault("detection.rules.off_hours.severity", "medium")

	// Service silence rule
	viper.SetDefault("detection.rules.service_silence.enabled", true)
	viper.SetDefault("detection.rules.service_silence.silence_after", "5m")
	viper.SetDefault("detection.rules.service_silence.check_interval", "30s")
	viper.SetDefault("detection.rules.service_silence.min_log_count", 5)
	viper.SetDefault("detection.rules.service_silence.severity", "critical")

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

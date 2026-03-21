// Package detection provides the anomaly detection engine and rule infrastructure.
package detection

import "time"

// DetectionConfig holds top-level detection settings and per-rule configurations.
type DetectionConfig struct {
	WindowDuration   time.Duration        `mapstructure:"window_duration"`
	EvictionInterval time.Duration        `mapstructure:"eviction_interval"`
	CooldownDuration time.Duration        `mapstructure:"cooldown_duration"`
	WarmupMultiplier int                  `mapstructure:"warmup_multiplier"`
	Rules            DetectionRulesConfig `mapstructure:"rules"`
}

// DetectionRulesConfig groups all per-rule configuration structs.
type DetectionRulesConfig struct {
	ErrorRate       ErrorRateConfig       `mapstructure:"error_rate"`
	Latency         LatencyConfig         `mapstructure:"latency"`
	RepeatedFailure RepeatedFailureConfig `mapstructure:"repeated_failure"`
	AuthBurst       AuthBurstConfig       `mapstructure:"auth_burst"`
	OffHours        OffHoursConfig        `mapstructure:"off_hours"`
	ServiceSilence  ServiceSilenceConfig  `mapstructure:"service_silence"`
}

// ErrorRateConfig configures the error rate spike detector.
type ErrorRateConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold int           `mapstructure:"threshold"`
	Window    time.Duration `mapstructure:"window"`
	Severity  string        `mapstructure:"severity"`
}

// LatencyConfig configures the latency threshold breach detector.
type LatencyConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	ThresholdMs       int           `mapstructure:"threshold_ms"`
	BreachRatePercent int           `mapstructure:"breach_rate_percent"`
	Window            time.Duration `mapstructure:"window"`
	Severity          string        `mapstructure:"severity"`
}

// RepeatedFailureConfig configures the repeated failure within a window detector.
type RepeatedFailureConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold int           `mapstructure:"threshold"`
	Window    time.Duration `mapstructure:"window"`
	Severity  string        `mapstructure:"severity"`
}

// AuthBurstConfig configures the authentication failure burst detector.
type AuthBurstConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Threshold int           `mapstructure:"threshold"`
	Window    time.Duration `mapstructure:"window"`
	IPField   string        `mapstructure:"ip_field"`
	UserField string        `mapstructure:"user_field"`
	Severity  string        `mapstructure:"severity"`
}

// OffHoursConfig configures the off-hours access pattern detector.
type OffHoursConfig struct {
	Enabled            bool     `mapstructure:"enabled"`
	BusinessHoursStart int      `mapstructure:"business_hours_start"`
	BusinessHoursEnd   int      `mapstructure:"business_hours_end"`
	Timezone           string   `mapstructure:"timezone"`
	SensitivePaths     []string `mapstructure:"sensitive_paths"`
	Severity           string   `mapstructure:"severity"`
}

// ServiceSilenceConfig configures the service silence (heartbeat loss) detector.
type ServiceSilenceConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	SilenceAfter  time.Duration `mapstructure:"silence_after"`
	CheckInterval time.Duration `mapstructure:"check_interval"`
	MinLogCount   int           `mapstructure:"min_log_count"`
	Severity      string        `mapstructure:"severity"`
}

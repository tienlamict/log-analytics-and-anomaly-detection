package main

import (
	"fmt"
	"os"

	"github.com/log-analytics/server/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("config loaded: kafka brokers=%v topic=%s\n", cfg.Kafka.Brokers, cfg.Kafka.Topic)
}

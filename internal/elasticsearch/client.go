package elasticsearch

import (
	"net/http"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
)

// ClientConfig holds configuration for constructing an Elasticsearch TypedClient.
type ClientConfig struct {
	Addresses       []string
	Username        string
	Password        string
	MaxIdleConns    int
	ResponseTimeout time.Duration
}

// NewClient constructs an *elasticsearch.TypedClient with a bounded connection pool
// and retry logic for transient HTTP errors.
func NewClient(cfg ClientConfig) (*elasticsearch.TypedClient, error) {
	maxIdle := cfg.MaxIdleConns
	if maxIdle == 0 {
		maxIdle = 10
	}

	responseTimeout := cfg.ResponseTimeout
	if responseTimeout == 0 {
		responseTimeout = 10 * time.Second
	}

	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: cfg.Addresses,
		Username:  cfg.Username,
		Password:  cfg.Password,
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   maxIdle,
			ResponseHeaderTimeout: responseTimeout,
		},
		RetryOnStatus: []int{502, 503, 504, 429},
		MaxRetries:    3,
	})
}

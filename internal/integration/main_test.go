//go:build integration

package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/testcontainers/testcontainers-go"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/goleak"
)

var (
	kafkaContainer *tckafka.KafkaContainer
	esContainer    testcontainers.Container
	kafkaBrokers   []string
	esAddress      string

	// liveMode is true when LIVE_KAFKA_BROKERS and LIVE_ES_ADDRESS are both set.
	// In live mode the test skips testcontainers and targets the running Docker stack.
	liveMode bool
)

func TestMain(m *testing.M) {
	liveBrokers := os.Getenv("LIVE_KAFKA_BROKERS")
	liveES := os.Getenv("LIVE_ES_ADDRESS")
	liveMode = liveBrokers != "" && liveES != ""

	if liveMode {
		kafkaBrokers = strings.Split(liveBrokers, ",")
		esAddress = liveES
		goleak.VerifyTestMain(m,
			goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
			goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
		)
		return
	}

	ctx := context.Background()

	var err error
	kafkaContainer, err = tckafka.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
		tckafka.WithClusterID("test-cluster"),
	)
	if err != nil {
		log.Fatalf("failed to start kafka container: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(kafkaContainer); err != nil {
			log.Printf("terminate kafka: %s", err)
		}
	}()

	kafkaBrokers, err = kafkaContainer.Brokers(ctx)
	if err != nil {
		log.Fatalf("failed to get kafka brokers: %s", err)
	}

	// Use GenericContainer directly so we fully control the wait strategy.
	// tces.Run (the Elasticsearch module) overrides WithWaitStrategy after user opts,
	// which caused a 60-second timeout even when security was disabled.
	esContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "docker.elastic.co/elasticsearch/elasticsearch:9.0.0",
			Env: map[string]string{
				"xpack.security.enabled":          "false",
				"xpack.security.http.ssl.enabled": "false",
				"discovery.type":                  "single-node",
				"ES_JAVA_OPTS":                    "-Xms512m -Xmx512m",
			},
			ExposedPorts: []string{"9200/tcp"},
			WaitingFor: wait.ForHTTP("/").
				WithPort("9200/tcp").
				WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		log.Fatalf("failed to start elasticsearch container: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(esContainer); err != nil {
			log.Printf("terminate es: %s", err)
		}
	}()

	esHost, err := esContainer.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get elasticsearch host: %s", err)
	}
	esPort, err := esContainer.MappedPort(ctx, "9200/tcp")
	if err != nil {
		log.Fatalf("failed to get elasticsearch port: %s", err)
	}
	esAddress = fmt.Sprintf("http://%s:%s", esHost, esPort.Port())

	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)
}

// newESClient builds a TypedClient pointed at the active Elasticsearch.
// Both live and isolated modes connect over plain HTTP without authentication.
func newESClient() (*elasticsearch.TypedClient, error) {
	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{esAddress},
	})
}

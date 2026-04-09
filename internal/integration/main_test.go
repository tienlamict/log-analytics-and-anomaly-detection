//go:build integration

package integration

import (
	"context"
	"log"
	"os"
	"strings"
	"testing"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/testcontainers/testcontainers-go"
	tces "github.com/testcontainers/testcontainers-go/modules/elasticsearch"
	tckafka "github.com/testcontainers/testcontainers-go/modules/kafka"
	"go.uber.org/goleak"
)

var (
	kafkaContainer *tckafka.KafkaContainer
	esContainer    *tces.ElasticsearchContainer
	kafkaBrokers   []string
	esAddress      string
	esCACert       []byte

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

	esContainer, err = tces.Run(ctx,
		"docker.elastic.co/elasticsearch/elasticsearch:8.9.0",
		tces.WithPassword("changeme"),
	)
	if err != nil {
		log.Fatalf("failed to start elasticsearch container: %s", err)
	}
	defer func() {
		if err := testcontainers.TerminateContainer(esContainer); err != nil {
			log.Printf("terminate es: %s", err)
		}
	}()

	esAddress = esContainer.Settings.Address
	esCACert = esContainer.Settings.CACert

	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"),
	)
}

// newESClient builds a TypedClient pointed at the active Elasticsearch.
// In live mode it connects unauthenticated (matching config.docker.yaml).
// In isolated mode it uses the TLS cert and credentials from the test container.
func newESClient() (*elasticsearch.TypedClient, error) {
	if liveMode {
		return elasticsearch.NewTypedClient(elasticsearch.Config{
			Addresses: []string{esAddress},
		})
	}
	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{esAddress},
		Username:  "elastic",
		Password:  "changeme",
		CACert:    esCACert,
	})
}

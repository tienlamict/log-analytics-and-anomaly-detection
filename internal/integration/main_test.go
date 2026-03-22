//go:build integration

package integration

import (
	"context"
	"log"
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
)

func TestMain(m *testing.M) {
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

// newESClient builds a TypedClient from the container settings.
// This bypasses the project's NewClient to use CACert for TLS.
func newESClient() (*elasticsearch.TypedClient, error) {
	return elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{esAddress},
		Username:  "elastic",
		Password:  "changeme",
		CACert:    esCACert,
	})
}

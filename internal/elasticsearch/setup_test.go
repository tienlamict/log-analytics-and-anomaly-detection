package elasticsearch

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/stretchr/testify/require"
)

// roundTripFunc is a function type that implements http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// successResponse returns a 200 OK response with the Elasticsearch product header.
func successResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"X-Elastic-Product": []string{"Elasticsearch"},
			"Content-Type":      []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"acknowledged":true}`)),
	}
}

// errorResponse returns a 500 response with the Elasticsearch product header.
func errorResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusInternalServerError,
		Header: http.Header{
			"X-Elastic-Product": []string{"Elasticsearch"},
			"Content-Type":      []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(`{"error":{"type":"internal_server_error","reason":"test error"}}`)),
	}
}

func TestApplyIndexTemplates_Success(t *testing.T) {
	var calledURLs []string

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calledURLs = append(calledURLs, req.URL.Path)
		return successResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	err = ApplyIndexTemplates(context.Background(), client)
	require.NoError(t, err)

	// Verify both templates were applied
	var logsFound, anomaliesFound bool
	for _, url := range calledURLs {
		if strings.Contains(url, "logs-template") {
			logsFound = true
		}
		if strings.Contains(url, "anomalies-template") {
			anomaliesFound = true
		}
	}
	require.True(t, logsFound, "expected logs-template PUT request, got URLs: %v", calledURLs)
	require.True(t, anomaliesFound, "expected anomalies-template PUT request, got URLs: %v", calledURLs)
}

func TestApplyIndexTemplates_LogsTemplateFailure(t *testing.T) {
	callCount := 0

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		return errorResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	err = ApplyIndexTemplates(context.Background(), client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "logs")
}

func TestApplyIndexTemplates_AnomaliesTemplateFailure(t *testing.T) {
	callCount := 0

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if callCount == 1 {
			// First call (logs template) succeeds
			return successResponse(), nil
		}
		// Second call (anomalies template) fails
		return errorResponse(), nil
	})

	client, err := elasticsearch.NewTypedClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
		Transport: transport,
	})
	require.NoError(t, err)

	err = ApplyIndexTemplates(context.Background(), client)
	require.Error(t, err)
	require.Contains(t, err.Error(), "anomalies")
}

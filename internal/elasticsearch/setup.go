package elasticsearch

import (
	"context"
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/indices/putindextemplate"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/dynamicmapping"
)

// ApplyIndexTemplates applies both logs-* and anomalies index templates to Elasticsearch.
// It calls applyLogsTemplate then applyAnomaliesTemplate, returning the first error encountered.
// If ES is unreachable, an error is returned immediately (fail-fast).
func ApplyIndexTemplates(ctx context.Context, client *elasticsearch.TypedClient) error {
	if err := applyLogsTemplate(ctx, client); err != nil {
		return err
	}
	return applyAnomaliesTemplate(ctx, client)
}

func applyLogsTemplate(ctx context.Context, client *elasticsearch.TypedClient) error {
	falseMapping := dynamicmapping.False
	trueVal := true
	priority := int64(100)

	req := &putindextemplate.Request{
		IndexPatterns: []string{"logs-*"},
		Priority:      &priority,
		Template: &types.IndexTemplateMapping{
			Mappings: &types.TypeMapping{
				Dynamic: &falseMapping,
				Properties: map[string]types.Property{
					"@timestamp": types.DateProperty{},
					"level":      types.KeywordProperty{},
					"service":    types.KeywordProperty{},
					"message":    types.TextProperty{},
					"fields":     types.FlattenedProperty{},
					"raw_source": types.KeywordProperty{Index: &trueVal},
				},
			},
		},
	}

	_, err := client.Indices.PutIndexTemplate("logs-template").Request(req).Do(ctx)
	if err != nil {
		return fmt.Errorf("apply logs template: %w", err)
	}
	return nil
}

func applyAnomaliesTemplate(ctx context.Context, client *elasticsearch.TypedClient) error {
	falseMapping := dynamicmapping.False
	priority := int64(100)

	req := &putindextemplate.Request{
		IndexPatterns: []string{"anomalies"},
		Priority:      &priority,
		Template: &types.IndexTemplateMapping{
			Mappings: &types.TypeMapping{
				Dynamic: &falseMapping,
				Properties: map[string]types.Property{
					"id":          types.KeywordProperty{},
					"rule_id":     types.KeywordProperty{},
					"severity":    types.KeywordProperty{},
					"service":     types.KeywordProperty{},
					"description": types.TextProperty{},
					"detected_at": types.DateProperty{},
					"evidence":    types.FlattenedProperty{},
				},
			},
		},
	}

	_, err := client.Indices.PutIndexTemplate("anomalies-template").Request(req).Do(ctx)
	if err != nil {
		return fmt.Errorf("apply anomalies template: %w", err)
	}
	return nil
}

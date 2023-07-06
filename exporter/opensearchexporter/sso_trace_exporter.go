// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

type SSOTracesExporter struct {
	logger *zap.Logger

	maxAttempts int

	client      *osClientCurrent
	bulkIndexer osBulkIndexerCurrent
	Namespace   string
	Dataset     string
}

func (s SSOTracesExporter) Shutdown(ctx context.Context) error {
	return s.bulkIndexer.Close(ctx)

}

func (s SSOTracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			spans := scopeSpans.At(j).Spans()
			scope := scopeSpans.At(j).Scope()
			schemaURL := scopeSpans.At(j).SchemaUrl()
			for k := 0; k < spans.Len(); k++ {
				if err := s.pushTraceRecord(ctx, resource, scope, schemaURL, spans.At(k)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	return multierr.Combine(errs...)
}

func defaultIfEmpty(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func (s SSOTracesExporter) pushTraceRecord(
	ctx context.Context,
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	schemaURL string,
	span ptrace.Span,
) error {
	sso := SSOSpan{}
	sso.Attributes = span.Attributes().AsRaw()
	sso.DroppedAttributesCount = span.DroppedAttributesCount()
	sso.DroppedEventsCount = span.DroppedEventsCount()
	sso.DroppedLinksCount = span.DroppedLinksCount()
	sso.EndTime = span.EndTimestamp().AsTime()
	sso.Kind = span.Kind().String()
	sso.Name = span.Name()
	sso.ParentSpanID = span.ParentSpanID().String()
	sso.Resource = resource.Attributes().AsRaw()
	sso.SpanID = span.SpanID().String()
	sso.StartTime = span.StartTimestamp().AsTime()
	sso.Status.Code = span.Status().Code().String()
	sso.Status.Message = span.Status().Message()
	sso.TraceID = span.TraceID().String()
	sso.TraceState = span.TraceState().AsRaw()

	if span.Events().Len() > 0 {
		sso.Events = make([]SSOSpanEvent, span.Events().Len())
		for i := 0; i < span.Events().Len(); i++ {
			e := span.Events().At(i)
			ssoEvent := &sso.Events[i]
			ssoEvent.Attributes = e.Attributes().AsRaw()
			ssoEvent.DroppedAttributesCount = e.DroppedAttributesCount()
			ssoEvent.Name = e.Name()
			ts := e.Timestamp().AsTime()
			if ts.Unix() != 0 {
				ssoEvent.Timestamp = &ts
			} else {
				now := time.Now()
				ssoEvent.ObservedTimestamp = &now
			}
		}
	}

	dataStream := DataStream{}
	if s.Dataset != "" {
		dataStream.Dataset = s.Dataset
	}

	if s.Namespace != "" {
		dataStream.Namespace = s.Namespace
	}

	if dataStream != (DataStream{}) {
		dataStream.Type = "span"
		sso.Attributes["data_stream"] = dataStream
	}

	sso.InstrumentationScope.Name = scope.Name()
	sso.InstrumentationScope.DroppedAttributesCount = scope.DroppedAttributesCount()
	sso.InstrumentationScope.Version = scope.Version()
	sso.InstrumentationScope.SchemaURL = schemaURL
	sso.InstrumentationScope.Attributes = scope.Attributes().AsRaw()

	if span.Links().Len() > 0 {
		sso.Links = make([]SSOSpanLinks, span.Links().Len())
		for i := 0; i < span.Links().Len(); i++ {
			link := span.Links().At(i)
			ssoLink := &sso.Links[i]
			ssoLink.Attributes = link.Attributes().AsRaw()
			ssoLink.DroppedAttributesCount = link.DroppedAttributesCount()
			ssoLink.TraceID = link.TraceID().String()
			ssoLink.TraceState = link.TraceState().AsRaw()
			ssoLink.SpanID = link.SpanID().String()
		}
	}
	payload, _ := json.Marshal(sso)

	// construct destination index name by combining Dataset and Namespace options if they are set.
	index := strings.Join([]string{"sso_traces", defaultIfEmpty(s.Dataset, "default"), defaultIfEmpty(s.Namespace, "namespace")}, "-")
	return pushDocuments(ctx, s.logger, index, payload, s.bulkIndexer, s.maxAttempts)

}

func newSSOTracesExporter(logger *zap.Logger, cfg *Config) (*SSOTracesExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client, err := newOpenSearchClient(logger, cfg)
	if err != nil {
		return nil, err
	}

	bulkIndexer, err := newBulkIndexer(logger, client, cfg)
	if err != nil {
		return nil, err
	}

	maxAttempts := getMaxAttempts(cfg)
	return &SSOTracesExporter{
		logger:      logger,
		maxAttempts: maxAttempts,
		client:      client,
		bulkIndexer: bulkIndexer,
		Namespace:   cfg.Namespace,
		Dataset:     cfg.Dataset,
	}, nil
}

func getMaxAttempts(cfg *Config) int {
	if cfg.Retry.Enabled {
		return cfg.Retry.MaxRequests
	}
	return 1
}

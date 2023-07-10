// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "opensearch"
	// The stability level of the exporter.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for OpenSearch exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: HTTPClientSettings{
			Timeout: 90 * time.Second,
		},
		Namespace: "",
		Dataset:   "",
		Retry: RetrySettings{
			Enabled:         true,
			MaxRequests:     3,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     1 * time.Minute,
		},
	}
}

func createTracesExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	tracesExporter, err := newTracesExporter(set.Logger, cfg.(*Config))
	if err != nil {
		return nil, fmt.Errorf("cannot configure OpenSearch traces tracesExporter: %w", err)
	}
	return exporterhelper.NewTracesExporter(ctx, set, cfg, tracesExporter.pushTraceData,
		exporterhelper.WithShutdown(tracesExporter.Shutdown))
}

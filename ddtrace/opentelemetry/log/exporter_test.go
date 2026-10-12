// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

package log

import (
	"testing"
	"time"

	"github.com/DataDog/dd-trace-go/v2/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

func TestNewOTLPExporterProtocol(t *testing.T) {
	t.Cleanup(func() { config.CreateNew() })
	for protocol, expected := range map[string]sdklog.Exporter{
		"":        &otlploghttp.Exporter{},
		"grpc":    &otlploggrpc.Exporter{},
		"unknown": &otlploghttp.Exporter{},
	} {
		t.Run(protocol, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_LOGS_PROTOCOL", protocol)
			config.CreateNew()
			exporter, err := newOTLPExporter(t.Context(), nil, nil)
			require.NoError(t, err)
			assert.IsType(t, expected, exporter.(*telemetryExporter).Exporter)
		})
	}
}

func TestResolveOTLPEndpointHTTP(t *testing.T) {
	t.Run("defaults to localhost:4318", func(t *testing.T) {
		endpoint, path, insecure := resolveOTLPEndpointHTTP()
		assert.Equal(t, "localhost:4318", endpoint)
		assert.Equal(t, "/v1/logs", path)
		assert.True(t, insecure)
	})

	t.Run("uses DD_AGENT_HOST", func(t *testing.T) {
		t.Setenv("DD_AGENT_HOST", "agent.example.com")
		endpoint, path, insecure := resolveOTLPEndpointHTTP()
		assert.Equal(t, "agent.example.com:4318", endpoint)
		assert.Equal(t, "/v1/logs", path)
		assert.True(t, insecure)
	})

	t.Run("uses DD_TRACE_AGENT_URL", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "http://trace-agent:8126")
		endpoint, path, insecure := resolveOTLPEndpointHTTP()
		assert.Equal(t, "trace-agent:4318", endpoint)
		assert.Equal(t, "/v1/logs", path)
		assert.True(t, insecure)
	})

	t.Run("DD_TRACE_AGENT_URL wins over DD_AGENT_HOST", func(t *testing.T) {
		t.Setenv("DD_AGENT_HOST", "agent-host")
		t.Setenv("DD_TRACE_AGENT_URL", "http://trace-agent:8126")
		endpoint, _, _ := resolveOTLPEndpointHTTP()
		assert.Equal(t, "trace-agent:4318", endpoint)
	})

	t.Run("preserves https scheme", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "https://secure-agent:8126")
		_, _, insecure := resolveOTLPEndpointHTTP()
		assert.False(t, insecure, "https should result in insecure=false")
	})

	t.Run("handles unix socket scheme", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "unix:///var/run/datadog/apm.socket")
		_, _, insecure := resolveOTLPEndpointHTTP()
		assert.True(t, insecure, "unix scheme should result in insecure=true")
	})

	t.Run("handles IPv6 addresses", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "http://[::1]:8126")
		endpoint, _, _ := resolveOTLPEndpointHTTP()
		assert.Equal(t, "[::1]:4318", endpoint)
	})
}

func TestResolveOTLPEndpointGRPC(t *testing.T) {
	t.Run("defaults to localhost:4317", func(t *testing.T) {
		endpoint, insecure := resolveOTLPEndpointGRPC()
		assert.Equal(t, "localhost:4317", endpoint)
		assert.True(t, insecure)
	})

	t.Run("uses DD_AGENT_HOST", func(t *testing.T) {
		t.Setenv("DD_AGENT_HOST", "agent.example.com")
		endpoint, insecure := resolveOTLPEndpointGRPC()
		assert.Equal(t, "agent.example.com:4317", endpoint)
		assert.True(t, insecure)
	})

	t.Run("uses DD_TRACE_AGENT_URL", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "http://trace-agent:8126")
		endpoint, insecure := resolveOTLPEndpointGRPC()
		assert.Equal(t, "trace-agent:4317", endpoint)
		assert.True(t, insecure)
	})

	t.Run("DD_TRACE_AGENT_URL wins over DD_AGENT_HOST", func(t *testing.T) {
		t.Setenv("DD_AGENT_HOST", "agent-host")
		t.Setenv("DD_TRACE_AGENT_URL", "http://trace-agent:8126")
		endpoint, _ := resolveOTLPEndpointGRPC()
		assert.Equal(t, "trace-agent:4317", endpoint)
	})

	t.Run("preserves https scheme", func(t *testing.T) {
		t.Setenv("DD_TRACE_AGENT_URL", "https://secure-agent:8126")
		_, insecure := resolveOTLPEndpointGRPC()
		assert.False(t, insecure, "https should result in insecure=false")
	})
}

func TestResolveExportTimeout(t *testing.T) {
	t.Run("defaults to 30 seconds", func(t *testing.T) {
		timeout := resolveExportTimeout()
		assert.Equal(t, 30*time.Second, timeout)
	})

	t.Run("uses OTEL_EXPORTER_OTLP_TIMEOUT", func(t *testing.T) {
		t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", "5000")
		timeout := resolveExportTimeout()
		assert.Equal(t, 5*time.Second, timeout)
	})

	t.Run("OTEL_EXPORTER_OTLP_LOGS_TIMEOUT wins over generic", func(t *testing.T) {
		t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", "5000")
		t.Setenv("OTEL_EXPORTER_OTLP_LOGS_TIMEOUT", "10000")
		timeout := resolveExportTimeout()
		assert.Equal(t, 10*time.Second, timeout)
	})

	t.Run("falls back to default on invalid value", func(t *testing.T) {
		t.Setenv("OTEL_EXPORTER_OTLP_LOGS_TIMEOUT", "invalid")
		timeout := resolveExportTimeout()
		assert.Equal(t, 30*time.Second, timeout)
	})
}

func TestParseTimeout(t *testing.T) {
	t.Run("parses milliseconds", func(t *testing.T) {
		timeout, err := parseTimeout("1000")
		assert.NoError(t, err)
		assert.Equal(t, time.Second, timeout)
	})

	t.Run("handles zero", func(t *testing.T) {
		timeout, err := parseTimeout("0")
		assert.NoError(t, err)
		assert.Equal(t, time.Duration(0), timeout)
	})

	t.Run("returns error for invalid input", func(t *testing.T) {
		_, err := parseTimeout("invalid")
		assert.Error(t, err)
	})

	t.Run("returns error for float", func(t *testing.T) {
		_, err := parseTimeout("1000.5")
		assert.Error(t, err)
	})
}

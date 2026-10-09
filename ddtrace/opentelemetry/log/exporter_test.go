// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2025 Datadog, Inc.

package log

import (
	"testing"

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

func TestSanitizeOTLPEndpoint(t *testing.T) {
	for _, tc := range []struct {
		name      string
		raw       string
		isGeneric bool
		expected  string
	}{
		{name: "generic no path", raw: "http://collector:4318", isGeneric: true, expected: "http://collector:4318/v1/logs"},
		{name: "generic trailing slash", raw: "http://collector:4318/", isGeneric: true, expected: "http://collector:4318/v1/logs"},
		{name: "generic custom path", raw: "http://collector:4318/otlp", isGeneric: true, expected: "http://collector:4318/otlp/v1/logs"},
		{name: "signal-specific path", raw: "http://collector:4318/custom/", expected: "http://collector:4318/custom/"},
		{name: "invalid", raw: "http://[::1", expected: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeOTLPEndpoint(tc.raw, tc.isGeneric))
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

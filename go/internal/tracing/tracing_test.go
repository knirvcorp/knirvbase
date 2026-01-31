package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
)

func TestInitTracer(t *testing.T) {
	// Test with invalid endpoint (this should fail gracefully)
	tp, err := InitTracer("test-service", "http://invalid-endpoint:14268/api/traces")
	// The tracer provider should be created even if the endpoint is invalid
	if tp == nil {
		t.Error("Expected TracerProvider to be created")
	}
	// Note: err might be nil initially, connection errors happen during export
	_ = err // Ignore error for this test
}

func TestStartSpan(t *testing.T) {
	// Initialize a tracer provider (even if it fails to connect)
	tp, _ := InitTracer("test-service", "http://localhost:14268/api/traces")
	if tp != nil {
		defer tp.Shutdown(context.Background())
	}

	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-operation",
		attribute.String("test.key", "test.value"))

	if newCtx == nil {
		t.Error("Expected non-nil context")
	}
	if span == nil {
		t.Error("Expected non-nil span")
	}

	// End the span
	span.End()
}

func TestStartSpanWithAttributes(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-operation-with-attrs",
		attribute.String("service", "test"),
		attribute.Int("count", 42))

	if newCtx == nil {
		t.Error("Expected non-nil context")
	}
	if span == nil {
		t.Error("Expected non-nil span")
	}

	span.End()
}

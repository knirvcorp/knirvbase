package logging

import (
	"errors"
	"testing"
)

func TestNewLogger(t *testing.T) {
	logger, err := NewLogger("info", "json")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	if logger == nil {
		t.Fatal("Expected Logger, got nil")
	}
	if logger.Logger == nil {
		t.Error("Expected zap.Logger to be initialized")
	}
}

func TestNewLoggerInvalidLevel(t *testing.T) {
	_, err := NewLogger("invalid", "json")
	if err == nil {
		t.Error("Expected error for invalid log level")
	}
}

func TestNewLoggerConsoleFormat(t *testing.T) {
	logger, err := NewLogger("debug", "console")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	if logger == nil {
		t.Fatal("Expected Logger, got nil")
	}
}

func TestWithBlockID(t *testing.T) {
	logger, _ := NewLogger("info", "json")
	blockLogger := logger.WithBlockID("test-block-123")

	if blockLogger == nil {
		t.Error("Expected logger with block ID, got nil")
	}

	// The logger should have the block_id field set
	// We can't easily test the actual logging output without capturing it,
	// but we can verify the method doesn't panic and returns a logger
}

func TestWithUserID(t *testing.T) {
	logger, _ := NewLogger("info", "json")
	userLogger := logger.WithUserID("user-456")

	if userLogger == nil {
		t.Error("Expected logger with user ID, got nil")
	}
}

func TestWithError(t *testing.T) {
	logger, _ := NewLogger("info", "json")
	testErr := errors.New("test error")
	errorLogger := logger.WithError(testErr)

	if errorLogger == nil {
		t.Error("Expected logger with error, got nil")
	}
}
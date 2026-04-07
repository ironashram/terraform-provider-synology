package provider

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-log/tflogtest"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestSynoLogger_Debug(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	logger.Debug("test debug message", "key", "value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestSynoLogger_Info(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	logger.Info("test info message", "key", "value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestSynoLogger_Warn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	logger.Warn("test warn message", "key", "value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestSynoLogger_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	logger.Error("test error message", "key", "value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestSynoLogger_Printf(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	logger.Printf("formatted %s %d", "message", 42)

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}
}

func TestSynoLogger_OddKeysAndValues(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	// Odd number of keysAndValues should log an error instead of panicking.
	logger.Debug("odd keys", "key_without_value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected error log entry for odd keysAndValues")
	}
}

func TestSynoLogger_NonStringKey(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	logger := NewLogger(ctx)

	// Non-string key should log an error instead of panicking.
	logger.Info("non-string key", 123, "value")

	entries, err := tflogtest.MultilineJSONDecode(&buf)
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected error log entry for non-string key")
	}
}

func TestSynoLogger_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	ctx = tflog.SetField(ctx, "synology_host", "nas.local")
	ctx = tflog.SetField(ctx, "synology_password", "secret")
	// Masking causes ApplyMask to write to the shared Fields map in-place,
	// which triggers the race when logging is called concurrently.
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "synology_password")

	logger := NewLogger(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logger.Debug(
				"performing request",
				"method", "GET",
				"url", "https://nas.local/webapi",
			)
		}()
	}
	wg.Wait()
}

func TestSynoLogger_ConcurrentMixedLevels(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := tflogtest.RootLogger(context.Background(), &buf)
	ctx = tflog.SetField(ctx, "synology_host", "nas.local")
	ctx = tflog.SetField(ctx, "synology_password", "secret")
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "synology_password")

	logger := NewLogger(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(5)
		go func() {
			defer wg.Done()
			logger.Debug("debug msg", "iter", i)
		}()
		go func() {
			defer wg.Done()
			logger.Info("info msg", "iter", i)
		}()
		go func() {
			defer wg.Done()
			logger.Warn("warn msg", "iter", i)
		}()
		go func() {
			defer wg.Done()
			logger.Error("error msg", "iter", i)
		}()
		go func() {
			defer wg.Done()
			logger.Printf("printf msg %d", i)
		}()
	}
	wg.Wait()
}

package provider

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	subsystem = "api-client"
)

type SynoLogger struct {
	ctx context.Context
	mu  sync.Mutex
}

func NewLogger(ctx context.Context) *SynoLogger {
	return &SynoLogger{
		ctx: tflog.NewSubsystem(ctx, subsystem),
	}
}

// log factors the mutex lock/unlock boilerplate into one place.
func (l *SynoLogger) log(fn func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fn()
}

func (l *SynoLogger) Error(msg string, keysAndValues ...any) {
	l.log(func() {
		fields, err := convertToFields(keysAndValues)
		if err != nil {
			tflog.SubsystemError(
				l.ctx,
				subsystem,
				fmt.Sprintf("invalid log key-value pairs: %s", err),
			)
		}
		tflog.SubsystemError(l.ctx, subsystem, msg, fields)
	})
}

func (l *SynoLogger) Printf(format string, v ...any) {
	l.log(func() {
		tflog.SubsystemInfo(l.ctx, subsystem, fmt.Sprintf(format, v...))
	})
}

func (l *SynoLogger) Info(msg string, keysAndValues ...any) {
	l.log(func() {
		fields, err := convertToFields(keysAndValues)
		if err != nil {
			tflog.SubsystemError(
				l.ctx,
				subsystem,
				fmt.Sprintf("invalid log key-value pairs: %s", err),
			)
		}
		tflog.SubsystemInfo(l.ctx, subsystem, msg, fields)
	})
}

func (l *SynoLogger) Debug(msg string, keysAndValues ...any) {
	l.log(func() {
		fields, err := convertToFields(keysAndValues)
		if err != nil {
			tflog.SubsystemError(
				l.ctx,
				subsystem,
				fmt.Sprintf("invalid log key-value pairs: %s", err),
			)
		}
		tflog.SubsystemDebug(l.ctx, subsystem, msg, fields)
	})
}

func (l *SynoLogger) Warn(msg string, keysAndValues ...any) {
	l.log(func() {
		fields, err := convertToFields(keysAndValues)
		if err != nil {
			tflog.SubsystemError(
				l.ctx,
				subsystem,
				fmt.Sprintf("invalid log key-value pairs: %s", err),
			)
		}
		tflog.SubsystemWarn(l.ctx, subsystem, msg, fields)
	})
}

func convertToFields(keysAndValues []any) (map[string]any, error) {
	additionalFields := make(map[string]any, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			return nil, fmt.Errorf("missing value for key %s", keysAndValues[i])
		}

		if key, ok := keysAndValues[i].(string); ok {
			additionalFields[key] = keysAndValues[i+1]
		} else {
			return nil, fmt.Errorf("key %v is not a string", keysAndValues[i])
		}
	}
	return additionalFields, nil
}

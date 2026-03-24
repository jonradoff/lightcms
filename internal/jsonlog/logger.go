package jsonlog

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a logging severity level.
type Level int8

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger is a structured JSON logger.
type Logger struct {
	out      io.Writer
	minLevel Level
	mu       sync.Mutex
}

// New creates a new Logger writing to out at the given minimum level.
func New(out io.Writer, minLevel Level) *Logger {
	return &Logger{out: out, minLevel: minLevel}
}

// Default is a package-level logger writing JSON to stdout at LevelInfo.
var Default = New(os.Stdout, LevelInfo)

// Info logs a message at INFO level with optional key=value fields.
func (l *Logger) Info(msg string, fields ...any) {
	l.write(LevelInfo, msg, fields)
}

// Warn logs a message at WARN level with optional key=value fields.
func (l *Logger) Warn(msg string, fields ...any) {
	l.write(LevelWarn, msg, fields)
}

// Error logs a message at ERROR level with optional key=value fields.
func (l *Logger) Error(msg string, fields ...any) {
	l.write(LevelError, msg, fields)
}

// Debug logs a message at DEBUG level with optional key=value fields.
func (l *Logger) Debug(msg string, fields ...any) {
	l.write(LevelDebug, msg, fields)
}

func (l *Logger) write(level Level, msg string, fields []any) {
	if level < l.minLevel {
		return
	}

	// Ensure fields are even-length key-value pairs.
	if len(fields)%2 != 0 {
		fields = append(fields, "MISSING_VALUE")
	}

	entry := map[string]any{
		"time":  time.Now().UTC().Format(time.RFC3339),
		"level": level.String(),
		"msg":   msg,
	}
	for i := 0; i < len(fields); i += 2 {
		key, ok := fields[i].(string)
		if !ok {
			key = "INVALID_KEY"
		}
		entry[key] = fields[i+1]
	}

	b, err := json.Marshal(entry)
	if err != nil {
		// Fallback: emit a minimal error entry.
		b = []byte(`{"level":"ERROR","msg":"jsonlog: failed to marshal entry"}`)
	}
	b = append(b, '\n')

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(b)
}

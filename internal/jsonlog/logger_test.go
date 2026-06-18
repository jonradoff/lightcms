package jsonlog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	cases := map[Level]string{
		LevelDebug: "DEBUG",
		LevelInfo:  "INFO",
		LevelWarn:  "WARN",
		LevelError: "ERROR",
		Level(99):  "UNKNOWN",
	}
	for lvl, want := range cases {
		if got := lvl.String(); got != want {
			t.Errorf("Level(%d).String() = %q, want %q", lvl, got, want)
		}
	}
}

func TestNew(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelWarn)
	if l == nil {
		t.Fatal("New returned nil")
	}
	if l.minLevel != LevelWarn {
		t.Errorf("minLevel = %v, want %v", l.minLevel, LevelWarn)
	}
}

func TestDefaultLogger(t *testing.T) {
	if Default == nil {
		t.Fatal("Default logger is nil")
	}
}

// parse decodes the single JSON log line written to buf.
func parse(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no log output")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid JSON log line %q: %v", line, err)
	}
	return m
}

func TestLevelMethods(t *testing.T) {
	cases := []struct {
		name  string
		log   func(l *Logger, msg string, f ...any)
		level string
	}{
		{"Info", (*Logger).Info, "INFO"},
		{"Warn", (*Logger).Warn, "WARN"},
		{"Error", (*Logger).Error, "ERROR"},
		{"Debug", (*Logger).Debug, "DEBUG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, LevelDebug)
			tc.log(l, "hello", "k", "v")
			m := parse(t, &buf)
			if m["level"] != tc.level {
				t.Errorf("level = %v, want %v", m["level"], tc.level)
			}
			if m["msg"] != "hello" {
				t.Errorf("msg = %v, want hello", m["msg"])
			}
			if m["k"] != "v" {
				t.Errorf("field k = %v, want v", m["k"])
			}
			if _, ok := m["time"]; !ok {
				t.Error("missing time field")
			}
		})
	}
}

func TestWriteBelowMinLevelSkipped(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelError)
	l.Info("should be dropped")
	l.Debug("also dropped")
	if buf.Len() != 0 {
		t.Errorf("expected no output below min level, got %q", buf.String())
	}
	l.Error("kept")
	if buf.Len() == 0 {
		t.Error("expected output at/above min level")
	}
}

func TestWriteOddFieldsGetsMissingValue(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo)
	l.Info("msg", "loneKey")
	m := parse(t, &buf)
	if m["loneKey"] != "MISSING_VALUE" {
		t.Errorf("odd field = %v, want MISSING_VALUE", m["loneKey"])
	}
}

func TestWriteNonStringKey(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo)
	l.Info("msg", 123, "value")
	m := parse(t, &buf)
	if m["INVALID_KEY"] != "value" {
		t.Errorf("non-string key not normalized: %v", m)
	}
}

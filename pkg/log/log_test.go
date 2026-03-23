package log

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestGetLogFields(t *testing.T) {
	tests := []struct {
		name       string
		args       []interface{}
		wantFields map[string]interface{}
	}{
		{
			name:       "single key-value pair",
			args:       []interface{}{"key", "value"},
			wantFields: map[string]interface{}{"key": "value"},
		},
		{
			name:       "multiple key-value pairs",
			args:       []interface{}{"name", "test", "count", 42},
			wantFields: map[string]interface{}{"name": "test", "count": 42},
		},
		{
			name:       "empty args",
			args:       []interface{}{},
			wantFields: map[string]interface{}{},
		},
		{
			name:       "nil args",
			args:       nil,
			wantFields: map[string]interface{}{},
		},
		{
			name:       "odd number of args drops last key",
			args:       []interface{}{"key1", "value1", "orphan"},
			wantFields: map[string]interface{}{"key1": "value1"},
		},
		{
			name:       "single arg is dropped",
			args:       []interface{}{"only_key"},
			wantFields: map[string]interface{}{},
		},
		{
			name:       "non-string keys are converted to string",
			args:       []interface{}{123, "numeric_key", true, "bool_key"},
			wantFields: map[string]interface{}{"123": "numeric_key", "true": "bool_key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLogFields(tt.args...)
			if len(got) != len(tt.wantFields) {
				t.Errorf("getLogFields() returned %d fields, want %d", len(got), len(tt.wantFields))
				return
			}
			for k, wantVal := range tt.wantFields {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("getLogFields() missing key %q", k)
					continue
				}
				if gotVal != wantVal {
					t.Errorf("getLogFields()[%q] = %v, want %v", k, gotVal, wantVal)
				}
			}
		})
	}
}

func TestLogFunctions(t *testing.T) {
	// Capture logrus output for testing
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	logrus.SetLevel(logrus.DebugLevel)
	defer func() {
		logrus.SetOutput(nil)
		logrus.SetLevel(logrus.InfoLevel)
	}()

	tests := []struct {
		name    string
		logFunc func(string, ...interface{})
		msg     string
		args    []interface{}
	}{
		{
			name:    "Debug with fields",
			logFunc: Debug,
			msg:     "debug message",
			args:    []interface{}{"key", "value"},
		},
		{
			name:    "Info with fields",
			logFunc: Info,
			msg:     "info message",
			args:    []interface{}{"count", 1},
		},
		{
			name:    "Warn with no fields",
			logFunc: Warn,
			msg:     "warn message",
			args:    []interface{}{},
		},
		{
			name:    "Error with multiple fields",
			logFunc: Error,
			msg:     "error message",
			args:    []interface{}{"err", "something failed", "code", 500},
		},
		{
			name:    "Debug with no args",
			logFunc: Debug,
			msg:     "simple debug",
			args:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			// Should not panic
			tt.logFunc(tt.msg, tt.args...)
			output := buf.String()
			if output == "" {
				t.Errorf("%s produced no output", tt.name)
			}
			if !bytes.Contains([]byte(output), []byte(tt.msg)) {
				t.Errorf("%s output %q does not contain message %q", tt.name, output, tt.msg)
			}
		})
	}
}

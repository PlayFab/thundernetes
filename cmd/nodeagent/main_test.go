package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	healthzHandler(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "OK", recorder.Body.String())
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		setEnv   bool
		required bool
		expected string
	}{
		{
			name:     "returns env value when set",
			key:      "TEST_GET_ENV_SET",
			value:    "some_value",
			setEnv:   true,
			required: false,
			expected: "some_value",
		},
		{
			name:     "returns empty for optional missing env",
			key:      "TEST_GET_ENV_MISSING",
			setEnv:   false,
			required: false,
			expected: "",
		},
		{
			name:     "returns empty string when env is empty",
			key:      "TEST_GET_ENV_EMPTY",
			value:    "",
			setEnv:   true,
			required: false,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.value)
			}
			result := getEnv(tt.key, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetNumericEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		setEnv   bool
		fallback int
		expected int
	}{
		{
			name:     "returns parsed int when set",
			key:      "TEST_NUMERIC_VALID",
			value:    "42",
			setEnv:   true,
			fallback: 10,
			expected: 42,
		},
		{
			name:     "returns fallback when not set",
			key:      "TEST_NUMERIC_MISSING",
			setEnv:   false,
			fallback: 10,
			expected: 10,
		},
		{
			name:     "returns fallback when invalid number",
			key:      "TEST_NUMERIC_INVALID",
			value:    "not_a_number",
			setEnv:   true,
			fallback: 10,
			expected: 10,
		},
		{
			name:     "returns fallback when empty string",
			key:      "TEST_NUMERIC_EMPTY",
			value:    "",
			setEnv:   true,
			fallback: 10,
			expected: 10,
		},
		{
			name:     "returns zero when env is zero",
			key:      "TEST_NUMERIC_ZERO",
			value:    "0",
			setEnv:   true,
			fallback: 10,
			expected: 0,
		},
		{
			name:     "returns negative number",
			key:      "TEST_NUMERIC_NEGATIVE",
			value:    "-5",
			setEnv:   true,
			fallback: 10,
			expected: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.value)
			}
			result := getNumericEnv(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBooleanEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		setEnv   bool
		fallback bool
		expected bool
	}{
		{
			name:     "returns true when set to true",
			key:      "TEST_BOOL_TRUE",
			value:    "true",
			setEnv:   true,
			fallback: false,
			expected: true,
		},
		{
			name:     "returns false when set to false",
			key:      "TEST_BOOL_FALSE",
			value:    "false",
			setEnv:   true,
			fallback: true,
			expected: false,
		},
		{
			name:     "returns fallback when not set",
			key:      "TEST_BOOL_MISSING",
			setEnv:   false,
			fallback: true,
			expected: true,
		},
		{
			name:     "returns fallback when invalid",
			key:      "TEST_BOOL_INVALID",
			value:    "not_a_bool",
			setEnv:   true,
			fallback: true,
			expected: true,
		},
		{
			name:     "returns fallback when empty",
			key:      "TEST_BOOL_EMPTY",
			value:    "",
			setEnv:   true,
			fallback: false,
			expected: false,
		},
		{
			name:     "accepts 1 as true",
			key:      "TEST_BOOL_ONE",
			value:    "1",
			setEnv:   true,
			fallback: false,
			expected: true,
		},
		{
			name:     "accepts 0 as false",
			key:      "TEST_BOOL_ZERO",
			value:    "0",
			setEnv:   true,
			fallback: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.key, tt.value)
			}
			result := getBooleanEnv(tt.key, tt.fallback)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		setEnv   bool
		expected log.Level
	}{
		{
			name:     "debug level",
			envValue: "debug",
			setEnv:   true,
			expected: log.DebugLevel,
		},
		{
			name:     "warn level",
			envValue: "warn",
			setEnv:   true,
			expected: log.WarnLevel,
		},
		{
			name:     "defaults to info when not set",
			setEnv:   false,
			expected: log.InfoLevel,
		},
		{
			name:     "defaults to info for invalid value",
			envValue: "invalid_level",
			setEnv:   true,
			expected: log.InfoLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv("LOG_LEVEL", tt.envValue)
			} else {
				t.Setenv("LOG_LEVEL", "")
			}
			setLogLevel()
			assert.Equal(t, tt.expected, log.GetLevel())
		})
	}
}

func TestMetricsHandler(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		url            string
		expectedStatus int
	}{
		{
			name:           "valid GSDK info",
			body:           `{"Flavor":"Unity","Version":"1.0"}`,
			url:            "/v1/metrics/test-server/gsdkinfo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid GSDK info with different flavor",
			body:           `{"Flavor":"Unreal","Version":"2.5.1"}`,
			url:            "/v1/metrics/another-server/gsdkinfo",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid JSON body returns bad request",
			body:           `not-json`,
			url:            "/v1/metrics/test-server/gsdkinfo",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset the package-level flag so logging branch is exercised
			gsdkMetricsLogged = false

			n := &NodeAgentManager{
				gameServerMap: &sync.Map{},
			}

			req := httptest.NewRequest(http.MethodPost, tt.url, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			n.metricsHandler(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
		})
	}
}

func TestMetricsHandlerLogsOnlyOnce(t *testing.T) {
	gsdkMetricsLogged = false

	n := &NodeAgentManager{
		gameServerMap: &sync.Map{},
	}

	gi := GsdkVersionInfo{Flavor: "Unity", Version: "1.0"}
	b, err := json.Marshal(gi)
	assert.NoError(t, err)

	// first call — should set gsdkMetricsLogged to true
	req1 := httptest.NewRequest(http.MethodPost, "/v1/metrics/server1/gsdkinfo", bytes.NewReader(b))
	w1 := httptest.NewRecorder()
	n.metricsHandler(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.True(t, gsdkMetricsLogged)

	// second call — gsdkMetricsLogged should still be true (no reset)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/metrics/server2/gsdkinfo", bytes.NewReader(b))
	w2 := httptest.NewRecorder()
	n.metricsHandler(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.True(t, gsdkMetricsLogged)
}

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestParseInt64FromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		setEnv       bool
		defaultValue int64
		expected     int64
	}{
		{
			name:         "returns value from env when set",
			envVar:       "TEST_INT64_VALID",
			envValue:     "42",
			setEnv:       true,
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "returns default when env not set",
			envVar:       "TEST_INT64_NOT_SET",
			setEnv:       false,
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "returns default when env value is not a number",
			envVar:       "TEST_INT64_INVALID",
			envValue:     "not_a_number",
			setEnv:       true,
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "parses negative numbers",
			envVar:       "TEST_INT64_NEGATIVE",
			envValue:     "-5",
			setEnv:       true,
			defaultValue: 10,
			expected:     -5,
		},
		{
			name:         "parses zero",
			envVar:       "TEST_INT64_ZERO",
			envValue:     "0",
			setEnv:       true,
			defaultValue: 10,
			expected:     0,
		},
		{
			name:         "returns default for empty string",
			envVar:       "TEST_INT64_EMPTY",
			envValue:     "",
			setEnv:       true,
			defaultValue: 10,
			expected:     10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv {
				t.Setenv(tt.envVar, tt.envValue)
			}
			result := ParseInt64FromEnv(tt.envVar, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateHeartbeatRequest(t *testing.T) {
	tests := []struct {
		name        string
		hb          *HeartbeatRequest
		expectError bool
		// if CurrentGameHealth was empty, it should be set to "Healthy"
		expectedHealth string
	}{
		{
			name: "valid heartbeat",
			hb: &HeartbeatRequest{
				CurrentGameState:  GameStateStandingBy,
				CurrentGameHealth: "Healthy",
			},
			expectError:    false,
			expectedHealth: "Healthy",
		},
		{
			name: "empty health defaults to Healthy",
			hb: &HeartbeatRequest{
				CurrentGameState:  GameStateActive,
				CurrentGameHealth: "",
			},
			expectError:    false,
			expectedHealth: "Healthy",
		},
		{
			name: "empty state returns error",
			hb: &HeartbeatRequest{
				CurrentGameState:  "",
				CurrentGameHealth: "Healthy",
			},
			expectError: true,
		},
		{
			name: "both empty state and health - health defaults, state errors",
			hb: &HeartbeatRequest{
				CurrentGameState:  "",
				CurrentGameHealth: "",
			},
			expectError: true,
		},
		{
			name: "valid with players",
			hb: &HeartbeatRequest{
				CurrentGameState:  GameStateActive,
				CurrentGameHealth: "Healthy",
				CurrentPlayers:    []ConnectedPlayer{{PlayerId: "player1"}},
			},
			expectError:    false,
			expectedHealth: "Healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateHeartbeatRequest(tt.hb)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHealth, tt.hb.CurrentGameHealth)
			}
		})
	}
}

func TestIsValidStateTransition(t *testing.T) {
	tests := []struct {
		name     string
		oldState GameState
		newState GameState
		expected bool
	}{
		{"empty to Initializing", "", GameStateInitializing, true},
		{"empty to StandingBy", "", GameStateStandingBy, true},
		{"Initializing to StandingBy", GameStateInitializing, GameStateStandingBy, true},
		{"StandingBy to Active", GameStateStandingBy, GameStateActive, true},
		{"same state Initializing", GameStateInitializing, GameStateInitializing, true},
		{"same state StandingBy", GameStateStandingBy, GameStateStandingBy, true},
		{"same state Active", GameStateActive, GameStateActive, true},
		{"Active to StandingBy invalid", GameStateActive, GameStateStandingBy, false},
		{"StandingBy to Initializing invalid", GameStateStandingBy, GameStateInitializing, false},
		{"Active to Initializing invalid", GameStateActive, GameStateInitializing, false},
		{"Initializing to Active invalid (skip StandingBy)", GameStateInitializing, GameStateActive, false},
		{"empty to Active invalid", "", GameStateActive, false},
		{"empty to Terminating invalid", "", GameStateTerminating, false},
		{"Active to Terminating invalid", GameStateActive, GameStateTerminating, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidStateTransition(tt.oldState, tt.newState)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"no special chars", "hello world", "hello world"},
		{"removes newlines", "hello\nworld", "helloworld"},
		{"removes carriage returns", "hello\rworld", "helloworld"},
		{"removes both", "hello\r\nworld", "helloworld"},
		{"empty string", "", ""},
		{"multiple newlines", "a\nb\nc", "abc"},
		{"only newlines", "\n\n\n", ""},
		{"mixed content", "line1\nline2\rline3\r\n", "line1line2line3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInternalServerError(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := assert.AnError
	internalServerError(recorder, err, "test error message")

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "500")
	assert.Contains(t, recorder.Body.String(), "test error message")
}

func TestBadRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := assert.AnError
	badRequest(recorder, err, "bad request message")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "400")
	assert.Contains(t, recorder.Body.String(), "bad request message")
}

func TestParseSessionDetails(t *testing.T) {
	tests := []struct {
		name                  string
		obj                   map[string]interface{}
		expectedSessionID     string
		expectedSessionCookie string
		expectedPlayers       []string
	}{
		{
			name: "all fields present",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"sessionID":      "session-123",
					"sessionCookie":  "cookie-abc",
					"initialPlayers": []interface{}{"player1", "player2"},
				},
			},
			expectedSessionID:     "session-123",
			expectedSessionCookie: "cookie-abc",
			expectedPlayers:       []string{"player1", "player2"},
		},
		{
			name: "missing session fields",
			obj: map[string]interface{}{
				"status": map[string]interface{}{},
			},
			expectedSessionID:     "",
			expectedSessionCookie: "",
			expectedPlayers:       nil,
		},
		{
			name:                  "missing status entirely",
			obj:                   map[string]interface{}{},
			expectedSessionID:     "",
			expectedSessionCookie: "",
			expectedPlayers:       nil,
		},
		{
			name: "empty initial players",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"sessionID":      "session-456",
					"sessionCookie":  "cookie-def",
					"initialPlayers": []interface{}{},
				},
			},
			expectedSessionID:     "session-456",
			expectedSessionCookie: "cookie-def",
			expectedPlayers:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.obj}
			sessionID, sessionCookie, initialPlayers := parseSessionDetails(u, "test-gs", "test-ns")
			assert.Equal(t, tt.expectedSessionID, sessionID)
			assert.Equal(t, tt.expectedSessionCookie, sessionCookie)
			assert.Equal(t, tt.expectedPlayers, initialPlayers)
		})
	}
}

func TestParseStateHealth(t *testing.T) {
	tests := []struct {
		name           string
		obj            map[string]interface{}
		expectedState  string
		expectedHealth string
		expectError    bool
	}{
		{
			name: "both state and health present",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"state":  "Active",
					"health": "Healthy",
				},
			},
			expectedState:  "Active",
			expectedHealth: "Healthy",
			expectError:    false,
		},
		{
			name: "state missing",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"health": "Healthy",
				},
			},
			expectError: true,
		},
		{
			name: "health missing",
			obj: map[string]interface{}{
				"status": map[string]interface{}{
					"state": "Active",
				},
			},
			expectError: true,
		},
		{
			name: "both missing",
			obj: map[string]interface{}{
				"status": map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name:        "no status field",
			obj:         map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.obj}
			state, health, err := parseStateHealth(u)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedState, state)
				assert.Equal(t, tt.expectedHealth, health)
			}
		})
	}
}

func TestParseBuildName(t *testing.T) {
	tests := []struct {
		name              string
		obj               map[string]interface{}
		expectedBuildName string
		expectError       bool
	}{
		{
			name: "build name present",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						"BuildName": "my-build",
					},
				},
			},
			expectedBuildName: "my-build",
			expectError:       false,
		},
		{
			name: "build name missing",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{},
				},
			},
			expectError: true,
		},
		{
			name: "labels missing",
			obj: map[string]interface{}{
				"metadata": map[string]interface{}{},
			},
			expectError: true,
		},
		{
			name:        "metadata missing",
			obj:         map[string]interface{}{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.obj}
			buildName, err := parseBuildName(u)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBuildName, buildName)
			}
		})
	}
}

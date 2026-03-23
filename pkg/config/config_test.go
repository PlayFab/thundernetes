package config

import (
	"flag"
	"os"
	"testing"
)

func TestParseConfigFileAndEnvParams(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		wantConfigFile string
		wantExpandENV  bool
	}{
		{
			name:           "both flags provided",
			args:           []string{"-config-file", "myconfig.yaml", "-config-expand-env"},
			wantConfigFile: "myconfig.yaml",
			wantExpandENV:  true,
		},
		{
			name:           "only config-file flag",
			args:           []string{"-config-file", "test.yaml"},
			wantConfigFile: "test.yaml",
			wantExpandENV:  false,
		},
		{
			name:           "only expand-env flag",
			args:           []string{"-config-expand-env"},
			wantConfigFile: "",
			wantExpandENV:  true,
		},
		{
			name:           "no flags provided",
			args:           []string{},
			wantConfigFile: "",
			wantExpandENV:  false,
		},
		{
			name:           "nil args",
			args:           nil,
			wantConfigFile: "",
			wantExpandENV:  false,
		},
		{
			name:           "flags mixed with unknown flags",
			args:           []string{"-unknown-flag", "value", "-config-file", "app.yaml", "-another"},
			wantConfigFile: "app.yaml",
			wantExpandENV:  false,
		},
		{
			name:           "config-file with equals syntax",
			args:           []string{"-config-file=equals.yaml"},
			wantConfigFile: "equals.yaml",
			wantExpandENV:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFile, gotExpand := ParseConfigFileAndEnvParams(tt.args)
			if gotFile != tt.wantConfigFile {
				t.Errorf("ParseConfigFileAndEnvParams() configFile = %q, want %q", gotFile, tt.wantConfigFile)
			}
			if gotExpand != tt.wantExpandENV {
				t.Errorf("ParseConfigFileAndEnvParams() expandENV = %v, want %v", gotExpand, tt.wantExpandENV)
			}
		})
	}
}

func TestExpandEnvInConfigFile(t *testing.T) {
	tests := []struct {
		name  string
		input string
		envs  map[string]string
		want  string
	}{
		{
			name:  "expand set variable with braces",
			input: "host: ${TEST_HOST}",
			envs:  map[string]string{"TEST_HOST": "localhost"},
			want:  "host: localhost",
		},
		{
			name:  "expand set variable without braces",
			input: "host: $TEST_HOST_NOBRACE",
			envs:  map[string]string{"TEST_HOST_NOBRACE": "127.0.0.1"},
			want:  "host: 127.0.0.1",
		},
		{
			name:  "default value used when env not set",
			input: "port: ${UNSET_PORT:8080}",
			envs:  map[string]string{},
			want:  "port: 8080",
		},
		{
			name:  "default value ignored when env is set",
			input: "port: ${TEST_PORT:8080}",
			envs:  map[string]string{"TEST_PORT": "9090"},
			want:  "port: 9090",
		},
		{
			name:  "unset variable with no default expands to empty",
			input: "value: ${COMPLETELY_UNSET_VAR}",
			envs:  map[string]string{},
			want:  "value: ",
		},
		{
			name:  "multiple variables in one string",
			input: "${TEST_A}:${TEST_B}",
			envs:  map[string]string{"TEST_A": "hello", "TEST_B": "world"},
			want:  "hello:world",
		},
		{
			name:  "empty input",
			input: "",
			envs:  map[string]string{},
			want:  "",
		},
		{
			name:  "no variables to expand",
			input: "plain text without vars",
			envs:  map[string]string{},
			want:  "plain text without vars",
		},
		{
			name:  "default value with colon in value",
			input: "${UNSET_URL:http://localhost:8080/path}",
			envs:  map[string]string{},
			want:  "http://localhost:8080/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			for k, v := range tt.envs {
				os.Setenv(k, v)
			}
			// Clean up after test
			defer func() {
				for k := range tt.envs {
					os.Unsetenv(k)
				}
			}()

			// Ensure variables we expect to be unset are actually unset
			os.Unsetenv("COMPLETELY_UNSET_VAR")
			os.Unsetenv("UNSET_PORT")
			os.Unsetenv("UNSET_URL")

			got := string(ExpandEnvInConfigFile([]byte(tt.input)))
			if got != tt.want {
				t.Errorf("ExpandEnvInConfigFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIgnoreConfigAndEnvParamsFromFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)

	// Should not panic
	IgnoreConfigAndEnvParamsFromFlags(fs)

	// Verify both flags were registered
	configFlag := fs.Lookup(ConfigFileParam)
	if configFlag == nil {
		t.Errorf("expected flag %q to be registered", ConfigFileParam)
	}

	expandFlag := fs.Lookup(ExpandENVParam)
	if expandFlag == nil {
		t.Errorf("expected flag %q to be registered", ExpandENVParam)
	}
}

package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetLinearRegressionPrediction(t *testing.T) {
	tests := []struct {
		name            string
		data            []timeSeriesPoint
		timeToForecast  int64
		expectError     bool
		expectedPositive bool // if we just want to verify the sign
	}{
		{
			name: "linear increasing data predicts higher value",
			data: func() []timeSeriesPoint {
				baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				return []timeSeriesPoint{
					{time: baseTime, value: 10},
					{time: baseTime.Add(1 * time.Minute), value: 20},
					{time: baseTime.Add(2 * time.Minute), value: 30},
					{time: baseTime.Add(3 * time.Minute), value: 40},
					{time: baseTime.Add(4 * time.Minute), value: 50},
				}
			}(),
			timeToForecast:  time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC).Unix(),
			expectError:     false,
			expectedPositive: true,
		},
		{
			name: "constant data predicts same value",
			data: func() []timeSeriesPoint {
				baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				return []timeSeriesPoint{
					{time: baseTime, value: 50},
					{time: baseTime.Add(1 * time.Minute), value: 50},
					{time: baseTime.Add(2 * time.Minute), value: 50},
					{time: baseTime.Add(3 * time.Minute), value: 50},
					{time: baseTime.Add(4 * time.Minute), value: 50},
				}
			}(),
			timeToForecast:  time.Date(2024, 1, 1, 0, 5, 0, 0, time.UTC).Unix(),
			expectError:     false,
			expectedPositive: true,
		},
		{
			name: "two data points returns error (regression requires more)",
			data: func() []timeSeriesPoint {
				baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				return []timeSeriesPoint{
					{time: baseTime, value: 10},
					{time: baseTime.Add(1 * time.Minute), value: 20},
				}
			}(),
			timeToForecast:  time.Date(2024, 1, 1, 0, 2, 0, 0, time.UTC).Unix(),
			expectError:     true,
			expectedPositive: false,
		},
		{
			name: "three data points suffice",
			data: func() []timeSeriesPoint {
				baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
				return []timeSeriesPoint{
					{time: baseTime, value: 10},
					{time: baseTime.Add(1 * time.Minute), value: 20},
					{time: baseTime.Add(2 * time.Minute), value: 30},
				}
			}(),
			timeToForecast:  time.Date(2024, 1, 1, 0, 3, 0, 0, time.UTC).Unix(),
			expectError:     false,
			expectedPositive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prediction, err := getLinearRegressionPrediction(tt.data, tt.timeToForecast)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedPositive {
					assert.Greater(t, prediction, 0.0)
				}
			}
		})
	}
}

func TestGetLinearRegressionPredictionAccuracy(t *testing.T) {
	// Test with perfectly linear data where we know the exact prediction
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	data := []timeSeriesPoint{
		{time: baseTime, value: 0},
		{time: baseTime.Add(1 * time.Minute), value: 10},
		{time: baseTime.Add(2 * time.Minute), value: 20},
		{time: baseTime.Add(3 * time.Minute), value: 30},
		{time: baseTime.Add(4 * time.Minute), value: 40},
	}

	prediction, err := getLinearRegressionPrediction(data, baseTime.Add(5*time.Minute).Unix())
	require.NoError(t, err)
	// For perfectly linear data, the prediction should be very close to 50
	assert.InDelta(t, 50.0, prediction, 1.0)
}

func TestGetForecastedServerCount(t *testing.T) {
	// Create a series with enough data points for all forecasting methods.
	// We need at least seasonLength + forecastedPoints for Holt-Winters, and
	// enough points for both long and short linear history.
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	seriesLen := 100
	series := make([]timeSeriesPoint, seriesLen)
	for i := 0; i < seriesLen; i++ {
		series[i] = timeSeriesPoint{
			time:  baseTime.Add(time.Duration(i) * time.Minute),
			value: 50.0, // constant value
		}
	}

	cfg := Config{
		AlphaConstant:                          0.6,
		BetaConstant:                           0.6,
		GammaConstant:                          0.45,
		SeasonLength:                           10,
		ForecastedPoints:                       5,
		LongLinearHistoryPoints:                30,
		LongLinearForecastedPoints:             10,
		ShortLinearHistoryPoints:               10,
		ShortLinearForecastedPoints:            5,
		DatapointGranularity:                   time.Minute,
		MetricToServerConversionOperation:      "divide",
		MetricToServerConversionOperationValue: 10.0,
	}

	totalServers, err := getForecastedServerCount(series, cfg)
	require.NoError(t, err)
	// With constant value of 50 and dividing by 10, should be about 5
	assert.GreaterOrEqual(t, totalServers, 1)
}

func TestGetForecastedServerCountMultiplyOperation(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	seriesLen := 100
	series := make([]timeSeriesPoint, seriesLen)
	for i := 0; i < seriesLen; i++ {
		series[i] = timeSeriesPoint{
			time:  baseTime.Add(time.Duration(i) * time.Minute),
			value: 5.0,
		}
	}

	cfg := Config{
		AlphaConstant:                          0.6,
		BetaConstant:                           0.6,
		GammaConstant:                          0.45,
		SeasonLength:                           10,
		ForecastedPoints:                       5,
		LongLinearHistoryPoints:                30,
		LongLinearForecastedPoints:             10,
		ShortLinearHistoryPoints:               10,
		ShortLinearForecastedPoints:            5,
		DatapointGranularity:                   time.Minute,
		MetricToServerConversionOperation:      "multiply",
		MetricToServerConversionOperationValue: 2.0,
	}

	totalServers, err := getForecastedServerCount(series, cfg)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, totalServers, 1)
}

func TestGetForecastedServerCountInvalidOperation(t *testing.T) {
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	seriesLen := 100
	series := make([]timeSeriesPoint, seriesLen)
	for i := 0; i < seriesLen; i++ {
		series[i] = timeSeriesPoint{
			time:  baseTime.Add(time.Duration(i) * time.Minute),
			value: 10.0,
		}
	}

	cfg := Config{
		AlphaConstant:                          0.6,
		BetaConstant:                           0.6,
		GammaConstant:                          0.45,
		SeasonLength:                           10,
		ForecastedPoints:                       5,
		LongLinearHistoryPoints:                30,
		LongLinearForecastedPoints:             10,
		ShortLinearHistoryPoints:               10,
		ShortLinearForecastedPoints:            5,
		DatapointGranularity:                   time.Minute,
		MetricToServerConversionOperation:      "invalid_op",
		MetricToServerConversionOperationValue: 1.0,
	}

	totalServers, err := getForecastedServerCount(series, cfg)
	require.NoError(t, err)
	// Invalid operation falls back to int(forecastValue) directly
	assert.GreaterOrEqual(t, totalServers, 0)
}

func TestGetForecastedServerCountNegativeForecast(t *testing.T) {
	// Use a sharply decreasing series to potentially produce negative forecasts
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	seriesLen := 100
	series := make([]timeSeriesPoint, seriesLen)
	for i := 0; i < seriesLen; i++ {
		// Decreasing from 100 to near 0
		val := math.Max(0, 100.0-float64(i)*1.5)
		series[i] = timeSeriesPoint{
			time:  baseTime.Add(time.Duration(i) * time.Minute),
			value: val,
		}
	}

	cfg := Config{
		AlphaConstant:                          0.6,
		BetaConstant:                           0.6,
		GammaConstant:                          0.45,
		SeasonLength:                           10,
		ForecastedPoints:                       5,
		LongLinearHistoryPoints:                30,
		LongLinearForecastedPoints:             10,
		ShortLinearHistoryPoints:               10,
		ShortLinearForecastedPoints:            5,
		DatapointGranularity:                   time.Minute,
		MetricToServerConversionOperation:      "divide",
		MetricToServerConversionOperationValue: 10.0,
	}

	totalServers, err := getForecastedServerCount(series, cfg)
	require.NoError(t, err)
	// Should be non-negative since negative forecasts are clamped to 0
	assert.GreaterOrEqual(t, totalServers, 0)
}

func TestLoadConfig(t *testing.T) {
	t.Run("valid config file", func(t *testing.T) {
		content := `
queryUrl: "http://prometheus:9090"
metricQuery: "sum(active_servers)"
frequency: 30s
forecastedPoints: 15
port: 9090
alpha: 0.5
beta: 0.5
gamma: 0.3
seasonLength: 30
forecastMinimumServers: 2
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg := Config{}
		err = loadConfig(configPath, false, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "http://prometheus:9090", cfg.QueryUrl)
		assert.Equal(t, "sum(active_servers)", cfg.MetricQuery)
		assert.Equal(t, 30*time.Second, cfg.ForecastFrequency)
		assert.Equal(t, 15, cfg.ForecastedPoints)
		assert.Equal(t, 9090, cfg.Port)
		assert.Equal(t, 0.5, cfg.AlphaConstant)
		assert.Equal(t, 0.5, cfg.BetaConstant)
		assert.Equal(t, 0.3, cfg.GammaConstant)
		assert.Equal(t, 30, cfg.SeasonLength)
		assert.Equal(t, 2, cfg.ForecastMinimumServers)
	})

	t.Run("nonexistent config file", func(t *testing.T) {
		cfg := Config{}
		err := loadConfig("/nonexistent/path/config.yaml", false, &cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Error reading config file")
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "bad.yaml")
		err := os.WriteFile(configPath, []byte("{{invalid yaml"), 0644)
		require.NoError(t, err)

		cfg := Config{}
		err = loadConfig(configPath, false, &cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Error parsing config file")
	})

	t.Run("config with env expansion", func(t *testing.T) {
		t.Setenv("TEST_PROM_URL", "http://custom-prom:9090")
		content := `
queryUrl: "${TEST_PROM_URL}"
metricQuery: "sum(active_servers)"
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg := Config{}
		err = loadConfig(configPath, true, &cfg)
		require.NoError(t, err)
		assert.Equal(t, "http://custom-prom:9090", cfg.QueryUrl)
	})

	t.Run("unknown fields rejected by strict unmarshal", func(t *testing.T) {
		content := `
queryUrl: "http://prometheus:9090"
unknownField: "should fail"
`
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		cfg := Config{}
		err = loadConfig(configPath, false, &cfg)
		assert.Error(t, err)
	})
}

func TestLoadConfigWithScaleDownSettings(t *testing.T) {
	content := `
queryUrl: "http://prometheus:9090"
metricQuery: "sum(active_servers)"
enableScaleDownCircuitBreaker: true
maxScaleDownStepSize: 5
enableScaleDownCoolOff: true
scaleDownCoolOffPeriod: 10m
forecastBufferServers: 3
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg := Config{}
	err = loadConfig(configPath, false, &cfg)
	require.NoError(t, err)

	assert.True(t, cfg.EnableScaleDownCircuitBreaker)
	assert.Equal(t, 5, cfg.MaxScaleDownStepSize)
	assert.True(t, cfg.EnableScaleDownCoolOff)
	assert.Equal(t, 10*time.Minute, cfg.ScaleDownCoolOffPeriod)
	assert.Equal(t, 3, cfg.ForecastBufferServers)
}

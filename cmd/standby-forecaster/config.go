package main

import (
	"flag"
	"path/filepath"
	"time"

	"k8s.io/client-go/util/homedir"
)

const (
	defaultMetricQuery = "sum(simulated_instances)"
)

type Config struct {
	QueryUrl                               string        `yaml:"queryUrl"`
	MetricQuery                            string        `yaml:"metricQuery"`
	ForecastFrequency                      time.Duration `yaml:"frequency"`
	ForecastedPoints                       int           `yaml:"forecastedPoints"`
	Port                                   int           `yaml:"port"`
	HistoricalQueryRange                   time.Duration `yaml:"historicalQueryRange"`
	AlphaConstant                          float64       `yaml:"alpha"`
	BetaConstant                           float64       `yaml:"beta"`
	GammaConstant                          float64       `yaml:"gamma"`
	SeasonLength                           int           `yaml:"seasonLength"`
	EnableHoltWintersForecast              bool          `yaml:"enableHoltWintersForecast"`
	EnableLongLinearForecast               bool          `yaml:"enableLongLinearForecast"`
	EnableShortlinearForecast              bool          `yaml:"enableShortLinearForecast"`
	LongLinearHistoryPoints                int           `yaml:"longLinearHistoryPoints"`
	LongLinearForecastedPoints             int           `yaml:"longLinearForecastedPoints"`
	ShortLinearHistoryPoints               int           `yaml:"shortLinearHistoryPoints"`
	ShortLinearForecastedPoints            int           `yaml:"shortLinearForecastedPoints"`
	ForecastMinimumServers                 int           `yaml:"forecastMinimumServers"`
	ForecastBufferServers                  int           `yaml:"forecastBufferServers"`
	DatapointGranularity                   time.Duration `yaml:"datapointGranularity"`
	EnableScaleDownCircuitBreaker          bool          `yaml:"enableScaleDownCircuitBreaker"`
	MaxScaleDownStepSize                   int           `yaml:"maxScaleDownStepSize"`
	EnableScaleDownCoolOff                 bool          `yaml:"enableScaleDownCoolOff"`
	ScaleDownCoolOffPeriod                 time.Duration `yaml:"scaleDownCoolOffPeriod"`
	TargetGameServerBuildName              string        `yaml:"targetGameServerBuildName"`
	TargetGameServerBuildNamespace         string        `yaml:"targetGameServerBuildNameSpace"`
	MetricToServerConversionOperation      string        `yaml:"metricToServerConversionOperation"`
	MetricToServerConversionOperationValue float64       `yaml:"metricToServerConversionOperationValue"`
	K8s                                    struct {
		RunInCluster bool   `yaml:"runInCluster"`
		KubeConfig   string `yaml:"kubeConfig"`
	} `yaml:"k8s"`
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.QueryUrl, "queryUrl", "http://localhost:9090", "URL to query the Prometheus server")
	f.StringVar(&c.MetricQuery, "metricQuery", defaultMetricQuery, "Query to retrieve the metric")
	f.IntVar(&c.ForecastedPoints, "points", 10, "number of points to forecast")
	f.IntVar(&c.Port, "port", 8080, "port to serve metrics endpoint")
	f.DurationVar(&c.ForecastFrequency, "forecast-frequency", 15*time.Second, "how often to calculate a new forecast")
	f.DurationVar(&c.HistoricalQueryRange, "historical-query-range", 6*24*time.Hour, "historical range for data points")
	f.Float64Var(&c.AlphaConstant, "alpha", 0.6, "alpha constant")
	f.Float64Var(&c.BetaConstant, "beta", 0.6, "beta constant")
	f.Float64Var(&c.GammaConstant, "gamma", 0.45, "gamma constant")
	f.IntVar(&c.SeasonLength, "seasonLength", 60, "season length in data points")
	f.BoolVar(&c.EnableHoltWintersForecast, "enableHoltWintersForecast", true, "enable Holt Winters forecast")
	f.BoolVar(&c.EnableLongLinearForecast, "enableLongLinearForecast", true, "enable Long Linear forecast")
	f.BoolVar(&c.EnableShortlinearForecast, "enableShortLinearForecast", true, "enable Short Linear forecast")
	f.IntVar(&c.LongLinearHistoryPoints, "longLinearHistoryPoints", 30, "number of historical points to use for Long Linear forecast")
	f.IntVar(&c.LongLinearForecastedPoints, "longLinearForecastedPoints", 10, "number of forecasted points to use for Long Linear forecast")
	f.IntVar(&c.ShortLinearHistoryPoints, "shortLinearHistoryPoints", 10, "number of historical points to use for Short Linear forecast")
	f.IntVar(&c.ShortLinearForecastedPoints, "shortLinearForecastedPoints", 5, "number of forecasted points to use for Short Linear forecast")
	f.IntVar(&c.ForecastMinimumServers, "forecastMinimumServers", 1, "minimum number of Standby servers to forecast")
	f.IntVar(&c.ForecastBufferServers, "forecastBufferServers", 0, "number of 'buffer' standby servers to add to forecast")
	f.DurationVar(&c.DatapointGranularity, "datapointGranularity", 1*time.Minute, "granularity of datapoints gathered from Prometheus")
	f.BoolVar(&c.EnableScaleDownCircuitBreaker, "enableScaleDownCircuitBreaker", true, "enable scale down circuit breaker")
	f.IntVar(&c.MaxScaleDownStepSize, "maxScaleDownStepSize", 1, "maximum step size for scale down")
	f.BoolVar(&c.EnableScaleDownCoolOff, "enableScaleDownCoolOff", true, "enable scale down cool off")
	f.DurationVar(&c.ScaleDownCoolOffPeriod, "scaleDownCoolOffPeriod", 5*time.Minute, "scale down cool off period")
	f.StringVar(&c.TargetGameServerBuildName, "targetGameServerBuildName", "gameserverbuild-sample-netcore", "target game server build name")
	f.StringVar(&c.TargetGameServerBuildNamespace, "targetGameServerBuildNamespace", "default", "target game server build namespace")
	f.StringVar(&c.MetricToServerConversionOperation, "metricToServerConversionOperation", "divide", "metric to server conversion operation. Can be 'divide' or 'multiply'")
	f.Float64Var(&c.MetricToServerConversionOperationValue, "metricToServerConversionOperationValue", 10, "metric to server conversion operation value")
	f.BoolVar(&c.K8s.RunInCluster, "k8s.runInCluster", false, "run in cluster")
	defaultKubConfig := ""
	if home := homedir.HomeDir(); home != "" {
		defaultKubConfig = filepath.Join(home, ".kube", "config")
	}
	f.StringVar(&c.K8s.KubeConfig, "k8s.kubeConfig", defaultKubConfig, "(optional) absolute path to the kubeconfig file")
}

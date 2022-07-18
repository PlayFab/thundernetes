package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grafana/dskit/flagext"
	"github.com/khaines/holtwinters"
	"github.com/pkg/errors"
	config "github.com/playfab/thundernetes/pkg/config"
	log "github.com/playfab/thundernetes/pkg/log"
	promClient "github.com/prometheus/client_golang/api"
	promapi "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
	"github.com/sajari/regression"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type timeSeriesPoint struct {
	time  time.Time
	value float64
}

const (
	metricsNamespace = "thundernetes"
)

var (
	WintersPrediction = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "winters_prediction",
		Help:      "The predicted value of the metric using holt winters method",
	})

	LongLinearPrediction = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "long_linear_prediction",
		Help:      "The predicted value of the metric using linear regression",
	})
	ShortLinearPrediction = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "short_linear_prediction",
		Help:      "The predicted value of the metric using linear regression",
	})

	ChosenForecast = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "chosen_prediction",
		Help:      "The chosen prediction of the metric",
	})

	TotalServersForecast = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "total_servers_prediction",
		Help:      "The total numbers of servers predicted",
	})

	PrometheusQueryExecutionTime = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "prometheus_query_execution_time",
		Help:      "The time it took to execute the prometheus query",
	})

	ForcastComputeTime = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: metricsNamespace,
		Name:      "forecast_compute_time",
		Help:      "The time it took to compute the forcast",
	})
)

func main() {
	log.Info("Starting Standby Forecaster")

	// parse special settings for config file and env expansion
	configFile, expandENV := config.ParseConfigFileAndEnvParams(os.Args[1:])

	cfg := Config{}
	flagext.RegisterFlags(&cfg)

	// if the configuration file was specified, load it into the Config instance
	// and expand ENVs if enabled
	if configFile != "" {
		if err := loadConfig(configFile, expandENV, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error loading config from %s: %v\n", configFile, err)
			os.Exit(1)
		}
	}

	config.IgnoreConfigAndEnvParamsFromFlags(flag.CommandLine)
	// parse the remaining command line params as overrides.
	flag.Parse()

	// root context for process
	ctx, ctxCancel := context.WithCancel(context.Background())
	// setup signal handlers
	shutdown := make(chan os.Signal, 2)
	defer close(shutdown)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// set a timer that ticks on the given frequency
	ticker := time.NewTicker(cfg.ForecastFrequency)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), nil)

	// setting to "now" on startup vs 0 to avoid a condition where the system is restarting often and scales down
	// too quickly, since the cooldown is not respected.
	lastScaleDownTime := time.Now()

	// loop through the timer
mainloop:
	for {
		select {
		case <-shutdown:
			log.Info("Shutting down")
			// cancel the root context, to abort any pending requests
			ctxCancel()
			break mainloop
		case <-ticker.C:
			// query the prometheus metrics
			startTime := time.Now()
			series, err := queryPrometheusMetrics(ctx, cfg.QueryUrl, cfg.MetricQuery, cfg.HistoricalQueryRange)
			PrometheusQueryExecutionTime.Set(float64(time.Since(startTime).Microseconds()))
			if err != nil {
				log.Error("Failed to query prometheus metrics", "error", err)
				continue
			}

			totalServersNeeded, err := getForecastedServerCount(series, cfg)
			if err != nil {
				log.Error("Failed to create a forecasted value", "error", err)
				continue
			}

			lastScaleDownTime, err = scaleGameServerBuild(ctx, totalServersNeeded, cfg, lastScaleDownTime)
			if err != nil {
				log.Error("Failed to scale the game server build", "error", err)
			}
		}
	}
}

func scaleGameServerBuild(ctx context.Context, totalServersNeeded int, config Config, lastScaleDownTime time.Time) (time.Time, error) {
	// now we need to get the current number of servers for the target game server build
	client, err := getKubeClient(config.TargetGameServerBuildNamespace, config.K8s.RunInCluster, config.K8s.KubeConfig)
	if err != nil {
		return time.Time{}, err
	}
	fetchCtx, fetchCtxCancel := context.WithTimeout(ctx, time.Duration(30*time.Second))
	defer fetchCtxCancel()
	gameServerBuild, err := client.Get(fetchCtx, config.TargetGameServerBuildName)
	if err != nil {
		return time.Time{}, err
	}
	// To determin how many standby servers are needed by the forecase, we need to subtract the active servers
	// from the projected total.
	currentActiveServers := gameServerBuild.Status.CurrentActive
	currentStandbyServers := gameServerBuild.Status.CurrentStandingBy
	desiredStandbyServers := totalServersNeeded - currentActiveServers
	wasScaleDown := false
	// if the number of needed standby servers is less than the current number of standby servers,
	// we need to scale down the standby servers.
	if desiredStandbyServers < currentStandbyServers {
		// if the Scaledown cooldown is set, we can only scale down if the cooldown has expired.
		if config.EnableScaleDownCoolOff && time.Since(lastScaleDownTime) < config.ScaleDownCoolOffPeriod {
			log.Info("Scale down cooldown is enabled and has not expired. Skipping scale down", "cooldown", config.ScaleDownCoolOffPeriod, "elapsed", time.Since(lastScaleDownTime))
			return lastScaleDownTime, nil
		}

		// If the scale down circuit breaker is enabled and the difference is greater than the threshold,
		// only scale down by the amount allowed by the circuit breaker.
		if config.EnableScaleDownCircuitBreaker && (currentStandbyServers-desiredStandbyServers) > config.MaxScaleDownStepSize {
			newDesiredStandbyServers := currentStandbyServers - config.MaxScaleDownStepSize
			log.Info("Desired scale down size exceeds max scale down step size. Scaling down by max step size", "desired", desiredStandbyServers, "current", currentStandbyServers, "max_step_size", config.MaxScaleDownStepSize, "new_desired", newDesiredStandbyServers)
			desiredStandbyServers = newDesiredStandbyServers
		}

		if desiredStandbyServers < config.ForecastMinimumServers {
			log.Info("Desired scale down size is less than the minimum number of standby servers. Scaling down to minimum number of standby servers", "desired", desiredStandbyServers, "minimum", config.ForecastMinimumServers)
			desiredStandbyServers = config.ForecastMinimumServers
		}

		log.Info("Scaling down standby servers", "current", currentStandbyServers, "desired", desiredStandbyServers)
		wasScaleDown = true
	} else if desiredStandbyServers > currentStandbyServers {
		log.Info("Scaling up standby servers", "current", currentStandbyServers, "desired", desiredStandbyServers)
	}

	if config.ForecastBufferServers > 0 {
		log.Info("Adding 'buffer' servers to final desired number of standby servers", "current", desiredStandbyServers, "buffer", config.ForecastBufferServers)
		desiredStandbyServers += config.ForecastBufferServers
	}
	gameServerBuild.TypeMeta.APIVersion = "mps.playfab.com/v1alpha1"
	gameServerBuild.TypeMeta.Kind = "GameServerBuild"
	gameServerBuild.Spec.StandingBy = desiredStandbyServers

	updateCtx, updateCtxCancel := context.WithTimeout(ctx, time.Duration(30*time.Second))
	defer updateCtxCancel()

	gameServerBuild, err = client.Update(updateCtx, gameServerBuild)
	if err != nil {
		log.Warn("msg", "Failed to update gameserver build, will attempt on next pass", "error", err)
		return time.Time{}, err
	}
	if wasScaleDown {
		now := time.Now()
		lastScaleDownTime = now
	}
	log.Info("Updated gameserver build standby count", "name", gameServerBuild.Name, "standby", gameServerBuild.Spec.StandingBy)
	return lastScaleDownTime, nil
}

func getForecastedServerCount(series []timeSeriesPoint, config Config) (int, error) {
	// triple exponential smoothing only requires the data point values
	// and assumes each is a consistent chunk of time
	values := make([]float64, len(series))
	for i := 0; i < len(series); i++ {
		values[i] = series[i].value
	}

	// create the forecasted series
	startTime := time.Now()
	forecastedSeries, err := holtwinters.TripleExponentialSmoothing(
		values,
		config.AlphaConstant,
		config.BetaConstant,
		config.GammaConstant,
		config.SeasonLength,
		config.ForecastedPoints)
	if err != nil {
		log.Error("Failed to create forecasted series from input series", "error", err)
		return 0, err
	}
	ForcastComputeTime.Set(float64(time.Since(startTime).Microseconds()))

	log.Debug("Exponential Smoothing Forecasted Series", "series", forecastedSeries)
	wintersForecast := forecastedSeries[len(forecastedSeries)-1]
	WintersPrediction.Set(wintersForecast)
	log.Debug("Holt-Winters Forecast", "forecast", wintersForecast)

	// long linear regression forecast
	longLinearForecast, err := getLinearRegressionPrediction(series[len(series)-config.LongLinearHistoryPoints:], series[len(series)-1].time.Add(time.Duration(config.LongLinearForecastedPoints*int(config.DatapointGranularity))).Unix())
	if err != nil {
		return 0, err
	}
	log.Debug("Long Linear Forecast", "forecast", longLinearForecast)
	LongLinearPrediction.Set(longLinearForecast)

	// short linear regression forecast
	shortLinearForecast, err := getLinearRegressionPrediction(series[len(series)-config.ShortLinearHistoryPoints:], series[len(series)-1].time.Add(time.Duration(config.ShortLinearForecastedPoints*int(config.DatapointGranularity))).Unix())
	if err != nil {
		return 0, err
	}
	log.Debug("Short Linear Forecast", "forecast", shortLinearForecast)
	ShortLinearPrediction.Set(shortLinearForecast)

	// always take the prediction with the highest value
	forecastValue := math.Max(wintersForecast, math.Max(longLinearForecast, shortLinearForecast))
	if forecastValue < 0 {
		forecastValue = 0
	}
	log.Debug("Chosen forecast value", "forecast", forecastValue)
	ChosenForecast.Set(forecastValue)

	// now convert this value into the total number of servers we need.
	// In some cases the metric being tracked is players or another metric
	// which does not directly represent the number of servers required.
	totalServersNeeded := 0
	if config.MetricToServerConversionOperation == "divide" {
		totalServersNeeded = int(math.Ceil(forecastValue / config.MetricToServerConversionOperationValue))
	} else if config.MetricToServerConversionOperation == "multiply" {
		totalServersNeeded = int(math.Ceil(forecastValue * config.MetricToServerConversionOperationValue))
	} else {
		log.Warn("Invalid metric to server conversion operation setting. No conversion will occur", "operation", config.MetricToServerConversionOperation)
		totalServersNeeded = int(forecastValue)
	}
	log.Debug("Total servers needed", "servers", totalServersNeeded)
	TotalServersForecast.Set(float64(totalServersNeeded))
	return totalServersNeeded, nil
}

// get kube client using the service account token for the pod
func getKubeClient(namespace string, runInCluster bool, kubeConfigPath string) (*GameServerBuildClient, error) {

	var kubeConfig *rest.Config
	var err error
	if runInCluster {
		kubeConfig, err = rest.InClusterConfig()
	} else {
		var configBytes []byte
		configBytes, err = ioutil.ReadFile(kubeConfigPath)
		if err != nil {
			return nil, err
		}
		kubeConfig, err = clientcmd.RESTConfigFromKubeConfig(configBytes)
	}

	if err != nil {
		return nil, err
	}
	config := *kubeConfig
	config.ContentConfig.GroupVersion = &schema.GroupVersion{Group: "mps.playfab.com", Version: "v1alpha1"}
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	config.UserAgent = rest.DefaultKubernetesUserAgent()
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &GameServerBuildClient{restClient: client, ns: namespace}, nil
}

func getLinearRegressionPrediction(data []timeSeriesPoint, timeToForcast int64) (float64, error) {
	// create the regression model
	r := new(regression.Regression)

	// loop through the data
	for i := 0; i < len(data); i++ {
		r.Train(regression.DataPoint(data[i].value, []float64{float64(data[i].time.Unix())}))
	}
	err := r.Run()
	if err != nil {
		return 0.0, err
	}
	// predict the value
	return r.Predict([]float64{float64(timeToForcast)})
}

func queryPrometheusMetrics(ctx context.Context, queryUrl, metricQuery string, historyRange time.Duration) ([]timeSeriesPoint, error) {
	// create a prometheus client
	client, err := promClient.NewClient(promClient.Config{
		Address: queryUrl,
	})
	if err != nil {
		return nil, err
	}

	api := promapi.NewAPI(client)

	// query the prometheus metrics
	rng := promapi.Range{}
	rng.Start = time.Now().Add(-historyRange)
	rng.End = time.Now()
	rng.Step = time.Minute
	queryCtx, queryCancel := context.WithTimeout(ctx, time.Duration(30*time.Second))
	defer queryCancel()
	queryResult, _, err := api.QueryRange(queryCtx, metricQuery, rng)
	if err != nil {
		return nil, err
	}
	results := queryResult.(model.Matrix)
	// initialize the time series
	timeSeries := make([]timeSeriesPoint, 0)

	// loop through the values
	for _, point := range results[0].Values {
		// add the time series
		timeSeries = append(timeSeries, timeSeriesPoint{
			time:  point.Timestamp.Time(),
			value: float64(point.Value),
		})
	}

	return timeSeries, nil
}

// LoadConfig read YAML-formatted config from filename into cfg.
func loadConfig(filename string, expandENV bool, cfg *Config) error {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "Error reading config file")
	}

	if expandENV {
		buf = config.ExpandEnvInConfigFile(buf)
	}

	err = yaml.UnmarshalStrict(buf, cfg)
	if err != nil {
		return errors.Wrap(err, "Error parsing config file")
	}

	return nil
}

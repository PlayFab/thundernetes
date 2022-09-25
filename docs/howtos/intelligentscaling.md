---
layout: default
title: Intelligent Scaling
parent: How to's
nav_order: 6
---

# Intelligent scaling

Thundernetes offers a dynamic standby forecaster that will automatically scale the size of the standby pool to meet projected utilization needs. Utilization data is gathered from a Prometheus data source, utilizing a configured query to return time series data.

Status: Alpha - This component is still being developed and may change drastically in design. Do not use for production scenarios.

Prediction is implemented using [linear regression](https://en.wikipedia.org/wiki/Linear_regression) and [Holt-Winters](https://en.wikipedia.org/wiki/Exponential_smoothing#Triple_exponential_smoothing_(Holt_Winters)) methods. The algorithm takes the top value predicted by the algorithms and sets the `standingBy` accordingly. It is the user's responsibility to set the `max` with a proper value.

You can use the forecaster immediately with Thundernetes. Until it has 2 "seasons" worth of data, the Holt-Winters calculation will not work effectively, but the linear regression forecasters will work just fine.

## Installation & Setup

You can see an example set of settings with the [sample forecaster.yaml](https://github.com/PlayFab/thundernetes/blob/main/samples/standby-forecaster/forecaster.yaml) file.

Installing the forecaster is directly done by defining and applying a kubernetes spec file, similar to our [sample](https://github.com/PlayFab/thundernetes/blob/main/samples/standby-forecaster/forecaster.yaml). However there will be a number of important settings to change in the configuration.

### Configuration settings

The preferred way to configure the forecaster is to create a ConfigMap as the sample above does and mount that as a file within the application pod. Within this YAML file a number of settings can be defined:

| Setting Name  | Default Value | Description |
|---------------|---------------|-------------|
| `targetGameServerBuildName` | ""  | The `GameServerBuild` this forecaster should manage the standby pool of.  |
| `targetGameServerBuildNamespace` | `default`  | The namespace of where the `GameServerBuild` resides. |
| `queryUrl`  | `"http://localhost:9090"` | The location of the Prometheus instance used to gather telemetry from.  |
| `metricQuery` | "" | The PromQL query to execute in order to determine server utilization as a single time series. |
| `historicalQueryRange` | `6h` | The time range used when querying Prometheus data to build historical trends. |
| `datapointGranularity` | `1m` | Time range of each data point returned by Prometheus. |
| `seasonLength`  | `1440`  | The number of datapoints which represent a full "season" of data. For game servers this is usually 1 day, so 1440 minutes. The size of each data point is affected by the `datapointGranularity` setting.  |
| `forecastedPoints` | `30` | The number of data points to forecast into the future based on existing data. A value of 30 with a 1m granularity will result in forecasts for 30min into the future. The size of each data point is affected by the `datapointGranularity` setting. |

Example configuration:

{% include code-block-start.md %}
  targetGameServerBuildName: "gameserverbuild-sample-netcore"
  targetGameServerBuildNamespace: "default"
  queryUrl: "http://prometheus-operated.monitoring:9090"
  metricQuery: "sum(simulated_instances)"
  historicalQueryRange: 6h
  datapointGranularity: 1m    
  seasonLength: 60
  forecastedPoints: 10
  k8s:
    runInCluster: true
{% include code-block-end.md %}

### Server load simulation

We have developed a [server load simulation component](https://github.com/PlayFab/thundernetes/tree/main/cmd/server-load-simulator) that will dynamically emit metrics with simulated load data, which can be used to test the forecaster. You can use [this YAML](https://github.com/PlayFab/thundernetes/blob/main/samples/server-load-simulator/simulator.yaml) to install the simulator. This YAML file installs:

- a Kubernetes Deployment for the simulator
- a Kubernetes Service that points to the Prometheus endpoint of the simulator
- a ServiceMonitor that points to the Service

The simulator has some command line arguments that can be customized:

| Setting Name  | Default Value | Description |
|---------------|---------------|-------------|
| `maxValue` | `100`  | Maximum value of the load.  |
| `frequency` | `time.Hour`  | ForecastFrequency of the load. |
| `jitter`  | `0.15` | Jitter of the load.  |
| `port` | `8080`  | Port to listen on (for the Prometheus metrics).  |

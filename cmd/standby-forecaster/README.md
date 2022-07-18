# Dynamic Standby Forecaster

The dynamic standby forecaster will automatically scale the size of the standby pool to meet projected utilization needs. Utilization data is gathered from a Prometheus data source, utilizing a configured query to return time series data.

Status: Alpha - This component is still being developed and may change drastically in design. Do not use for production scenarios.

## Installation & Setup

You can see an example set of settings with the [sample forecaster.yaml](../../samples/standby-forecaster/forcaster.yaml) file.

Installing the forecaster is directly done by defining and applying a kubernetes spec file, similar to our [sample](../../samples/standby-forecaster/forcaster.yaml). However there will be a number of important settings to change in the configuration.

### Configuration settings

The preferred way to configure the forecaster is to create a Configmap as the sample above does and mount that as a file within the application pod. Within this YAML file a number of settings can be defined:

| Setting Name  | Default Value | Description |
|---------------|---------------|-------------|
| `targetGameServerBuildName` | ""  | The `GameServerBuild` this forecaster should manage the standby pool of.  |
| `targetGameServerBuildNamespace` | `default`  | The namespace of where the `GameServerBuild` resides. |
| `queryUrl`  | `"http://localhost:9090"` | The location of the Prometheus instance used to gather telemetry from.  |
| `metricsQuery` | "" | The PromQL query to execute in order to determine server utilization as a single time series. |
| `historicalQueryRange` | `6h` | The time range used when querying Prometheus data to build historical trends. |
| `datapointGranularity` | `1m` | Time range of each data point returned by Prometheus. |
| `seasonLength`  | `1440`  | The number of datapoints which represent a full "season" of data. For game servers this is usually 1 day, so 1440 minutes. The size of each data point is affected by the `datapointGranularity` setting.  |
| `forecastedPoints` | `30` | The number of data points to forecast into the future based on existing data. A value of 30 with a 1m granularity will result in forecasts for 30min into the future. The size of each data point is affected by the `datapointGranularity` setting. |

Example configuration:

```yaml
  targetGameServerBuildName: "gameserverbuild-sample-netcore"
  targetGameServerBuildNamespace: "default"
  queryUrl: "http://prometheus-operated.monitoring:9090"
  metricsQuery: "sum(simulated_instances)"
  historicalQueryRange: 6h
  datapointGranularity: 1m    
  seasonLength: 60
  forecastedPoints: 10
  k8s:
    runInCluster: true
```
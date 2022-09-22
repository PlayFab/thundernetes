---
layout: default
title: Monitoring
parent: How to's
nav_order: 1
---

# Monitoring

Thundernetes is able to export game server related metrics to a [Prometheus](https://prometheus.io/docs/introduction/overview/) server, and these can also be imported
to a [Grafana](https://grafana.com/docs/grafana/latest/introduction/) server for easy and intuitive visualizations.

[![Grafana Dasboard Example](../assets/images/grafana_dashboard_1.png)](../assets/images/grafana_dashboard_1.png)

Prometheus uses a pull model to retrieve data, and needs apps to implement an endpoint that responds to its HTTP requests. For this, Thundernetes exposes the following endpoints:

* **{controller manager service IP}:8080/metrics**
* **{nodeagent service IP}:56001/metrics**

## Install Thundernetes with Prometheus and Grafana

While it's possible to create and manage your own Prometheus and Grafana instances to consume the endpoints described above, it is also possible to install both into your K8s cluster using very few steps thanks to the [kube-prometheus](https://github.com/prometheus-operator/kube-prometheus) project. This will install the prometheus-operator and Grafana, and will connect them automatically. For this, follow these steps:

{% include code-block-start.md %}
# clone the kube-prometheus repository
git clone https://github.com/prometheus-operator/kube-prometheus.git

cd kube-prometheus/

# install kube-prometheus' CRDs
kubectl create -f manifests/setup
until kubectl get servicemonitors --all-namespaces ; do date; sleep 1; echo ""; done
kubectl create -f manifests/
{% include code-block-end.md %}

> **_NOTE_**: You can also check the [official Prometheus Operator docs](https://prometheus-operator.dev/)

After this, you can install Thundernetes using the install files "with monitoring", these automatically create Prometheus ServiceMonitors that crawl the endpoints described above.

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/installfiles/operator_with_monitoring.yaml
{% include code-block-end.md %}

## Check the data in Prometheus and Grafana

To test this, you can install the netcore Game Server Build sample, this is a basic application that uses GSDK to send information to Thundernetes. You should also allocate a server so you can see the data.

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/netcore/sample-requestslimits.yaml
{% include code-block-end.md %}
This will create a Game Server Build with 2 standby Game Servers, you can check they were successfully created like this:

{% include code-block-start.md %}
# check the build
kubectl get gsb

# check the servers
kubectl get gs
{% include code-block-end.md %}
 To allocate a server, you need to have access to the thundernetes-controller-manager IP, in an AKS cluster you can get that like this:
 {% include code-block-start.md %}
 export IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
 {% include code-block-end.md %}

 Then allocate the server. The buildId must be the same than the one in the YAML file of the netcore sample, and the sessionId is used to identify the session:
 {% include code-block-start.md %}
curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"ac1b7082-d811-47a7-89ae-fe1a9c48a6da"}' http://${IP}:5000/api/v1/allocate
 {% include code-block-end.md %}

You can check the data exported in Prometheus, to access the Prometheus instance in your cluster use  port forwarding and open localhost:9090 in your browser:
{% include code-block-start.md %}
kubectl --namespace monitoring port-forward svc/prometheus-k8s 9090
{% include code-block-end.md %}

You can check the same data in Grafana and create a custom dashboard, to access the Grafana instance in your cluster use port forwarding and open localhost:3000 in your browser:
{% include code-block-start.md %}
kubectl --namespace monitoring port-forward svc/grafana 3000
{% include code-block-end.md %}
There is a custom Grafana dashboard example that visualizes some of this data in the [samples/grafana](https://github.com/PlayFab/thundernetes/tree/main/samples/grafana) directory.

## List of exported Prometheus metrics

| Metric name | Metric type | Source |
| --- | --- | --- |
| gameserver_states | Gauge | nodeagent |
| connected_players | Gauge | nodeagent |
| gameservers_current_state_per_build | Gauge | controller-manager |
| gameservers_created_total | Counter | controller-manager |
| gameservers_sessionended_total | Counter | controller-manager |
| gameservers_crashed_total | Counter | controller-manager |
| gameservers_deleted_total | Counter | controller-manager |
| allocations_total | Counter | controller-manager |

## More pictures

[![Grafana Dasboard Example 2](../assets/images/grafana_dashboard_2.png)](../assets/images/grafana_dashboard_2.png)

[![Grafana Dasboard Example 3](../assets/images/grafana_dashboard_3.png)](../assets/images/grafana_dashboard_3.png)

[![Grafana Dasboard Example 4](../assets/images/grafana_dashboard_4.png)](../assets/images/grafana_dashboard_4.png)

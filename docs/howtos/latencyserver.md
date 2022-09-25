---
layout: default
title: Measure latency
parent: How to's
nav_order: 3
---

# How to measure latency

If you have multiple Thundernetes clusters on different regions, it might be useful to have a way to measure latency to these clusters. For this, we implemented a basic UDP echo server based on PlayFab's Quality of Service Server, you can check both the [C++ implementation](https://github.com/PlayFab/XPlatCppSdk/blob/master/code/source/playfab/QoS/PlayFabQoSApi.cpp) and the [C# implementation](https://github.com/PlayFab/CSharpSDK/blob/master/PlayFabSDK/source/Qos/PlayFabQosApi.cs).This server follows these simple rules:

- The server only accepts UDP requests, and the data must be 32 bytes max and must also start with 0xFFFF (1111 1111 1111 1111).
- If the requests are valid, the server will respond with the same data it received, but with the first 4 bytes flipped to 0x0000 (0000 0000 0000 0000).

In [Thundernetes' main repository](https://github.com/PlayFab/thundernetes/tree/main/cmd/latencyserver) you can find the code for the server and a Dockerfile to build the image. We also provide 2 example YAML files: one for [deploying the server](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/latencyserver.yaml), and another for [deploying a ServiceMonitor](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/monitor.yaml) to crawl the [prometheus metrics](./monitoring.md). All you have to do is run:

{% include code-block-start.md %}
# for the latency server
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/latencyserver/latencyserver.yaml

# for the service monitor
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/latencyserver/monitor.yaml
{% include code-block-end.md %}

The UDP server runs on the port defined by the `UDP_SERVER_PORT` environment variable. A prometheus `/metrics` endpoint is also exposed, on the port defined by the `METRICS_SERVER_PORT` environment variable, with a counter for the number of successful requests it has received.

## Deploying the latency server

You can find the deployment YAML file [here](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/latencyserver.yaml) and the corresponding ServiceMonitor file [here](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/monitor.yaml).

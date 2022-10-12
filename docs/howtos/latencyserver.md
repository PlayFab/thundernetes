---
layout: default
title: Measure latency
parent: How to's
nav_order: 3
---

# How to measure latency to various regions

You can deploy Thundernetes to more than one regions. There are two reasons to do this:

- Additional regions provide redundancy. If a single region fails, players can access servers in other regions.
- Additional regions allow players to access servers that are "nearby" and deliver low-latency connectivity.

If you have multiple Thundernetes clusters on different regions, it is useful to have a way to measure latency from the client device (e.g. console/PC) to these clusters. For this purpose, we  have implemented a basic UDP echo server based on PlayFab's Quality of Service Server. This server follows these simple rules:

- The server only accepts UDP requests, and the data must be 32 bytes max and must also start with 0xFFFF (1111 1111 1111 1111).
- If the requests are valid, the server will respond with the same data it received, but with the first 4 bytes flipped to 0x0000 (0000 0000 0000 0000).

The usage of UDP is important, because most multiplayer games use UDP transport for their most performance-critical game traffic. Internet service providers and other elements of the Internet ecosystem may deliver differentiated performance for UDP vs TCP vs ICMP flows.

For client code, you can check the [C++ implementation](https://github.com/PlayFab/XPlatCppSdk/blob/master/code/source/playfab/QoS/PlayFabQoSApi.cpp) and the [C# implementation](https://github.com/PlayFab/CSharpSDK/blob/master/PlayFabSDK/source/Qos/PlayFabQosApi.cs).

## Flow

This is the typical flow for using the latency server in the context of a player device:

- Create a UDP socket.
- For each region, send a single UDP datagram to port 3075 on the latency server. The message content must start with 0xFFFF (1111 1111 1111 1111).
- The server will reply with a single datagram, with the message contents having the first 2 bytes "flipped" to 0x0000 (0000 0000 0000 0000). The rest of the datagram contents will be copied from the initial ping.
- Measure the time between sending the UDP message and receiving a response.
- Sort the response times from lowest to highest, and request a GameServer in the region with the lowest latency.

## Deploying the latency server

In [Thundernetes' main repository](https://github.com/PlayFab/thundernetes/tree/main/cmd/latencyserver) you can find the code for the server and a Dockerfile to build the image. We also provide 2 example YAML files: one for [deploying the server](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/latencyserver.yaml), and another for [deploying a ServiceMonitor](https://github.com/PlayFab/thundernetes/blob/main/samples/latencyserver/monitor.yaml) to grab the [prometheus metrics](./monitoring.md). All you have to do is run:

{% include code-block-start.md %}
# for the latency server
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/latencyserver/latencyserver.yaml

# for the service monitor
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/latencyserver/monitor.yaml
{% include code-block-end.md %}

The UDP server runs on the port defined by the `UDP_SERVER_PORT` environment variable. A prometheus `/metrics` endpoint is also exposed, on the port defined by the `METRICS_SERVER_PORT` environment variable, with a counter for the number of successful requests it has received.

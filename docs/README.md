---
layout: default
title: Home
nav_order: 1
description: "Thundernetes makes it easy to run your game servers on Kubernetes."
permalink: /
---

# Thundernetes 

Welcome to Thundernetes, an open source project that allows you to run your game servers on a Kubernetes cluster! 

:exclamation: Latest release: [![GitHub release](https://img.shields.io/github/release/playfab/thundernetes.svg)](https://github.com/playfab/thundernetes/releases)

## Description

Thundernetes is a project originating from the [Azure PlayFab Multiplayer Servers](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) team and other teams in Azure/XBOX that enables you to run both Windows and Linux game servers on your Kubernetes cluster. Thundernetes can be useful in the following scenarios:

- host your game servers on a Kubernetes cluster, either on a public cloud provider or on-premise and allow your users to connect from everywhere
- pre-warm game servers so that they are ready to accept players within seconds, when the game is about to start
- as part of your iterative development process, you can use Thundernetes locally to test your game server code

Thundernetes offers:

- game server auto-scaling enabled by default, based on [requested standingBy levels](./gameserverbuild.md)
- a [latency server](./howtos/latencyserver.md) to test client connection to multiple Kubernetes cluster and determine the best cluster to connect to
- a [Game Server SDK](./gsdk/README.md) in multiple languages/environments (Unity, Unreal, C#, C++, Java, Go) and a [local utility](./gsdk/runlocalmultiplayeragent.md) to test your game server integration locally
- a [web-based User Interface](./thundernetesui/README.md) to manage Thundernetes deployments in multiple clusters. This component utilizes a [REST API](./gameserverapi/README.md) which you can use to manage your game servers
- an experimental [intelligent standingBy server count forecaster](./howtos/intelligentscaling.md) that utilizes various algorithms to predict the number of game servers that will be needed
- [game server related Prometheus metrics and Grafana charts](./howtos/monitoring.md)

## Prerequisite knowledge

New to Kubernetes or containers? Check our [prerequisites](prerequisites.md) document that has resources that will fill the knowledge gaps when working with technologies within Thundernetes. 

## Requirements

Thundernetes requires:

- A Kubernetes cluster, either on-premise or on a public cloud provider. Ideally, the cluster should support having a Public IP per Node to allow external incoming connections
- A game server 
  - integrated with the open source [Game Server SDK](https://github.com/playfab/gsdk) (GSDK). GSDK facilitates communication between your game server and Thundernetes. It has been battle-tested by multiple AAA titles for years on the [Azure PlayFab Multiplayer Servers service](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) and supports multiple popular programming languages and game engines like Unity, Unreal, C#, C++, Java, Go.
  - built as a Windows or Linux container image. This image should be deployed to a container registry that your Kubernetes cluster can access.

> **_NOTE_**: You can avoid having to integrate with GSDK by using the [wrapper sample](howtos/usingwrapper.md). This sample is great if you want to experiment with Thundernetes, however proper GSDK integration is highly recommended.

## Quickstart

Check the [quickstart](quickstart.md) document on how to install Thundernetes on your cluster and run a sample game server to verify that Thundernetes is working properly. 

Check the following image to see how easy it is to install and use Thundernetes:

[![asciicast](https://asciinema.org/a/438455.svg)](https://asciinema.org/a/438455)

For a video presentation, check:

[![What is Project Thundernetes? How Kubernetes Helps Games Scale](https://img.youtube.com/vi/zwnUfq1ygic/0.jpg)](https://www.youtube.com/watch?v=zwnUfq1ygic)


## Contributing

If you are interested in contributing to Thundernetes, please read our [Contributing Guide](contributing.md) and open a PR. We'd be more than happy to help you out!

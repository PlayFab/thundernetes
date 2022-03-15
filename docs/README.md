---
layout: default
title: Home
nav_order: 1
description: "Thundernetes makes it easy to run your game servers on Kubernetes."
permalink: /
---

# Thundernetes 

Welcome to Thundernetes, an open source project from teams in Azure/XBOX that enables you to run Linux game servers on your Kubernetes cluster! 

:exclamation: Latest release: [![GitHub release](https://img.shields.io/github/release/playfab/thundernetes.svg)](https://github.com/playfab/thundernetes/releases)

## Prerequisite knowledge

New to Kubernetes or containers? Check our [prerequisites](prerequisites.md) document that has resources that will fill the knowledge gaps when working with technologies within Thundernetes. 

## Requirements

Thundernetes requires:

- A Kubernetes cluster, either on-premise or on a public cloud provider. Ideally, the cluster should support having a Public IP per Node to allow external incoming connections
- A game server 
  - integrated with the open source [Game Server SDK](https://playfab.com/gsdk) (GSDK). GSDK has been battle-tested by multiple AAA titles for years on the [Azure PlayFab Multiplayer Servers service](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) and supports multiple popular programming languages and game engines like Unity, Unreal, C#, C++, Java, Go.
  - built as a Linux container image. This image should be deployed to a container registry that your Kubernetes cluster can access.

> **_NOTE_**: You can avoid having to integrate with GSDK by using the [wrapper sample](usingwrapper.md). This sample is great if you want to experiment with Thundernetes, however proper GSDK integration is highly recommended.

## Quickstart

Check the [quickstart](quickstart.md) document on how to install Thundernetes on your cluster and run a sample game server to verify that Thundernetes is working properly. 

Click on the following image to see how easy it is to install and use Thundernetes:

[![asciicast](https://asciinema.org/a/438455.svg)](https://asciinema.org/a/438455)

## Recommended links

- [Using a wrapper](usingwrapper.md) - use a wrapper process to launch your game server in Thundernetes, without integrating with GSDK
- [Your game server](yourgameserver.md) - how to use Thundernetes with your own game server
- [Defining a GameServerBuild](gameserverbuild.md) - how to define a GameServerBuild in YAML
- [Game Server lifecycle](gameserverlifecycle.md) - game server process lifecycle
- [Architecture](architecture.md) - overview of Thundernetes architecture
- [Troubleshooting Guide](troubleshooting/README.md) - public repository for all of Thundernetes troubleshooting guides
- [Development notes](development.md) - useful development notes if you are plan on contributing to Thundernetes
- [Frequently Asked Questions](FAQ.md) - frequently asked questions

## Contributing

If you are interested in contributing to Thundernetes, please read our [Contributing Guide](contributing.md) and open a PR. We'd be more than happy to help you out!

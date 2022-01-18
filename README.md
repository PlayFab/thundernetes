![CI workflow](https://github.com/playfab/thundernetes/actions/workflows/main.yml/badge.svg)
[![Software License](https://img.shields.io/badge/license-Apache-brightgreen.svg?style=flat-square)](LICENSE)
[![GitHub release](https://img.shields.io/github/release/playfab/thundernetes.svg)](https://github.com/playfab/thundernetes/releases)
![](https://img.shields.io/badge/status-alpha-red.svg)

# thundernetes

> thundernetes is an experimental project and not recommended for production use. However, feel free to try it and let us know of any feedback!

## Description

Thundernetes is a project from the [Azure PlayFab Multiplayer Servers (MPS)](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) team that enables you to run Linux game servers on your Kubernetes cluster. Thundernetes can be useful in the following scenarios:

- host your game servers on a Kubernetes cluster, either on a public cloud provider or on-premises
- do manual allocations of game server sessions
- validate your game server integration with GSDK
- as part of your iterative development process, you can use thundernetes to test your game server code before pushing it to the MPS service
- as part of your CI/CD pipeline, you can publish the game server to a container registry and then have it deploy to a Kubernetes cluster where you can run your tests

For a game server to be able to run in thundernetes, it must use the [PlayFab Game Server SDK (GSDK)](https://github.com/PlayFab/gsdk) either directly on the game server binary or indirectly, via a wrapper (check [here](https://github.com/PlayFab/MpsSamples/tree/master/wrappingGsdk) for an example).

> We will refer to the Azure PlayFab Multiplayer Servers as "MPS" in all pages of the documentation.

One of the main goals for thundernetes is to be portable with MPS - this means that your Linux Game Server that works on thundernetes will work with MPS and vice versa.

Thundernetes requires a Kubernetes cluster with Public IP per Node. We've tested it extensively on [Azure Kubernetes Service - AKS](https://docs.microsoft.com/azure/aks/intro-kubernetes) as well as in local clusters using [kind](https://kind.sigs.k8s.io/). You also need to have ports 10000-50000 open in your cluster, since these are the ports that Thundernetes will set up on your Kubernetes Nodes so they can receive game network traffic and forward to your game server Pod.

> You can use a Kubernetes cluster without a Public IP. However, you would need to configure your own network architecture if you want to access your game servers. For example, if you use a cloud provider's Load Balancer, you would need to configure routes from Load Balancer's public endpoints to the internal ones on your Kubernetes cluster.
> You can try Azure Kubernetes Service for free [azure.com/free](https://azure.com/free).

## Prerequisites

Check our [prerequisites](docs/prerequisites.md) document that has resources that will fill the knowledge gaps when working with technologies within thundernetes. 

## Quickstart

Check the [quickstart](docs/quickstart.md) document on how to install thundernetes on your cluster and run the sample game server. 

### Installing on Azure Kubernetes Service

Click on the following image for a quick preview of the quickstart:

[![asciicast](https://asciinema.org/a/438455.png)](https://asciinema.org/a/438455)

## Links

- [Prerequisites](docs/prerequisites.md) - resources that will fill the knowledge gaps when working with technologies within thundernetes
- [Quickstart](docs/quickstart.md) - Recommended - how to install thundernetes on your cluster and run the sample game server
- [Defining a GameServerBuild](docs/gameserverbuild.md) - Recommended - how to define a GameServerBuild in YAML
- [Your game server](docs/yourgameserver.md) - how to use thundernetes with your own game server
- [Game Server lifecycle](docs/gameserverlifecycle.md) - game server process lifecycle
- [Architecture](docs/architecture.md)
- [Frequently Asked Questions](docs/FAQ.md)
- [Troubleshooting Guide](docs/troubleshooting/README.md) - public repository for all of thundernetes Troubleshooting guides
- [Development notes](docs/development.md) - useful if you are working on thundernetes development
- [Other Kubernetes resources](docs/resources.md)

## Feedback

As mentioned, thundernetes is in preview and a work in progress. If you find a bug or have a feature request, please file an issue [here](https://github.com/PlayFab/thundernetes/issues) and we will try to get back to you as soon as possible. You can also reach us directly on [Game Stack server on Discord](https://discord.gg/gamestack).

## Contributing

If you are interested in contributing to thundernetes, please read our [Contributing Guide](docs/contributing.md) and open a PR. We'd be more than happy to help you out!

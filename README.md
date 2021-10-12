![CI workflow](https://github.com/playfab/thundernetes/actions/workflows/main.yml/badge.svg)
[![Software License](https://img.shields.io/badge/license-Apache-brightgreen.svg?style=flat-square)](LICENSE)
![](https://img.shields.io/badge/status-alpha-red.svg)

# thundernetes

> thundernetes is an experimental project and not recommended for production use. However, we consider it as a great tool for testing your game server before uploading it to Azure PlayFab Multiplayer Servers.

## Description

Thundernetes is an preview project from the [Azure PlayFab Multiplayer Servers (MPS)](https://docs.microsoft.com/en-us/gaming/playfab/features/multiplayer/servers/) team that enables you to run Linux game servers that use the [PlayFab Game Server SDK (GSDK)](https://github.com/PlayFab/gsdk) on your Kubernetes cluster. Thundernetes can be useful while developing your game server in the following scenarios:

- validate your game server integration with GSDK
- do manual allocations of game server sessions
- as part of your iterative development process, you can use thundernetes to test your game server code before pushing it to the MPS service
- as part of your CI/CD pipeline, you can publish the game server to a container registry and then have it deploy to a Kubernetes cluster where you can run your tests

> We will refer to the Azure PlayFab Multiplayer Servers service as "MPS" in all pages of the documentation.

Goal for thundernetes is to be portable with MPS - this means that your Linux Game Server that works on thundernetes should work with MPS.

Thundernetes requires a Kubernetes cluster with Public IP per Node. We've tested it extensively on [Azure Kubernetes Service - AKS](https://docs.microsoft.com/en-us/azure/aks/intro-kubernetes) as well as in local clusters using [kind](https://kind.sigs.k8s.io/). You also need to have ports 10000-50000 open in your cluster, since these are the ports that Thundernetes will use to receive traffic and forward to your game server.

> Quick reminder that you can try Azure (and AKS) for free at [azure.com/free](https://azure.com/free).

## Quickstart

Check the [quickstart](docs/quickstart.md) document on how to install thundernetes on your cluster and run the sample game server. 

### Installing on Azure Kubernetes Service

Click on the following image for a quick preview of the quickstart:

[![asciicast](https://asciinema.org/a/438455.png)](https://asciinema.org/a/438455)

## Links

- [Quickstart](docs/quickstart.md) - Recommended - how to install thundernetes on your cluster and run the sample game server
- [Defining a GameServerBuild](docs/gameserverbuild.md) - Recommended - how to define a GameServerBuild in YAML
- [Your game server](docs/yourgameserver.md) - Recommended - how to use thundernetes with your own game server
- [Architecture](docs/architecture.md)
- [Frequently Asked Questions](docs/FAQ.md)
- [Other Kubernetes resources](docs/resources.md)
- [Development notes](docs/development.md) - useful if you are working on thundernetes development

## Feedback

As mentioned, thundernetes is in preview and a work in progress. If you find a bug or have a feature request, please file an issue [here](https://github.com/PlayFab/thundernetes/issues) and we will try to get back to you as soon as possible. You can also reach us on [Game Stack server on Discord](https://discord.gg/gamestack).

![CI workflow](https://github.com/playfab/thundernetes/actions/workflows/main.yml/badge.svg)
[![Software License](https://img.shields.io/badge/license-Apache-brightgreen.svg?style=flat-square)](LICENSE)
[![GitHub release](https://img.shields.io/github/release/playfab/thundernetes.svg)](https://github.com/playfab/thundernetes/releases)
![](https://img.shields.io/badge/status-alpha-red.svg)
[![CodeQL](https://github.com/PlayFab/thundernetes/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/PlayFab/thundernetes/actions/workflows/codeql-analysis.yml)

# (code name) thundernetes

> Thundernetes is an experimental project and not recommended for production use. However, feel free to try it and let us know of any feedback! Thundernetes is a [code name](https://github.com/PlayFab/thundernetes/issues/177), project will soon have a new name!

## Description

Thundernetes is a project from the [Azure PlayFab Multiplayer Servers (MPS)](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) team that enables you to run Linux game servers on your Kubernetes cluster. Thundernetes can be useful in the following scenarios:

- host your game servers on a Kubernetes cluster, either on a public cloud provider or on-premises
- do manual allocations of game server sessions
- validate your game server integration with GSDK
- as part of your iterative development process, you can use thundernetes to test your game server code before pushing it to the MPS service
- as part of your CI/CD pipeline, you can publish the game server to a container registry and then have it deploy to a Kubernetes cluster where you can run your tests

## Usage

Check [our website](https://playfab.github.io/thundernetes) for more information.


## Feedback

As mentioned, thundernetes is in preview and a work in progress. If you find a bug or have a feature request, please file an issue [here](https://github.com/PlayFab/thundernetes/issues) and we will try to get back to you as soon as possible. You can also reach us directly on [Game Stack server on Discord](https://discord.gg/gamestack).


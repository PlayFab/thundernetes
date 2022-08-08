[![e2e](https://github.com/PlayFab/thundernetes/actions/workflows/main.yml/badge.svg)](https://github.com/PlayFab/thundernetes/actions/workflows/e2e.yml)
[![unit-tests](https://github.com/PlayFab/thundernetes/actions/workflows/main.yml/badge.svg)](https://github.com/PlayFab/thundernetes/actions/workflows/unit-tests.yml)
[![Software License](https://img.shields.io/badge/license-Apache-brightgreen.svg?style=flat-square)](LICENSE)
[![GitHub release](https://img.shields.io/github/release/playfab/thundernetes.svg)](https://github.com/playfab/thundernetes/releases)
![](https://img.shields.io/badge/status-beta-lightgreen.svg)
[![CodeQL](https://github.com/PlayFab/thundernetes/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/PlayFab/thundernetes/actions/workflows/codeql-analysis.yml)

# Thundernetes

Thundernetes makes it easy to run your game servers on Kubernetes.

## ℹ️ Description

Thundernetes is a project originating from the [Azure PlayFab Multiplayer Servers](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/) team and other teams in Azure/XBOX that enables you to run both Windows and Linux game servers on your Kubernetes cluster. Thundernetes can be useful in the following scenarios:

- host your game servers on a Kubernetes cluster, either on a public cloud provider or on-premise and allow your users to connect from everywhere
- pre-warm game servers so that they are ready to accept players within seconds, when the game is about to start
- as part of your iterative development process, you can use Thundernetes locally to test your game server code

Thundernetes offers:

- game server auto-scaling, based on [requested standingBy levels](https://playfab.github.io/thundernetes/gameserverbuild.html)
- a [latency server](https://playfab.github.io/thundernetes/howtos/latencyserver.html) to test client connection to multiple Kubernetes cluster and determine the best cluster to connect to
- a [Game Server SDK](https://playfab.github.io/thundernetes/gsdk/README.html) in multiple languages/environments (Unity, Unreal, C#, C++, Java, Go) and a [local utility](https://playfab.github.io/thundernetes/gsdk/runlocalmultiplayeragent.html) to test your game server integration locally
- a [web-based User Interface](https://playfab.github.io/thundernetes/thundernetesui/README.html) to manage Thundernetes deployments in multiple clusters. This component utilizes a [REST API](https://playfab.github.io/thundernetes/gameserverapi/README.html) which you can use to manage your game servers
- an experimental [intelligent standingBy server count forecaster](https://playfab.github.io/thundernetes/howtos/intelligentscaling.html) that utilizes various algorithms to predict the number of game servers that will be needed
- [game server related Prometheus metrics and Grafana charts](https://playfab.github.io/thundernetes/howtos/monitoring.html)

## 📚 Documentation

Check 🔥[our website](https://playfab.github.io/thundernetes)🔥 for more information.

## 📦 Video presentation

Check out our video presentation for GDC 2022!

[![What is Project Thundernetes? How Kubernetes Helps Games Scale](https://img.youtube.com/vi/zwnUfq1ygic/0.jpg)](https://www.youtube.com/watch?v=zwnUfq1ygic)

## 💬❓Feedback - Community 

As mentioned, Thundernetes is in beta stage. If you find a bug or have a feature request, please file an issue [here](https://github.com/PlayFab/thundernetes/issues) and we will try to get back to you as soon as possible. You can also reach us directly on [Game Dev server on Discord](https://aka.ms/msftgamedevdiscord).

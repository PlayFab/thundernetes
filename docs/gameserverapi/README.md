---
layout: default
title: GameServer API
nav_order: 9
has_children: true
---

# GameServer API

The GameServer API is a RESTful API service that facilitates access to Thundernetes' Custom Resources: GameServerBuilds, GameServers, and GameServerDetails. It is an alternative for people who don't want to use tools like [kubectl](https://kubernetes.io/docs/reference/kubectl/kubectl/), and an easy way to integrate Thundernetes to your own applications. It also allows you to use our [Thundernetes UI](../thundernetesui/README.md).

## How to install the GameServer API

We provide a [Docker image](https://github.com/PlayFab/thundernetes/pkgs/container/thundernetes-gameserverapi) with the API, you have to deploy it into your cluster along with Thundernetes. We also have an [example YAML file](https://github.com/PlayFab/thundernetes/tree/main/samples/gameserverapi) for the deployment, all you have to do is run:

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/gameserverapi/gameserverapi.yaml
{% include code-block-end.md %}

This example uses a LoadBalancer to expose the application, so it will be assigned an external IP (this doesn't work locally unless you have a local implementation of a LoadBalancer, you can use [port forwarding](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/) instead).

> **_NOTE_**: The GameServer API provides direct access to your Thundernetes resources, we delegate the securing of the service to the user, for example, you can [use an Ingress](../howtos/serviceingress.md).

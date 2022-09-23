---
layout: default
title: .NET Sample
parent: Samples
grand_parent: Quickstart
nav_order: 1
---

# .NET Core game server

This sample, located [here](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore), is a simple .NET Core Web API app that implements GSDK. You can install it on your Kubernetes cluster by running the following command:

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/netcore/sample.yaml
{% include code-block-end.md %}

> _**NOTE**_: To read about the fields that you need to specify for a GameServerBuild, you can check [this document](../gameserverbuild.md).

Try using `kubectl get gs` to see the running game servers, you should see something similar to this:

{% include code-block-start.md %}
dgkanatsios@desktopdigkanat:thundernetes$ kubectl get gs
NAME                                   HEALTH    STATE        PUBLICIP      PORTS      SESSIONID
gameserverbuild-sample-netcore-ayxzh   Healthy   StandingBy   52.183.89.4   80:10001
gameserverbuild-sample-netcore-mveex   Healthy   StandingBy   52.183.89.4   80:10000
{% include code-block-end.md %}

and `kubectl get gsb` to see the status of the GameServerBuild:

{% include code-block-start.md %}
NAME                             STANDBY   ACTIVE   CRASHES   HEALTH
gameserverbuild-sample-netcore   2/2       0        0         Healthy
{% include code-block-end.md %}

> _**NOTE**_: `gs` and `gsb` are just short names for GameServer and GameServerBuild, respectively. You could just type `kubectl get gameserver` or `kubectl get gameserverbuild` instead.

To allocate a game server (convert its state to active) and scale your GameServerBuild, you can check [here](allocation-scaling.md).

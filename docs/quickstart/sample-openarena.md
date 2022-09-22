---
layout: default
title: OpenArena sample
parent: Samples
grand_parent: Quickstart
nav_order: 2
---

# Openarena sample

This sample, located [here](https://github.com/PlayFab/thundernetes/tree/main/samples/openarena), is based on the popular open source FPS game [OpenArena](https://openarena.ws/smfnews.php). You can install it using this script:

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/openarena/sample.yaml
{% include code-block-end.md %}

To connect to an active server, you need to download the OpenArena client from [here](https://openarena.ws/download.php?view.4).

To allocate a game server (convert its state to active) and scale your GameServerBuild, you can check [here](allocation-scaling.md).
---
layout: default
title: Minecraft sample
parent: Samples
grand_parent: Quickstart
nav_order: 3
---

# Minecraft sample

This sample, located [here](https://github.com/PlayFab/thundernetes/tree/main/samples/minecraft), is based on the popular sandbox game [Minecraft](https://www.minecraft.net/). You can install it using this script:

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/samples/minecraft/sample.yaml
{% include code-block-end.md %}

To connect to an active server, you need to own a Minecraft Java Edition copy (paid) from [here](https://www.minecraft.net/en-us/get-minecraft).

To allocate a game server (convert its state to active) and scale your GameServerBuild, you can check [here](allocation-scaling.md).
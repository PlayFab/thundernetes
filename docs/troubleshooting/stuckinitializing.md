---
layout: default
title: Stuck game server in initializing
parent: Troubleshooting
nav_order: 3
---

# My GameServer state does not transition into StandingBy

First of all, you should check if the GameServer has created a Pod. Try `kubectl get gs` to see the GameServer name, then use `kubectl get pod` to see if there is a Pod with same name as the GameServer. You can use `kubectl get pod` to see the high-level status of the Pod. Try running `kubectl logs <podName>` to see logs from your game server process. Also, try running `kubectl describe pod` to see the status of the Pod. Check there for some obvious failures, like failure to access the container registry, Pod creation failure because of resource constraints, etc. If everything works great, then probably there is an issue with the GSDK integration of your GameServer. You should take a look at [LocalMultiplayerAgent](../howtos/runlocalmultiplayeragent.md) to see how to run/test your GameServer locally and test its GSDK integration.
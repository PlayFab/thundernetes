---
layout: default
title: Game Server does not transition to StandingBy state
parent: Troubleshooting
nav_order: 3
---

# My GameServer state stays in empty or Initializing state and does not transition into StandingBy

❗ Check [here](../gsdk/gameserverlifecycle.md) for more information on GameServer lifecycle.

❗ You can check for the state of your GameServer by typing `kubectl get gs <gsName>` or `kubectl describe gs <gsName>`

You might see your GameServer stuck in the `Initializing` state (or in the empty state) and not transitioning to `StandingBy`. This can be a problem since eventually this GameServer cannot be allocated (converted to Active). Thundernetes only looks for `StandingBy` servers when looking for a GameServer to allocate.

If the GameServer state is empty, tou should check if a corresponding Pod has been created for this GameServer. Pods have the same name and are created in the same namespace as their corresponding GameServer so they are easy to locate. Try `kubectl get gs` to see the GameServer name, then use `kubectl get pod` to see if there is a Pod with same name as the GameServer. You can use `kubectl get pod` to see the high-level status of the Pod. Try running `kubectl logs <podName>` to see logs from your game server process. Also, try running `kubectl describe pod` to see the status of the Pod. Check there for some obvious failures, like failure to access the container registry, Pod creation failure because of resource constraints, etc. You can also try `kubectl exec -it <podName> -- sh` to open a shell in the game server Pod and investigate.

If everything looks OK, then probably there is an issue with the GSDK integration of your GameServer. You should take a look at [LocalMultiplayerAgent](../gsdk/runlocalmultiplayeragent.md) to see how to run/test your GameServer locally and test its GSDK integration.
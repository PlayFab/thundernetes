---
layout: default
title: Game server is stuck
parent: Troubleshooting
nav_order: 2
---

# What should we do if a GameServer gets stuck? 

Sometimes the GameServer process might not responding, due to a programming bug or a misconfiguration. You can check the logs by using the command `kubectl logs <Name>` and running a command shell into the Pod with `kubectl exec -it <Name> -- sh`. It might be useful to also check the NodeAgent logs for the Node that this Pod is running on, to see potential issues on GameServer and NodeAgent communication. You can check [our guide on how to get access to the NodeAgent logs](controllernodeagent.md) for more information.

If you want to delete the Pod, you can use the kubectl command `kubectl delete gs <Name>` since this will take down both the GameServer instance, the corresponding Pod as well as the GameServerDetail CR instance (in case the game server was allocated). 

❗ Do not directly delete the Pod. It will be deleted automatically when the GameServer is deleted.

❗ Do not manually overwrite GameServer.status details, this will create issues during controller reconciliation. 
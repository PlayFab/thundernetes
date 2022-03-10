---
layout: default
title: Stuck game servers
parent: Troubleshooting
nav_order: 2
---

# Help! What should we do if a GameServer gets stuck? 

You can run ```kubectl delete gs <Name>``` since this will take down both the GameServer instance, the corresponding Pod as well as the GameServerDetail CRD instance (in case the game server was allocated). 

:exclamation: Do not delete the Pod. It will be deleted automatically when the GameServer is deleted.
:exclamation: Do not manually override GameServer.status details, this will create issues during controller reconciliation. 
---
layout: default
title: Upgrading your game server
parent: How to's
nav_order: 9
---

# Upgrading your game server

Thundernetes does not support the concept of rolling upgrades to the GameServerBuild. The allocation API call uses the BuildID to allocate a game server on a specific GameServerBuild, so a new one should be used to eventually replace the old GameServerBuild. GameServerBuild specification should be considered immutable, except of course for the `standingBy` and `max` numbers.

To upgrade your game server process, you can follow the steps below:

- create a new GameServerBuild with a different BuildID and scale it to a high enough `standingBy` number of servers.
- modify your matchmaker/lobby service to allocate game servers using the new BuildID. 
- at the same time, set the `standingBy` number of the old Build to zero. Existing sessions on the old Build will eventually finish, but since the `standingBy` number is zero, no new game servers will be created.
- as the number of Actives on the new GameServerBuild increases, you should increase the `standingBy`/`max` numbers. Cluster Autoscaler should be on so that the number of Nodes in the cluster will increase as needed.
- once the number of Actives on the old GameServerBuild is zero, you can delete it.

> _**NOTE**_: At this point, Thundernetes does not do anything to prevent you from modifying any part of the GameServerBuild specification. However, this scenario is not recommended, as it can lead to unexpected behavior.
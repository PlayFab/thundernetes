---
layout: default
title: Host Networking
parent: How to's
nav_order: 7
---

# Host Networking

Thundernetes supports running your GameServer Pods with host networking. To do that, you need to provide a GameServerBuild YAML like [this](http://github.com/playfab/thundernetes/tree/main/samples/netcore/sample-hostnetwork.yaml), setting the `hostNetwork` value to true on PodSpec template. During Pod creation, Thundernetes controllers will **override** the containerPort with the same value that will be assigned in the hostPort. 

You **have to** use the generated port when you instantiate your game server process. To grab the port number, you should use the [GSDK](gsdk/README.md) `GetGameServerConnectionInfo` method.

```csharp
string ListeningPortKey = "gameport";
var gameServerConnectionInfo = GameserverSDK.GetGameServerConnectionInfo();
var portInfo = gameServerConnectionInfo.GamePortsConfiguration.Where(x=>x.Name == ListeningPortKey);
if(portInfo.Count() == 0)
{
    throw new Exception("No port info found for " + ListeningPortKey);
}
var port = portInfo.Single().ServerListeningPort;
```

> _**NOTE**_: It is necessary to provide a `containerPort` value in the GameServerBuild YAML, since it is required for GameServerBuild validation (specifically, the way the PodTemplate is validated from Kubernetes). However, as mentioned, this provided value is not used since it's overwritten by the `hostPort` value.
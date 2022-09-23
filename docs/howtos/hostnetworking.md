---
layout: default
title: Host Networking
parent: How to's
nav_order: 7
---

# Host Networking

Thundernetes supports running your GameServer Pods with host networking. In Kubernetes, enabling host networking will allow your Pods to use the host's (Node) network namespace and IP address. To register your game server Pods to use host networking, you need to provide a GameServerBuild YAML like [this](https://github.com/playfab/thundernetes/tree/main/samples/netcore/sample-hostnetwork.yaml), setting the `hostNetwork` value to true on PodSpec template. During Pod creation, Thundernetes controllers will **override** the containerPort with the value that will be assigned for the hostPort. 

You **have to** use the generated port when you instantiate your game server process. This is necessary since all the Pods will use the same network namespace and we want to prevent any port collisions from happening.

To grab the port number, you should use the [GSDK](../gsdk/README.md) `GetGameServerConnectionInfo` method. This is the code for the C# GSDK, code for other GSDKs would be similar:

{% include code-block-start.md %}
string ListeningPortKey = "gameport"; // IMPORTANT: this must be the same name with the one in the YAML file, for this port
var gameServerConnectionInfo = GameserverSDK.GetGameServerConnectionInfo();
var portInfo = gameServerConnectionInfo.GamePortsConfiguration.Where(x=>x.Name == ListeningPortKey);
if(portInfo.Count() == 0)
{
    throw new Exception("No port info found for " + ListeningPortKey);
}
var port = portInfo.Single().ServerListeningPort;
{% include code-block-end.md %}

> _**NOTE**_: It is necessary to provide a `containerPort` value in the GameServerBuild YAML, since it is required for GameServerBuild validation (specifically, the way the PodTemplate is validated on Kubernetes). However, as mentioned, this provided value is not used since it's overwritten by the `hostPort` value.
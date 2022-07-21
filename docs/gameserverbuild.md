---
layout: default
title: GameServerBuild definition
nav_order: 7
---

# GameServerBuild definition

GameServerBuild defines the specification and auto-scaling configuration of the GameServers that you want to run in the cluster. Each version of your game server should have its own GameServerBuild.

Here you can see the YAML that can be used to create a GameServerBuild in Thundernetes. The only fields that you should change after the GameServerBuild is created are the *standingBy* and the *max* ones. The other fields should be considered immutable.

```yaml
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample # required, name of the GameServerBuild
spec:
  titleID: "1E03" # required, corresponds to a unique ID for your game. Can be an arbitrary string
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5" # required, build ID of your game, must be GUID. Will be used for allocations, must be unique for each Build/version of your game server
  standingBy: 2 # required, number of standing by servers to create
  max: 4 # required, max number of servers to create. Sum of active+standingBy servers will never be larger than max
  crashesToMarkUnhealthy: 5 # optional. It is the number of crashes needed to mark the GameServerBuild unhealthy. Once this happens, no other operation will take place. If it is not set, Thundernetes will keep creating new GameServers as the old ones crash
  buildMetadata: # optional. Retrievable via GSDK, used to customize your game server
    - key: "buildMetadataKey1"
      value: "buildMetadataValue1"
    - key: "buildMetadataKey2"
      value: "buildMetadataValue2"
  portsToExpose: 
    - 7777
  template:
    spec:
      containers:
        - image: registryrepo/youGameServerImage:tag # image of your game server
          name: gameserver-sample # name of the container. 
          ports:
          - containerPort: 7777 # port that you want to expose
            name: gameport # name of the port that you want to expose. 
```

The template.spec contains the definition for a [Kubernetes Pod](https://kubernetes.io/docs/concepts/workloads/pods/). As a result, you should include here whatever is needed for your game server to run (environment variables, storage, etc).

## PortsToExpose

This is a list of ports that you want to be exposed in the [Worker Node/VM](https://kubernetes.io/docs/concepts/architecture/nodes/) when the Pod is created. Each Pod you create will have >=1 number of containers. There, each container will have its own *Ports* definition. If a port number in this definition is included in the *portsToExpose* array, this port will be publicly exposed in the Node/VM. Thundernetes accomplishes this by setting a **hostPort** value for each of the container ports you want to expose. If you have set a hostPort value in the container port definition, Thundernetes will override this value.

The reason for the `PortsToExpose` declaration is that you may want to use some ports on your Pod containers for other purposes rather than players connecting to it.

Ports that are to be exposed are assigned a number in the port range 10000-12000 by default. This port range is configurable, check [here](howtos/configureportrange.md) for details. 

**IMPORTANT**: Port names must be specified for all the ports that are in the *portsToExpose* array. Reason is that these ports are accessible via the [GSDK](gsdk/README.md), using their name. This way, the game server can discover them on runtime.

## CrashesToMarkUnhealthy

CrashesToMarkUnhealthy (integer) is the number of crashes that will transition the GameServerBuild to Unhealthy. Once this happens, no other reconcile/resize operation will take place on the GameServerBuild. To allow Thundernetes to continue performing reconciliations on the GameServerBuild after it has become Unhealthy, you can increase the value of the CrashesToMarkUnhealthy field or remove it completely. The GameServerBuild will be marked as Healthy again till the number of crashes reaches the value of CrashesToMarkUnhealthy.

Be very careful if you decided to remove the CrashesToMarkUnhealthy field. If you remove it, the GameServerBuild will never be marked as Unhealthy, no matter how many crashes it has. This might have the negative impact on Thundernetes constantly creating GameServers to replace the ones that have crashed. For this reason, we always recommend to set the CrashesToMarkUnhealthy field using a value that makes sense for your game/environment.

## Using host networking

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

## Game server image upgrades

You should **not** change any parts of the Pod specification in your GameServerBuild (including image/ports etc.). The best practice to upgrade your game server version is to spin up a separate GameServerBuild and gradually move traffic from the old GameServerBuild to the new one. Check our [relevant documentation](../howtos/upgradebuild.md) for more information.
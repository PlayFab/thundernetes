---
layout: default
title: GameServerBuild definition
nav_order: 7
---

# GameServerBuild definition

GameServerBuild defines the specification and auto-scaling configuration of a specific version of your GameServers that you want to run in the cluster. Each version of your game server should have its own GameServerBuild.

You need to specify these properties in your GameServerBuild YAML file:

- `titleID`: this is a unique string for your game, in case you want to host multiple games in the same cluster
- `buildID`: this is a unique identifier (GUID) for the specific version of your game server
- `standingBy`: this is a key property for the auto-scaling feature. A server in the `standingBy` state is a server that has loaded all the necessary assets and configuration and is ready to accept players. Thundernetes will always try to keep this number of servers in the `standingBy` state, always respecting the `max` threshold (see below). For more information on the game server states, check the [Game Server lifecycle](./gsdk/gameserverlifecycle.md) document.
- `max`: this is the maximum number of servers in all states. The sum of game servers in `initializing` + `standingBy` + `active` states will never be beyong the `max`, for each GameServerBuild.
- `buildMetadata`: an optional array of key/value pair strings that you can access from your game server process using the [Game Server SDK](./gsdk/README.md)
- `portsToExpose`: in this field you define which ports of your Pod will be exposed outside the cluster. Read on for more details.
- `crashesToMarkUnhealthy`: **optional but highly recommended**, this is the threshold for the number of crashes that will trigger your GameServerBuild to become `Unhealthy`. Read on for more details.
- `template`: this is the specification of [your game server pod](https://kubernetes.io/docs/concepts/workloads/pods/). You should include here whatever is needed for your game server to run (environment variables, storage, etc).

Here you can see a sample YAML file:

{% include code-block-start.md %}
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample # required, name of the GameServerBuild
spec:
  titleID: "1E03" # required, corresponds to a unique ID for your game. Can be an arbitrary string
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5" # required, build ID of your game, must be GUID. Will be used for allocations, must be unique for each Build/version of your game server
  standingBy: 2 # required, number of standing by servers to create
  max: 4 # required, max number of servers to create. Sum of active+standingBy+initializing servers will never be larger than max
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
{% include code-block-end.md %}

In general, the only fields that you should change after the GameServerBuild is created are the standingBy and the max ones. The other fields should be considered immutable.

## PortsToExpose

This is a list of ports that you want to be exposed in the [Worker Node/VM](https://kubernetes.io/docs/concepts/architecture/nodes/) when the Pod is created. Each Pod you create will have >=1 number of containers. There, each container will have its own *Ports* definition. If a port number in this definition is included in the *portsToExpose* array, this port will be publicly exposed in the Node/VM. Thundernetes accomplishes this by setting a **hostPort** value for each of the container ports you want to expose. If you have set a hostPort value in the container port definition, Thundernetes will override this value.

The reason for the `PortsToExpose` declaration is that you may want to use some ports on your Pod containers for other purposes rather than players connecting to it.

Ports that are to be exposed are assigned a number in the port range 10000-12000 by default. This port range is configurable, check [here](howtos/customportrange.md) for details. 

**IMPORTANT**: Port names must be specified for all the ports that are in the *portsToExpose* array. Reason is that these ports are accessible via the [GSDK](gsdk/README.md), using their name. This way, the game server can discover them on runtime.

## CrashesToMarkUnhealthy

CrashesToMarkUnhealthy (integer) is the number of crashes that will transition the GameServerBuild to Unhealthy. Once this happens, no other reconcile/resize operation will take place on the GameServerBuild. To allow Thundernetes to continue performing reconciliations on the GameServerBuild after it has become Unhealthy, you can increase the value of the CrashesToMarkUnhealthy field or remove it completely. The GameServerBuild will be marked as Healthy again till the number of crashes reaches the value of CrashesToMarkUnhealthy.

Be very careful if you decided to remove the CrashesToMarkUnhealthy field. If you remove it, the GameServerBuild will never be marked as Unhealthy, no matter how many crashes it has. This might have the negative impact on Thundernetes constantly creating GameServers to replace the ones that have crashed. For this reason, we always recommend to set the CrashesToMarkUnhealthy field using a value that makes sense for your game/environment.

## Host Networking

Thundernetes supports Kubernetes host networking (i.e. using the Node's network namespace), check the [host networking document](./howtos/hostnetworking.md) for more information.

## Game server image upgrades

You should **not** change any parts of the Pod specification in your GameServerBuild (including image/ports etc.). The best practice to upgrade your game server version is to spin up a separate GameServerBuild and gradually move traffic from the old GameServerBuild to the new one. Check our [relevant documentation](./howtos/upgradebuild.md) for more information.
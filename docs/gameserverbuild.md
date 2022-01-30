# GameServerBuild definition

GameServerBuild defines the amount and the specification of the GameServers that you want to run in the cluster.

> A GameServerBuild is equivalent to a Build region in MPS. GameServer containers that work in thundernetes should work in a similar way on PlayFab Multiplayer Servers service.

Here you can see the YAML that can be used to create a GameServerBuild in thundernetes. The only fields that you should change after the GameServerBuild is created are the *standingBy* and the *max* ones. The other fields should be considered immutable.

```yaml
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample # required, name of the GameServerBuild
spec:
  titleID: "1E03" # required, corresponds to the PlayFab TitleID your game server is using. Can be an arbitrary string
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f5" # required, build ID of your game, must be GUID. Will be used for allocations
  standingBy: 2 # required, number of standing by servers to create
  max: 4 # reqired, max number of servers to create. Active+StandingBy servers will never be larger than max
  crashesToMarkUnhealthy: 5 # optional, default is 5. It is the number of crashes needed to mark the GameServerBuild unhealthy. Once this happens, no other operation will take place 
  buildMetadata: # optional. Retrievable via GSDK, used to customize your game server
    - key: "buildMetadataKey1"
      value: "buildMetadataValue1"
    - key: "buildMetadataKey1"
      value: "buildMetadataValue1"
  portsToExpose: # port names that you need to expose for your game server, read more below
    - containerName: gameserver-sample # name of the container that you want its port exposed
      portName: gameport # name of the port that you want to expose
  template:
    spec:
      containers:
        - image: youGameServerImage:tag # image of your game server
          name: gameserver-sample # name of the container. Must be the same as portsToExpose.containerName
          ports:
          - containerPort: 7777 # port that you want to expose
            name: gameport # name of the port that you want to expose. Must be the same as portsToExpose.portName
```

The template.spec contains the definition for a [Kubernetes Pod](https://kubernetes.io/docs/concepts/workloads/pods/). As a result, you should include here whatever is needed for your game server (environment variables, storage, etc) to run.

## PortsToExpose

This is a list of containerName/portName tuples: These are the ports that you want to be exposed in the [Worker Node/VM](https://kubernetes.io/docs/concepts/architecture/nodes/) when the Pod is created. The way this works is that each Pod you create will have >=1 number of containers. There, each container will have its own *Ports* definition. If a port in this definition is included in the *portsToExpose* array, this port will be publicly exposed in the Node/VM. This is accomplished by the setting of a **hostPort** value for each of the container ports you want to expose. The reason we need this is that because i) you may want to use some ports on your Pod containers for other purposes rather than players connecting to it and ii) a portName must be unique within a container. Ports assigned are in the port range 10000-50000.

## CrashesToMarkUnhealthy

CrashesToMarkUnhealthy (integet) is the number of crashes that you want to trigger the GameServerBuild to become Unhealthy. Once this happens, no other operation will take place on the GameServerBuild. To allow thundernets to continue performing reconciliations on the GameServerBuild after it has become Unhealthy, you can increase the value of the CrashesToMarkUnhealthy field. The GameServerBuild will be marked as Healthy again till the number of crashes reaches the value of CrashesToMarkUnhealthy.

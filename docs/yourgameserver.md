---
layout: default
title: Running your own game server
nav_order: 5
---

# How to run your game server on Thundernetes?

You can use Thundernetes to host your game servers. This guide will walk you through on using Thundernetes i) locally using kind or ii) on Azure Kubernetes Service.

## Creating a local cluster with kind

Refer to the instructions at the [quickstart guide](quickstart/installing-kind.md) for creating a Kubernetes cluster locally with kind.

## Creating an Azure Kubernetes Service cluster

Refer to the instructions at the [quickstart guide](quickstart/installing-aks.md) for creating an Azure Kubernetes Service cluster.

## Install Thundernetes

Refer to the instructions at the [quickstart guide](quickstart/installing-thundernetes.md) about installing Thundernetes to your Kubernetes cluster.

## Run your game server locally using kind

- You are now ready to test your game server. To do this, you should first build your game server as a Linux container image. 
  - If you are using the Unity GSDK, you can take a look [here](https://github.com/PlayFab/MpsSamples/tree/master/UnityMirror#running-unity-server-as-a-linux-executable). 
  - If you are using the Unreal GSDK, check [here](https://github.com/PlayFab/gsdk/tree/master/UnrealPlugin#setting-up-a-linux-dedicated-server-on-playfab).
  - If you are using a .NET Core Game Server, you can check [here](https://github.com/PlayFab/MpsSamples/tree/master/wrappingGsdk#using-a-linux-build).
- Once you build your game server container image, you should [load it into kind](https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster). You should use a command like `kind load docker-image GAME_SERVER_IMAGE_NAME:TAG --name kind`
- Last step would be to create a GameServerBuild. To do that, you should create a yaml file with the following contents:

```yaml
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample
spec:
  titleID: "1E03" # required
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6" # must be a GUID
  standingBy: 2 # required
  max: 4 # required
  portsToExpose:
    - 80
  template:
    spec:
      containers:
        - image: thundernetes-netcore:0.2.0
          name: thundernetes-sample
          ports:
          - containerPort: 80 # your game server port
            protocol: TCP # your game server port protocol
            name: gameport # required field
```

You can call this file `gameserverbuild.yaml`.

- To configure this GameServerBuild to run on your cluster, you should run the following command:

```bash
kubectl apply -f /path/to/gameserverbuild.yaml
```

- Running `kubectl get gsb` and `kubectl get gs` should show something like this:

```bash
dgkanatsios@desktopdigkanat:digkanat$ kubectl get gsb
NAME                     STANDBY   ACTIVE   CRASHES   HEALTH
gameserverbuild-sample   2/2       0        0         Healthy
dgkanatsios@desktopdigkanat:digkanat$ kubectl get gs
NAME                           HEALTH    STATE        PUBLICIP     PORTS      SESSIONID
gameserverbuild-sample-rtgnm   Healthy   StandingBy   172.18.0.2   80:14913
gameserverbuild-sample-spdob   Healthy   StandingBy   172.18.0.2   80:14208
```

## Run your game server on Azure Kubernetes Service

As soon as you build your container image, you should publish it to a container registry. If you are using Azure Kubernetes Service, we recommend publishing your image to [Azure Container Registry](https://docs.microsoft.com/azure/container-registry/). To integrate your Azure Container Registry with your Azure Kubernetes Service cluster, check the instructions [here](https://docs.microsoft.com/azure/aks/cluster-container-registry-integration).

## Using host networking

Thundernetes supports running your GameServer Pods under host networking. To do that, you need to provide a GameServerBuild YAML like [this](http://github.com/playfab/thundernetes/tree/main/samples/netcore/sample-hostnetwork.yaml), setting the `hostNetwork` value to true on PodSpec template. During Pod creation, thundernetes controllers will override the containerPort with the same value that will be assigned in the hostPort. 

You **have to** use the generated port when you instantiate your game server process. To grab the port number, you should use GSDK.

```csharp
string ListeningPortKey = "nameOfThePortInTheGameServerBuild";
IDictionary<string, string> initialConfig = GameserverSDK.getConfigSettings();
if (initialConfig?.ContainsKey(ListeningPortKey) == true)
{
  int listeningPort = int.Parse(initialConfig[ListeningPortKey]);
  // instantiate your game server with the value of the listeningPort
```

> _**NOTE**_: Unfortunately, it is still necessary to provide a `containerPort` value in the GameServerBuild YAML, since it is required for GameServerBuild validation. However, as mentioned, this provided value is used nowhere since it's overwritten by the `hostPort` value.

## Game server image upgrades

You should **not** change the container image of your GameServerBuild. The best practice to upgrade your game server version is to

- spin up a separate GameServerBuild 
- configure your matchmaker to allocate against this new GameServerBuild
- configure the original GameServerBuild to 0 standingBy
- when all the active games in the original GameServerBuild end, you can safely delete it

However, at this point, thundernetes does not do anything to prevent you from changing the container image on the GameServerBuild YAML file, but you should consider the GameServerBuild as immutable (apart from changing the standingBy and max numbers, of course).

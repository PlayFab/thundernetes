# Architecture

This document describes the architecture of thundernetes as well as various design notes that are relevant to its implementation.

![Architecture diagram](diagram.png)

## Goal

End goal is to create a developer tool that will allow game developers to test their GSDK enabled Linux game servers. Tool will enable GameServer scaling and GameServer allocations. 

We are extending Kubernetes by creating an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). This involves creating a custom [controller](https://kubernetes.io/docs/concepts/architecture/controller/) and a couple of [Custom Resources (CRDs)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/). We are using the open source tool [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) to scaffold our operator files.

Specifically, we have two core entities in our project, which are represented by two respective CRDs:

- **GameServerBuild** ([YAML](../operator/config/crd/bases/mps.playfab.com_gameserverbuildss.yaml), [Go](../operator/api/v1alpha1/gameserverbuild_types.go)): this represents a collection of GameServers that will run the same Pod template and can be scaled in/out within the Build (i.e. add or remove instances of them). GameServers that are members of the same GameServerBuild share the same details in their execution environment, e.g. all of them could launch the same multiplayer map or the same type of game. So, you could have one GameServerBuild for a "Capture the flag" mode of your game and another GameServerBuild for a "Conquest" mode. Or, one GameServerBuild for players playing on map "X" and another GameServerBuild for players playing on map "Y". You can, however, modify how each GameServer operates via BuildMedata (configured on the GameServerBuild) or via environment variables. 
- **GameServer** ([YAML](../operator/config/crd/bases/mps.playfab.com_gameservers.yaml), [Go](../operator/api/v1alpha1/gameserver_types.go)): this represents the multiplayer game server itself. Each DedicatedGameServer has a single corresponding child [Pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/) which will run the container image containing your game server executable.

## GSDK integration

We have created a [sidecar](https://www.magalix.com/blog/the-sidecar-pattern) container that receives all the GSDK calls and modifies the GameServer state accordingly. The sidecar is also responsible of accepting the allocation calls. You can find the GSDK source code [here](https://github.com/PlayFab/gsdk) and check the docs [here](https://docs.microsoft.com/en-us/gaming/playfab/features/multiplayer/servers/integrating-game-servers-with-gsdk). The GSDK is used by game servers running in Azure PlayFab Multiplayer Servers (MPS), thus making the migration from thundernetes to MPS (and vice versa) pretty seamless.

### initcontainer

GSDK libraries are attempting to read configuration from a file (GSDKConfig.json). In order to create this file and let it be readable by the GameServer Pod, we have created a simple Kubernetes [Init Container](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) that shares a volume mount with the GSDK sidecar and will create the configuration file for it to read.

## Logging

Thundernetes does not attempt to solve the game server logging problem. There are various solutions on Kubernetes on this problem e.g. running a [fluentd](https://www.fluentd.org/) DaemonSet in the cluster which will grab the logs and forward them to an output of your choice. 

## End to end (e2e) testing

We are using [kind](https://kind.sigs.k8s.io/) and Kubernetes [client-go](https://github.com/kubernetes/client-go) library for end-to-end testing scenarios. Kind dynamically setups a Kubernetes cluster in which we create and allocate game servers and test various scenarios. Check [this](../e2e) folder for more details.

## metrics 

Thundernetes exposes various metrics regarding GameServer management in Prometheus format. To view them, you can forward traffic to port 8080 of the controller via

```bash
kubectl port-forward -n thundernetes-system deployments/thundernetes-controller-manager 8080:8080
```

Then, you can use your browser and point it to `http://localhost:8080/metrics` to view the metrics.

## Port allocation

Thundernetes requires ports in the range 10000-50000 to be open in the cluster (i.e. in the case of Azure Kubernetes Service, this port range must allow incoming traffic in the corresponding Network Security Group). Each GameServerBuild contains the portsToExpose field, which represents the port(s) that each GameServer listens to. This port is local to each GameServer container. Each container port, when the GameServer Pod is created, will be assigned a port in the range 10000-5000 (let's call it an external port) via a PortRegistry mechanism in the thundernetes controller. Game clients can send traffic to this external port. Once the GameServer session ends, the port is returned back to the pool of available ports and may be re-used in the future.

> Each port that is allocated by the PortRegistry is assigned to HostPort field of the Pod's definition. The fact that Nodes in the cluster have a Public IP makes this port accessible outside the cluster.

Noteworthy is the fact that this port range is used for all GameServerBuilds and for all Nodes in the cluster. This way, thundernetes can support up to 40000/number_of_exposed_ports GameServers per cluster game servers, i.e. if your GameServer needs only a single port, you can have up to 40k GameServers in the cluster.

## GameServer allocation

When you allocate a GameServer, thundernetes needs to do two things:

- Find a GameServer instance for the requested GameServerBuild in the StandindBy state and update it to the Active state.
- Inform the corresponding GameServer Pod (specifically, the sidecar container in that Pod) that the GameServer state is now Active. The sidecar will give this information back to the GameServer container. The way that this is accomplished is the following: each GameServer process/container regularly heartbeats (sends a JSON HTTP request) to the sidecar. When the sidecar is notified that the GameServer state has transitioned to Active, it will respond with the new state to the heartbeat coming from the GameServer container.

There are two ways we can accomplish the second step:

- Have a [Kubernetes watch](https://kubernetes.io/docs/reference/using-api/api-concepts/#efficient-detection-of-changes) from the sidecar to the Kubernetes' API server which will be notified when the GameServer is updated. This approach works well from a security perspective, since you can configure RBAC rules for the GameServer Pod.
- Have the controller's API service (which accepts the allocation requests) forward the allocation request to the sidecar. This is done via having the sidecar expose its HTTP server inside the cluster. Of course, this assumes that we trust the processes running on the containers in the cluster.

For communicating with the sidecar, we eventually picked the first approach. The second approach was used initially but was abandoned due to security concerns.
# Architecture

This document describes the architecture of thundernetes as well as various design notes that are relevant to its implementation.

![Architecture diagram](diagram.png)

## Goal

End goal is to create a developer tool that will allow game developers to test their GSDK enabled Linux game servers. Tool will enable GameServer scaling and GameServer allocations. 

We are extending Kubernetes by creating an [operator](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/). This involves creating a custom [controller](https://kubernetes.io/docs/concepts/architecture/controller/) and a couple of [Custom Resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/). We are using the open source tool [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) to scaffold our operator files.

Specifically, we have two core entities in our project, which are represented by two respective CRDs:

- **GameServerBuild** ([YAML](../operator/config/crd/bases/mps.playfab.com_gameserverbuildss.yaml), [Go](../operator/api/v1alpha1/gameserverbuild_types.go)): this represents a collection of GameServers that will run the same Pod template and can be scaled in/out within the Build (i.e. add or remove instances of them). GameServers that are members of the same GameServerBuild have a lot of similarities in their execution environment, e.g. all of them could launch the same multiplayer map or the same type of game. So, you could have one GameServerBuild for a "Capture the flag" mode of your game and another GameServerBuild for a "Conquest" mode. Or, a GameServerBuild for players playing on map "X" and a GameServerBuild for players playing on map "Y".
- **GameServer** ([YAML](../operator/config/crd/bases/mps.playfab.com_gameservers.yaml), [Go](../operator/api/v1alpha1/gameserver_types.go)): this represents the multiplayer game server itself. Each DedicatedGameServer has a single corresponding child [Pod](https://kubernetes.io/docs/concepts/workloads/pods/pod/) which will run the container image containing your game server executable.

## GSDK integration

We have created a [sidecar](https://www.magalix.com/blog/the-sidecar-pattern) container that receives all the GSDK calls and modifies the GameServer state accordingly. The sidecar is also responsible of accepting the allocation calls.

### initcontainer

GSDK libraries are reading configuration from a file (GSDKConfig.json). For this purpose, we have created a simple Kubernetes [Init Container](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/) that shares a volume mount with the GSDK sidecar and will create the configuration file for it to read.

## Logging

Thundernetes does not attempt to solve game server logging. There are various solutions on Kubernetes on this problem e.g. having a sidecar that will grab all stdout/stderr output and forward it to a logging aggregator. 

## E2E testing

We are using [kind](https://kind.sigs.k8s.io/) and Kubernetes [client-go](https://github.com/kubernetes/client-go) for end-to-end testing scenarios. Kind dynamically setups a Kubernetes cluster in which we dynamically create and allocate game servers and test various scenarios.

## metrics 

Thundernetes exposes various metrics regarding GameServer management in Prometheus format. To view them, you can forward traffic to port 8080 of the controller via

```bash
kubectl port-forward -n thundernetes-system deployments/thundernetes-controller-manager 8080:8080
```

Then, you can use your browser and point it to `http://localhost:8080/metrics` to view the metrics.
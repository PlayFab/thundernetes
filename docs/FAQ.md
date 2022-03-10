---
layout: default
title: Frequently Asked Questions
nav_order: 11
---

# Frequently Asked Questions

## Can I run a Unity or Unreal game server on thundernetes?

You can run any game server that supports the [PlayFab GameServer SDK](https://github.com/PlayFab/gsdk). Check a Unity sample [here](https://github.com/PlayFab/thundernetes/tree/main/samples/unity/README.md). On [this](https://github.com/PlayFab/MpsSamples) repository you can find samples for all programming languages GSDK supports, like C#/Java/C++/Go/Unity/Unreal.

## How can I add custom Annotations and/or Labels to my GameServer Pods?

The GameServerBuild template allows you to set custom Annotations and/or Labels along with the Pod specification. This is possible since GameServerBuild includes the entire PodTemplateSpec. Labels and Annotations are copied to the GameServers and the Pods in the GameServerBuild. Check the following YAML for an example:

```yaml
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample-netcore
spec:
  titleID: "1E03" # required
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6" # must be a GUID
  standingBy: 2 # required
  max: 4 # required
  portsToExpose:
    - containerName: thundernetes-sample-netcore # must be the same as the container name described below
      portName: gameport # must be the same as the port name described below
  template:
    metadata:
        annotations:
          annotation1: annotationvalue1
        labels:
          label1: labelvalue1
    spec:
      containers:
        - image: ghcr.io/playfab/thundernetes-netcore:0.2.0
          name: thundernetes-sample-netcore
          ports:
          - containerPort: 80 # your game server port
            protocol: TCP # your game server port protocol
            name: gameport # required field
```

## Can I run my game server pods in a non-default namespace?

You don't need to do anything special to run your game server Pods in a namespace different than "default". Old versions of thundernetes (up to 0.1) made use of a sidecar to access the Kubernetes API Server, so you needed to create special RoleBinding and ServiceAccount in the non-default namespace. With the transition to DaemonSet NodeAgent in 0.2, this is no longer necessary.

## How do I schedule thundernetes Pods and GameServer Pods into different Nodes?

In production environments, you would like to have system and thundernetes Pods (Pods that are created on the kube-system and thundernetes-system namespaces) scheduled on a different set Nodes other than the GameServer Pods. One reason for this might be that you want special Node types for your GameServers. For example, you might want to have dedicated Nodes with special GPUs for your GameServers. Another reason might be that you don't want any interruption whatsoever to Pods that are critical for the cluster to run properly (system and thundernetes Pods). One approach to achieve this isolation on public cloud providers is by using multiple Node Pools. A Node Pool is essentially a group of Nodes that share the same configuration (CPU type, memory, etc) and can be scaled independently of the others. In production scenarios, it is recommended to use three Node Pools:

- one Node Pool for Kubernetes system resources (everything in kube-system namespace) and thundernetes system resources (everything in thundernetes-system namespace)
- one Node Pool for telemetry related Pods (Prometheus, Grafana, etc)
- one Node Pool to host your GameServer Pods

Let's discuss on how to create and use a Node Pool to host the GameServer Pods.

1. First, you would need to create a separate NodePool for the GameServer Pods. Check [here](https://docs.microsoft.com/azure/aks/use-multiple-node-pools) on how to do it on Azure Kubernetes Service. Create this on "user" mode so that "kube-system" Pods are not scheduled on this NodePool. Most importantly, when creating a NodePool, you can specify custom Labels for the Nodes. Let's assume that you apply the `agentpool=gameserver` Label.
1. Use the `nodeSelector` field on your GameServer Pod spec to request that the GameServer Pod is scheduled on Nodes that have the `agentpool=gameserver` Label. Take a look at this [sample YAML file](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore/sample_second_node_pool.yaml) for an example.
1. When you create your GameServerBuild, the GameServer Pods will be scheduled on the NodePool you created.
1. Moreover, you should modify the `nodeSelector` field on the controller Pod spec to make sure it will be scheduled on the system Node Pool. On AKS, if the system Node Pool is called `nodepool1`, you should add this YAML snippet to the `thundernetes-controller-manager` Deployment on the [YAML install file](https://github.com/PlayFab/thundernetes/tree/main/installfiles/operator.yaml):

```YAML
nodeSelector:
  agentpool: nodepool1
```

You should add the above YAML snippet to any workloads you don't want to be scheduled on the GameServer NodePool. Check [here](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) for additional information on assigning pods to Nodes and check [here](https://docs.microsoft.com/azure/aks/use-system-pools#system-and-user-node-pools) for more information on AKS system and user node pools.

### Schedule DaemonSet Pods on GameServer Nodes

> For more information on the NodeAgent process running in the DaemonSet, check the architecture document [here](architecture.md#gsdk-integration).

Now that we've shown how to run multiple Node Pools, let's discuss how to schedule DaemonSet Pods running NodeAgent process to run only on Nodes that run game server Pods. Since NodeAgent's only concern is to work with game server Pods on Node's it's been scheduled, it's unnecessary to run in on Nodes that run system resources and/or telemetry. Since we have already split the cluster into multiple Node Pools, we can use the `nodeSelector` field on the DaemonSet Pod spec to request that the DaemonSet Pod is scheduled on Nodes that have the `agentpool=gameserver` Label (or whatever Label you have added to your game server Node Pool). Take a look at the following example to see how you can modify your DaemonSet YAML for this purpose:

```YAML
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: thundernetes-nodeagent
  namespace: thundernetes-system
spec:
  selector:
    matchLabels:
      name: nodeagent
  template:
    metadata:
      labels:
        name: nodeagent
    spec:
      nodeSelector: # add this line
        agentpool: gameserver # add this line as well
      containers:
      ...
```

## How do I make GameServer Pods start before DaemonSet Pods?

When a new Node is added to the Kubernetes cluster, a NodeAgent Pod (part of DaemonSet) will be created there. However, if there were pending GameServer Pods before the Node's addition to the cluster, they will also be scheduled on the new Node. Consequently, GameServer Pods might start at the same time as the NodeAgent Pod. GameServer Pods are heartbeating to the NodeAgent process so there is a chance that some heartbeats will be lost and, potentially, a state change from "" to "Initializing" will not be tracked (however, the GameServer Pod should have no trouble transitioning to StandingBy when the NodeAgent Pod is up and can process heartbeats).

There will be no impact from these lost heartbeats. However, you can tell Kubernetes to schedule NodeAgent Pods before the GameServer Pods by assigning Pod Priorities to the NodeAgent Pods. You can read more about Pod priority [here](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption) and specifically about the impact of Pod priority on scheduling order [here](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#effect-of-pod-priority-on-scheduling-order).

## How can I add resource constraints to my GameServer Pods?

Kubernetes supports resource constraints when you are creating a Pod ([reference](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)). Essentially, you can specify the amount of CPU and memory that your Pod can request when it starts (requests) as well as the maximum amount of CPU and memory that your Pod can use (limits). To configure resource constraints for your Pod, you can modify the GameServerBuild definition. Since the entire PodSpec is defined in the GameServerBuild definition, you can add these resource constraints to the PodSpec. Take a look at the following example to see how you can modify your GameServerBuild YAML for this purpose:

```yaml
template:
    spec:
      containers:
        - image: your-image:tag
          name: thundernetes-sample
          ports:
          - containerPort: 80 # your game server port
            protocol: TCP # your game server port protocol
            name: gameport # required field
          resources:
            requests:
              cpu: 100m
              memory: 500Mi
            limits:
              cpu: 100m
              memory: 500Mi
```

For a full sample, you can check [here](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore/sample-requestslimits.yaml).

## Not supported features (compared to MPS)

There are some features of MPS that are not yet supported on Thundernetes.

1. Thundernetes, for the time being, supports only Linux game servers. Work to support Windows is tracked [here](https://github.com/PlayFab/thundernetes/issues/8), please leave a comment if that's important for you. If you want to host Windows game servers, you can always use [MPS](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/).
1. On PlayFab MPS, you can upload a zip file that contains parts of your game server (referred to as assets). This is decompressed on the VM that your game server runs and is automatically mounted. You cannot do that on Thundernetes, however you can always mount a storage volume onto your Pod (e.g. check [here](https://kubernetes.io/docs/concepts/storage/volumes/#azuredisk) on how to mount an Azure Disk).

## Where does the name 'thundernetes' come from?

It's a combination of the words 'thunderhead' and 'kubernetes'. 'Thunderhead' is the internal code name for the Azure PlayFab Multiplayer Servers service. Credits to [Andreas Pohl](https://github.com/Annonator) for the naming idea!

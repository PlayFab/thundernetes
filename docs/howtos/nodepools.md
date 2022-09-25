---
layout: default
title: Specialized K8s Node Pools
parent: How to's
nav_order: 11
---

# How do I schedule Thundernetes Pods and GameServer Pods into different Node pools/groups?

In production environments, it is a best practice to have system and Thundernetes Pods (Pods that are created on the kube-system and thundernetes-system namespaces) scheduled on a different set of Nodes other than the GameServer Pods.

Some of the reasons for this are:

- You might want special Node types for your GameServers. For example, you might want to have dedicated Nodes with special GPUs for your GameServers or just bigger (in terms of CPU/RAM).
- Moreover, you might not want any interruption whatsoever to Pods that are critical for the cluster to run properly (system and thundernetes Pods). 
- Public IP per Node feature (required for Thundernetes game servers) can only be set on the Node pool that the GameServer Pods are scheduled on, thus making the system pods more secure.
- Last but not least, you might want to scale the GameServer Nodes independently of the system Nodes.

One approach to achieve this isolation on public cloud providers is by using multiple Node Pools/Groups. A Node Pool is essentially a group of Nodes that share the same configuration (CPU type, memory, etc) and can be scaled independently of the others. In production scenarios, it is recommended to use three Node Pools:

- one Node Pool for Kubernetes system resources (everything in kube-system namespace) and Thundernetes system resources (everything in thundernetes-system namespace)
- one Node Pool for telemetry related Pods (Prometheus, Grafana, etc)
- one Node Pool to host your GameServer Pods

For example, on Azure Kubernetes Service you can use the following guidelines:

1. Create a separate NodePool to host your GameServer Pods. Check [here](https://docs.microsoft.com/azure/aks/use-multiple-node-pools) on how to do it on Azure Kubernetes Service. Create this on "user" mode so that "kube-system" Pods are not scheduled on this NodePool. Moreover, when creating a NodePool, you can specify custom Labels for the Nodes. Let's assume that you apply the `agentpool=gameserver` Label.
2. Use the `nodeSelector` field on your GameServer Pod spec to request that the GameServer Pod is scheduled on Nodes that have the `agentpool=gameserver` Label. Take a look at this [sample YAML file](https://github.com/PlayFab/thundernetes/blob/main/samples/netcore/sample-secondnodepool.yaml) for an example.
3. When you create your GameServer Pods, these will be scheduled on the NodePool you created.
4. You should also modify the `nodeSelector` field on the controller Pod spec to make it will be scheduled on the system Node Pool. On AKS, if the NodePool is called `nodepool1`, you should add this YAML snippet to the `thundernetes-controller-manager` Deployment on the [YAML install file](https://github.com/PlayFab/thundernetes/tree/main/installfiles/operator.yaml):

{% include code-block-start.md %}nodeSelector:
     agentpool: nodepool1
{% include code-block-end.md %}

You should add the above YAML snippet to any workloads you don't want to be scheduled on the GameServer NodePool. Check [here](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) for additional information on assigning pods to Nodes and check [here](https://docs.microsoft.com/azure/aks/use-system-pools#system-and-user-node-pools) for more information on AKS system and user node pools.

## Schedule DaemonSet Pods on GameServer Nodes

Now that we've shown how to run multiple Node Pools, let's discuss how to schedule DaemonSet Pods running NodeAgent process to run only on Nodes that run game server Pods. NodeAgent's goal is to listen and respond to heartbeats from the gameservers and notify Kubernetes API server for any updates on their status. Since each NodeAgent instance has to communicate with game server Pods on the Node it's been scheduled, it's unnecessary to run it on Nodes that run system resources and/or telemetry.

Since we have already split the cluster into multiple Node Pools, we can use the `nodeSelector` field on the DaemonSet Pod spec to request that the DaemonSet Pod is scheduled on Nodes that have the `agentpool=gameserver` Label (or whatever Label you have added to your game server Node Pool). Take a look at the following example to see how you can modify your DaemonSet YAML for this purpose:

{% include code-block-start.md %}
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
{% include code-block-end.md %}

> _**NOTE**_: For more information on the NodeAgent process running in the DaemonSet, check the architecture document [here](../architecture.md#gsdk-integration).

## Limit the number of available ports

Thundernetes needs to provide dedicated port numbers for the GameServer ports that you need exposed to the internet (i.e. the ports that game clients will connect to). For each VM, Thundernetes provides a single port in the range of 10000-12000. By default, the PortRegistry mechanism inside Thundernetes will allocate a set of 10000-12000 range for every VM in the cluster. However, if you are using a dedicated Node Pool for your GameServers, you'd need to specify this to the Thundernetes controller to avoid assigning ports for more VMs that you can have and lead to pending Pods. There are two steps you need to to perform this:

- Label the GameServer Nodes with the `mps.playfab.com/gameservernode=true` Label.
- Controller YAML deployment needs to be updated by adding the `PORT_REGISTRY_EXCLUSIVELY_GAMESERVER_NODES` environment variable with the value of `"true"`.

---
layout: default
title: Cluster Autoscaling
parent: How to's
nav_order: 4
---

# Cluster Autoscaling

Thundernetes natively supports GameServer autoscaling via its standingBy/max mechanism. However, scaling GameServer Pods is just one part of the scaling process. The other part is about scaling the Kubernetes Nodes in the cluster. The challenges with Node autoscaling are mostly with scaling down. Nodes that have at least one GameServer in the Active state (where players are connected to the GameServer and playing the game) should not be removed, since this would effectively disconnect all users and destroy their experience. Nodes with GameServers on Initializing or StandingBy state are eligible to be removed, since no players are connected. We should also keep in mind that multiplayer game sessions are usually short lived, so Active game servers will eventually be terminated and new game servers in Initializing or StandingBy state will be created.

For Node autoscaling, Thundernetes can work with the open source [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler). To let Cluster Autoscaler be aware of the state of the Pods, Thundernetes adds the [safe-to-evict=false](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-types-of-pods-can-prevent-ca-from-removing-a-node) annotation to Active game server Pods and `safe-to-evict=true` to Pods in the Initializing or StandingBy state.

We also recommend using the [overprovisioning feature](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler) so you can spin up Nodes as soon as possible. Since the Cluster Autoscaler will create a new Node only when there are Pods in the Pending state and a new Node addition might take a couple of minutes, it might be desirable to have some Pods that just reserve resources with a negative PriorityClass, so these are the first ones that will end up on a Pending State. 

Each cloud provider has its own documentation for using the cluster autoscaler. If you are using Azure Kubernetes Service, you can easily enable cluster autoscaler using the documentation [here](https://docs.microsoft.com/azure/aks/cluster-autoscaler).

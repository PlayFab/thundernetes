---
layout: default
title: Cluster Autoscaling
parent: How to's
nav_order: 5
---

# Cluster Autoscaling

Thundernetes natively supports GameServer autoscaling via its standingBy/max mechanism. However, scaling Pods is just one part of the process. The other part is about scaling the Kubernetes Nodes in the cluster. For Node autoscaling, thundernetes can work with the open source [Kubernetes cluster autoscaler](https://github.com/kubernetes/autoscaler). We also recommend using the [overprovisioning feature](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler) so you can spin up Nodes as soon as possible. Each cloud provider has its own documentation for using the cluster autoscaler. If you are using Azure Kubernetes Service, you can easily enable cluster autoscaler using the documentation [here](https://docs.microsoft.com/azure/aks/cluster-autoscaler).

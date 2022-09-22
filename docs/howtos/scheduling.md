---
layout: default
title: Efficient scheduling
parent: How to's
nav_order: 13
---

# Efficient scheduling

By default, Pods are scheduled using the [Kubernetes scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/). Generally, its behavior is to spread the Pods into as many Nodes as possible. However, if you are using a cloud provider (e.g. Azure Kubernetes Service), you'd want to schedule your Game Server Pods into the less amount of Nodes possible, to save on costs. For example, if you have two VMs, you'll want to schedule the Pods on VM 1 till it can't host any more, then you'll start scheduling the Pods to VM 2. The reason for doing that is that on a potential cluster scale-down you will want to have Nodes with zero (or close to zero) Pods in the Active, so they can be efficiently reclaimed by the underlying cloud provider. To accomplish this type of tight scheduling, you can use the [Kubernetes inter-pod affinity strategy](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) when defining your Pod on the GameServerBuild.

To instruct the Kubernetes scheduler to try and schedule Pods into as few Nodes as possible you can use something like the following:

{% include code-block-start.md %}
  template:
    spec:
      affinity:
        podAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: BuildID
                  operator: In
                  values:
                  - "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6"
              topologyKey: "kubernetes.io/hostname"
{% include code-block-end.md %} 

To test this behavior check the [sample-nodeaffinity.yaml](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore/sample-nodeaffinity.yaml) file.
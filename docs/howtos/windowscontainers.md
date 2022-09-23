---
layout: default
title: Windows containers
parent: How to's
nav_order: 9
---

# Windows containers

To further expand Thundernetes and to be able to meet the demands of as many developers as possible, we now support GameServerBuilds that use Windows containers. This is possible thanks to Kubernetes' progress in this area, but there are still some features that are not fully supported on Windows containers, you can read more about this [here](https://kubernetes.io/docs/setup/production-environment/windows/intro-windows-in-kubernetes/). This means, for example, that some Thundernetes features like the `hostNetwork` option for pods, are only available on Linux containers for the time being.

## Requirements

- Your cluster must use Kubernetes version 1.23 or higher.
- Your cluster must have nodes that run Windows Server 2019 or higher.

## How it works

Currently Kubernetes still needs Linux machines to run the control plane, which is the component that actually does the orchestration of all the pods. What Kubernetes does support is the use of Windows worker nodes that are able to run Windows containers. This is mandatory because currently it is not possible to run Windows containers on Linux.

In the case of Thundernetes, the controller still runs only on Linux machines, same as with the control plane. What was added is two Windows versions of currently existing components that run on the worker nodes, that now can be Windows machines. These components are:

- **NodeAgent**: an application that constantly checks the pods on each node and communicates with the controller.
- **initcontainer**: an application that writes the necessary files on each pod for GSDK to run properly.

When you install Thundernetes in a cluster with Windows worker nodes, it will automatically use the correct version of both of these components on each node.

On the game server side, all you need to do is integrate your project with the Game Server SDK ([GSDK](../gsdk/README.md)), same as with Linux game servers. When you want to deploy a new game server build, simply add the following to the YAML file, we use this to know how to deploy the game servers correctly:

{% include code-block-start.md %}
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample
spec:
  ...
  template:
      spec:
        nodeSelector:
          kubernetes.io/os: windows
    ...
{% include code-block-end.md %}
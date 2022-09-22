---
layout: default
title: kind (Kubernetes In Docker)
parent: Create Kubernetes cluster
grand_parent: Quickstart
nav_order: 2
---

# Installing Kubernetes locally with kind

You can use a variety of options to run Kubernetes locally, either [kind](https://kind.sigs.k8s.io/) or [k3d](https://k3d.io/) or [minikube](https://kubernetes.io/docs/getting-started-guides/minikube/). In this guide, we will use [kind](https://kind.sigs.k8s.io/).

* Kind requires Docker, so make sure you have it up and running. You can find [Docker for Windows here](https://docs.docker.com/desktop/windows/install/)
* Install kind using the instructions [here](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* Create a configuration file to configure the cluster, using the contents listed below. 

Special attention is needed on the ports you will forward (the "containerPort" listed below). First of all, you need to expose port 5000 since this is the port used by the Thundernetes GameServer allocation API service. You will use this port to do game server allocations.
After that, you can optionally specify ports to test your game server by sending traffic to it. Thundernetes dynamically allocates ports for your game server, ranging from 10000 to 12000. Port assignment from this range is sequential. For example, if you use two game servers with each one having a single port, the first game server port will be mapped to port 10000 and the second will be mapped to port 10001. Be aware that if you scale down your GameServerBuild and scale it up again, you probably will not get the same port. Consequently, pay special attention to the ports that you will use in your kind configuration.

Save this content to a file called `kind-config.yaml`. This configuration will create a cluster with a single node.

{% include code-block-start.md %}
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  extraPortMappings:
  - containerPort: 5000
    hostPort: 5000
    listenAddress: "0.0.0.0"
    protocol: tcp
  - containerPort: 10000
    hostPort: 10000
    listenAddress: "0.0.0.0"
    protocol: tcp
  - containerPort: 10001
    hostPort: 10001
    listenAddress: "0.0.0.0"
    protocol: tcp
{% include code-block-end.md %}

* Run `kind create cluster --config /path/to/kind-config.yaml`
* Install kubectl ([instructions](https://kubernetes.io/docs/tasks/tools/#kubectl)) to manage your Kubernetes cluster
* Once it succeeds, run `kubectl cluster-info` to verify that the cluster is running. You should get something like this:

{% include code-block-start.md %}
Kubernetes control plane is running at https://127.0.0.1:34253
CoreDNS is running at https://127.0.0.1:34253/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
{% include code-block-end.md %}

Once you make sure cluster has been installed and operates smoothly, you can proceed to the [installing Thundernetes](./installing-thundernetes.md) section.
# Frequently Asked Questions

## Pod scheduling

By default, Pods are scheduled using the Kubernetes scheduler. Its behavior is to spread the Pods into as many Nodes as possible. However, if you are using a cloud provider (e.g. Azure Kubernetes Service), you'd want to schedule your Game Server Pods into the less amount of Nodes possible. For example, if you have two VMs, you'll want to schedule the Pods on VM 1 till it can't host any more, then you'll schedule the Pods to VM 2. The reason for doing that is that on a potential cluster scale-down you will want to have Nodes with zero (or close to zero) Pods, so they can be effiently reclaimed by the underlying cloud provider. To accomplish this type of tight scheduling, you can use the [Kubernetes inter-pod affinity strategy](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity) when defining your Pod.

To instruct the Kubernetes scheduler to try and schedule Pods into as few Nodes as possible you can use something like the following:

``` yaml
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
``` 

To test this behavior check the [sample-nodeaffinity.yaml](../samples/netcore/sample-nodeaffinity.yaml) file.

## How can I find the Public IP address from inside a GameServer?

The GSDK call (GetGameServerConnectionInfo) that returns Game Server connection information currently returns the internal IP of the Node. However, you can easily get the Public IP address by using one of the following web sites from your game server:

```bash
curl http://canhazip.com
curl http://whatismyip.akamai.com/
curl https://4.ifcfg.me/
curl http://checkip.amazonaws.com
curl -s http://whatismijnip.nl | awk '{print $5}'
curl -s icanhazip.com
curl ident.me
curl ipecho.net/plain
curl wgetip.com
curl ip.tyk.nu
curl bot.whatismyipaddress.com
wget -q -O - checkip.dyndns.org | sed -e 's/[^[:digit:]\|.]//g'
```

The above methods would work properly if the Node hosting your Pod has a Public IP.

[source](https://serversuit.com/community/technical-tips/view/finding-your-external-ip-address.html)

If you want a programmatic way to get the Public IP address, please leave a comment on [this issue](https://github.com/PlayFab/thundernetes/issues/136).

## Get GameServer logs

Thundernetes does not do anything special to obtain the logs for your GameServer Pods, since there already are a lot of solutions in the Kubernetes ecosystem for this purpose. One of easiest ways to do this is to use [fluentbit](https://fluentbit.io/) to capture logs and send them to [Azure Blob Storage](https://docs.microsoft.com/azure/storage/blobs/storage-blobs-overview) or on a Storage provide of your choice (you can see output providers for fluentbit [here](https://docs.fluentbit.io/manual/pipeline/outputs)).

You can use the following steps to setup fluentbit to capture logs from your GameServer Pods and send them to Azure Storage:

- Set up an Azure Storage Account. Check [here](https://docs.microsoft.com/azure/storage/common/storage-account-create?tabs=azure-portal) on how to do it using the Azure Portal.
- Install fluentbit on your Kubernetes cluster. Check [here](https://docs.fluentbit.io/manual/installation/kubernetes) on how to do it using the Azure Portal.
- As soon as you create the namespace and roles/role bindings, you should create the fluentbit ConfigMap containing the fluentbit configuration file. You can see a sample [here](../samples/fluentbit/fluent-bit-configmap.yaml). Remember to replace the values with your Azure Storage Account name and key.
- Finally, you should create the fluentbit DaemonSet, so a fluentbit Pod runs on every Node in your cluster and grabs the logs. You can find a sample [here](../samples/fluentbit/fluent-bit-ds.yaml).

## Node Autoscaling

Thundernetes natively supports GameServer autoscaling via its standingBy/max mechanism. However, scaling Pods is just one part of the process. The other part is about scaling the Kubernetes Nodes in the cluster. For Node autoscaling, thundernetes can work with the open source [Kubernetes cluster autoscaler](https://github.com/kubernetes/autoscaler). We also recommend using the [overprovisioning feature](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler) so you can spin up Nodes as soon as possible. Each cloud provider has its own documentation for using the cluster autoscaler. If you are using Azure Kubernetes Service, you can easily enable cluster autoscaler using the documentation [here](https://docs.microsoft.com/azure/aks/cluster-autoscaler).

## Can I run a Unity or Unreal game server on thundernetes?

You can run any game server that supports the [PlayFab GameServer SDK](https://github.com/PlayFab/gsdk). Check a Unity sample [here](../samples/unity/README.md). On [this](https://github.com/PlayFab/MpsSamples) repository you can find samples for all programming languages GSDK supports, like C#/Java/C++/Go/Unity/Unreal.

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
1. Use the `nodeSelector` field on your GameServer Pod spec to request that the GameServer Pod is scheduled on Nodes that have the `agentpool=gameserver` Label. Take a look at this [sample YAML file](../samples/netcore/sample_second_node_pool.yaml) for an example.
1. When you create your GameServerBuild, the GameServer Pods will be scheduled on the NodePool you created.
1. Moreover, you should modify the `nodeSelector` field on the controller Pod spec to make sure it will be scheduled on the system Node Pool. On AKS, if the system Node Pool is called `nodepool1`, you should add this YAML snippet to the `thundernetes-controller-manager` Deployment on the [YAML install file](../installfiles/operator.yaml):

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

For a full sample, you can check [here](../samples/netcore/sample-requestslimits.yaml).

## Not supported features (compared to MPS)

There are some features of MPS that are not yet supported on Thundernetes.

1. Thundernetes, for the time being, supports only Linux game servers. Work to support Windows is tracked [here](https://github.com/PlayFab/thundernetes/issues/8), please leave a comment if that's important for you. If you want to host Windows game servers, you can always use [MPS](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/).
1. On PlayFab MPS, you can upload a zip file that contains parts of your game server (referred to as assets). This is decompressed on the VM that your game server runs and is automatically mounted. You cannot do that on Thundernetes, however you can always mount a storage volume onto your Pod (e.g. check [here](https://kubernetes.io/docs/concepts/storage/volumes/#azuredisk) on how to mount an Azure Disk).

### Deleting namespace thundernetes-system stuck in terminating state

Thundernetes creates finalizers for the GameServer custom resource. So, if you delete the thundernetes controller and you try to remove the GameServer Pods and/or the namespace they are in, the namespace might be stuck in terminating state. To fix this, you can run the following commands:

```bash
 kubectl get namespace thundernetes-system -o json > tmp.json
```

Open tmp.json file and find this section:

```json
    "spec": {
        "finalizers": [
            "kubernetes"
        ]
    },
    "status": {
        "phase": "Active"
    }
```

Remove the finalizer section:

```json
 "spec": {

   },
   "status": {
     "phase": "Terminating"
   }
```

Upload the json file:

```bash
kubectl proxy # this command is blocking, so you can either run it on background or use a different shell for the next commands
curl -k -H "Content-Type: application/json" -X PUT --data-binary @tmp.json http://127.0.0.1:8001/api/v1/namespaces/thundernetes-system/finalize
kubectl get ns # verify that the namespace is gone
```

For more information about deleting namespaces stuck in terminating state check the [link](https://www.ibm.com/docs/en/cloud-private/3.2.0?topic=console-namespace-is-stuck-in-terminating-state).

## Where does the name 'thundernetes' come from?

It's a combination of the words 'thunderhead' and 'kubernetes'. 'Thunderhead' is the internal code name for the Azure PlayFab Multiplayer Servers service. Credits to [Andreas Pohl](https://github.com/Annonator) for the naming idea!

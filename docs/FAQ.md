# Frequently Asked Questions

## Pod scheduling

By default, Pods are scheduled using the Kubernetes scheduler. However, if you are using a cloud provider (e.g. Azure Kubernetes Service), you'd want to schedule your Game Server Pods as tight as possible. For example, if you have two VMs, you'll want to schedule the Pods on VM 1 till it can't host any more, then you'll schedule the Pods to VM 2. To do that, you can use the [Kubernetes inter-pod affinity strategy](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity).

By default GameServer application pods may schedule on different kubernetes node due nature of kubernetes default scheduler. To optimize and schedule the GameServer pods on the same node using PodAffinity can be beneficial in the PodSpec of CRD. Checkout this sample:

``` yaml
  podSpec:
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

The GSDK call to get the Public IP is not supported at this time, it returns "N/A". However, you can easily get the Public IP address by using one of the following web sites from your game server:

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

## Grab GameServer logs

One of easiest ways to grab logs from your GameServer Pods is to use [fluentbit](https://fluentbit.io/) to capture logs and send them to [Azure Blob Storage](https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-overview).

You can use the following steps to setup fluentbit to capture logs from your GameServer Pods:

- Set up an Azure Storage Account. Check [here](https://docs.microsoft.com/en-us/azure/storage/common/storage-account-create?tabs=azure-portal) on how to do it using the Azure Portal.
- Install fluentbit on your Kubernetes cluster. Check [here](https://docs.fluentbit.io/manual/installation/kubernetes) on how to do it using the Azure Portal.
- As soon as you create the namespace and roles/role bindings, you should create the fluentbit ConfigMap containing the fluentbit configuration file. You can see a sample [here](../samples/fluentbit/fluent-bit-configmap.yaml). Remember to replace the values with your Azure Storage Account name and key.
- Finally, you should create the fluentbit DaemonSet, so a fluentbit Pod runs on every Node in your cluster and grabs the logs. You can find a sample [here](../samples/fluentbit/fluent-bit-ds.yaml).

## Node Autoscaling

Scaling in Kubernetes is two fold. Pod autoscaling and Cluster autoscaling. Thundernetes enables pod autoscaling by default utilizing the standby mechanism. For Node autoscaling, Kubernetes cluster autoscaler can be potentially used, especially with the use of [overprovisioning](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler). If you are using Azure Kubernetes Service, you can [easily enable cluster autoscaler](https://docs.microsoft.com/en-us/azure/aks/cluster-autoscaler).

## Virtual Kubelet

In conjuction with cluster autoscaler, you can use [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project to accelerate the addition of new Pods to the cluster. If you are using Azure Kubernetes Service, you can easily enable Virtual Nodes feature (which is based on Virtual Kubelet) using the instructions [here](https://docs.microsoft.com/en-us/azure/aks/virtual-nodes).

## Can I run my game server pods in a non-default namespace?

You don't need to anything special to run your game server Pods in a namespace different than the "default". Old versions of thundernetes (up to 0.1) made use of a sidecar to access the Kubernetes API Server, so you need to create special RoleBinding and ServiceAccount in your namespace. With the transition to DaemonSet NodeAgent in 0.2, this is no longer necessary.

## How do I schedule thundernetes Pods and GameServer Pods into different Nodes?

There might be cases in which you would like to have system and operator Pods (Pods that are created on the kube-system and thundernetes-system namespaces) and your GameServer Pods scheduled on different Nodes. One reason for this might be that you want special Node types for your GameServers. For example, you might want to have a dedicated Node for your GameServers that are dependent on a special GPU. Another reason might be that you don't want any interruption whatsoever to Pods that are critical for the cluster to run properly. One approach to achieve this isolation is:

1. Create a separate NodePool to host your GameServer Pods. Check [here](https://docs.microsoft.com/en-us/azure/aks/use-multiple-node-pools) on how to do it on Azure Kubernetes Service. Create this on "user" mode so that "kube-system" Pods are not scheduled on this NodePool. When creating a NodePool, you can specify custom Labels for the Nodes. Let's assume that you apply the `agentpool=gameserver` Label.
1. Use the `nodeSelector` field on your GameServer Pod spec to request that the GameServer Pod is scheduled on Nodes that have the `agentpool=gameserver` Label. Take a look at this [sample YAML file](../samples/netcore/sample_second_node_pool.yaml) for an example.
1. When you create your GameServer Pods, those will be scheduled on the NodePool you created.
1. You should also modify the `nodeSelector` field on the controller Pod spec to make it will be scheduled on the system Node Pool. On AKS, if the NodePool is called `nodepool1`, you should add this YAML snippet to the `thundernetes-controller-manager` Deployment on the [YAML install file](../installfiles/operator.yaml):

```YAML
nodeSelector:
  agentpool: nodepool1
```

You should add this YAML snippet to any workloads you don't want to be scheduled on the GameServer NodePool. Check [here](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) for additional information on assigning pods to Nodes and check [here](https://docs.microsoft.com/en-us/azure/aks/use-system-pools#system-and-user-node-pools) for more information on AKS system and user node pools.

## Not supported features (compared to MPS)

There are some features of MPS that are not yet supported on Thundernetes.

1. Thundernetes, for the time being, supports only Linux game servers. Work to support Windows is tracked in #8, please leave a comment if that's important for you.
1. On PlayFab MPS, you can upload a zip file that contains parts of your game server (referred to as assets). This is decompressed on the VM that your game server runs and is automatically mounted. You cannot do that on Thundernetes, however you can always mount a storage volume onto your Pod (e.g. check [here](https://kubernetes.io/docs/concepts/storage/volumes/#azuredisk) on how to mount an Azure Disk). Work tracked in #13.

### Deleting namespace thundernetes-system stuck in terminating state

Thundernetes creates finalizers for the GameServer custom resource. So, if you delete the thundernetes controller and you try to remove the GameServer Pods and/or the namespace they are in, the namespace will be stuck in terminating state. To fix this, you can run the following commands:

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
kubectl proxy
curl -k -H "Content-Type: application/json" -X PUT --data-binary @tmp.json http://127.0.0.1:8001/api/v1/namespaces/thundernetes-system/finalize
kubectl get ns
```

For more information about deleting namespaces stuck in terminating state check the [link](https://www.ibm.com/docs/en/cloud-private/3.2.0?topic=console-namespace-is-stuck-in-terminating-state).

## Where does the name 'thundernetes' come from?

It's a combination of the words 'thunderhead' and 'kubernetes'. 'Thunderhead' is the internal code name for the Azure PlayFab Multiplayer Servers service. Credits to [Andreas Pohl](https://github.com/Annonator) for the naming idea!

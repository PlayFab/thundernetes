# Frequently Asked Questions

## Pod scheduling

By default, Pods are scheduled using the Kubernetes scheduler. However, if you are using a cloud provider (e.g. Azure Kubernetes Service), you'd want to schedule your Game Server Pods as tight as possible. For example, if you have two VMs, you'll want to schedule the Pods on VM 1 till it can't host any more, then you'll schedule the Pods to VM 2. To do that, you can use the [Kubernetes inter-pod affinity strategy](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#inter-pod-affinity-and-anti-affinity).

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

The above methods work since the Node hosting your Pod has a Public IP.

[source](https://serversuit.com/community/technical-tips/view/finding-your-external-ip-address.html)

## Node Autoscaling

Scaling in Kubernetes is two fold. Pod autoscaling and Cluster autoscaling. Thundernetes enables pod autoscaling by default utilizing the standby mechanism. For Node autoscaling, Kubernetes cluster autoscaler can be potentially used, especially with the use of [overprovisioning](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#how-can-i-configure-overprovisioning-with-cluster-autoscaler). If you are using Azure Kubernetes Service, you can [easily enable cluster autoscaler](https://docs.microsoft.com/en-us/azure/aks/cluster-autoscaler).

## Virtual Kubelet

In conjuction with cluster autoscaler, you can use [Virtual Kubelet](https://github.com/virtual-kubelet/virtual-kubelet) project to accelerate the addition of new Pods to the cluster. If you are using Azure Kubernetes Service, you can easily enable Virtual Nodes feature (which is based on Virtual Kubelet) using the instructions [here](https://docs.microsoft.com/en-us/azure/aks/virtual-nodes).

## How can I run my game server pods in a non-default namespace?

By default, thundernetes monitors the `default` namespace. If you want to run your game servers in a different namespace, you should first install the necessary ServiceAccount/RoleBinding RBAC roles on this namespace. This is because the sidecar running on the GameServer Pod needs access to talk to the Kubernetes API server. For information on Kubernetes RBAC, check [here](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

You can save the following configuration on a yaml file and then run `kubectl apply -f /path/to/file.yaml` to create the namespace and RBAC objects

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mynamespace
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gameserver-editor
  namespace: mynamespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: gameserver-editor-rolebinding
  namespace: mynamespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gameserver-editor-role
subjects:
- kind: ServiceAccount
  name: gameserver-editor
  namespace: mynamespace  
```

## Not supported features (compared to MPS)

There are some features of MPS that are not yet supported on Thundernetes.

1. Thundernetes, for the time being, supports only Linux game servers.
1. On PlayFab MPS, you can upload a zip file that contains parts of your game server (referred to as assets). This is decompressed on the VM that your game server runs and is automatically mounted. You cannot do that on Thundernetes, however you can always mount a storage volume onto your Pod (e.g. check [here](https://kubernetes.io/docs/concepts/storage/volumes/#azuredisk) on how to mount an Azure Disk).

## Where does the name 'thundernetes' come from?

It's a combination of the words 'thunderhead' and 'kubernetes'. 'Thunderhead' is the internal code name for the Azure PlayFab Multiplayer Servers service. Credits to [Andreas Pohl](https://github.com/Annonator) for the naming idea!
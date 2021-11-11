## How do I schedule thundernetes Pods and GameServer Pods into different Nodes?

There might be cases in which you would like to have system and operator Pods (Pods that are created on the kube-system and thundernetes-system namespaces) and your GameServer Pods scheduled on different Nodes. One reason for this might be that you want special Node types for your GameServers. For example, you might want to have a dedicated Node for your GameServers that are dependent on a special GPU. Another reason might be that you don't want any interruption whatsoever to Pods that are critical for the cluster to run properly. One approach to achieve this isolation is:

1. Create a separate NodePool to host your GameServer Pods. Check here on how to do it on Azure Kubernetes Service. Create this on "user" mode so that "kube-system" Pods are not scheduled on this NodePool. When creating a NodePool, you can specify custom Labels for the Nodes. Let's assume that you apply the ```agentpool=gameserver``` Label.
2. Use the ```nodeSelector``` field on your GameServer Pod spec to request that the GameServer Pod is scheduled on Nodes that have the ```agentpool=gameserver``` Label. Take a look at this sample YAML file for an example.
3. When you create your GameServer Pods, those will be scheduled on the NodePool you created.
4. You should also modify the ```nodeSelector``` field on the controller Pod spec to make it will be scheduled on the system Node Pool. On AKS, if the NodePool is called nodepool1, you should add this YAML snippet to the ```thundernetes-controller-manager``` Deployment on the [YAML install file](https://github.com/PlayFab/thundernetes/blob/39f3c8bb3cb4d4f95d128a9700be72cb1014cd21/installfiles/operator.yaml):
```nodeSelector:
  agentpool: nodepool1
```
You should add this YAML snippet to any workloads you don't want to be scheduled on the GameServer NodePool. Check here for additional information on assigning pods to Nodes and check here for more information on AKS system and user node pools.
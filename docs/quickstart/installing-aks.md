---
layout: default
title: Azure Kubernetes Service
parent: Create Kubernetes cluster
grand_parent: Quickstart
nav_order: 1
---

# Creating an AKS cluster

You can create an Azure Kubernetes Service cluster using the [Azure portal](https://portal.azure.com/). The only extra thing you need to do is enable Public IP Per Node feature for your Node Pool. This feature can only be enabled during the creation of the Node Pool so to do that, make sure you specifically select the Node Pool during creation and activate the feature. 

Once your cluster is created, you can [open the necessary ports to the Internet](#expose-ports-10000-12000-to-the-internet).

> _**NOTE**_: If you don't have an Azure subscription, you can [sign up for the free offer](https://azure.com/free)

## Create an Azure Kubernetes Service cluster using Azure CLI

Alternatively, you can use the following [Azure CLI](https://docs.microsoft.com/cli/azure/) commands to create an Azure Kubernetes Service (AKS) cluster with a Public IP per Node.

{% include code-block-start.md %}
az login # you don't need to do this if you're using Azure Cloud shell
# you should modify these values with your preferred ones
AKS_RESOURCE_GROUP=thundernetes # name of the resource group AKS will be installed
AKS_NAME=thundernetes # AKS cluster name
AKS_LOCATION=westus2 # AKS datacenter region
AKS_VERSION=1.22.4 # replace with the Kubernetes version that is supported in the region

# create a resource group
az group create --name $AKS_RESOURCE_GROUP --location $AKS_LOCATION
# create a new AKS cluster enabling the feature of Public IP per Node
az aks create --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --ssh-key-value ~/.ssh/id_rsa.pub --kubernetes-version $AKS_VERSION --enable-node-public-ip
# get credentials for this cluster, saving them in a separate file
az aks get-credentials --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --file ~/.kube/config-thundernetes
# make sure we're using the new cluster credentials
export KUBECONFIG=~/.kube/config-thundernetes
# check that cluster is up and running
kubectl cluster-info # get cluster information
kubectl get nodes # get list of nodes
kubectl get pods -n kube-system # get list of pods on the system namespace
{% include code-block-end.md %}

Last command requires you to use `kubectl`, the CLI tool for Kubernetes. Check the [instructions](https://kubernetes.io/docs/tasks/tools/#kubectl) about how to download and use it.

### Expose ports 10000-12000 to the Internet

Thundernetes requires VMs to have Public IPs (so game servers can be accessible) and be able to accept network traffic at port range 10000-12000 from the Internet.

> _**NOTE**_: This port range is configurable, check [here](../howtos/customportrange.md) for details. 

To allow traffic to these ports, you need to perform the following steps *after your AKS cluster gets created*:

* Login to the [Azure Portal](https://portal.azure.com)
* Find the resource group where the AKS resources are kept, it should have a name like `MC_resourceGroupName_AKSName_location`. Alternative, you can type `az resource show --namespace Microsoft.ContainerService --resource-type managedClusters -g $AKS_RESOURCE_GROUP -n $AKS_NAME -o json | jq .properties.nodeResourceGroup` on your shell to find it.
* Find the Network Security Group object, which should have a name like `aks-agentpool-********-nsg`
* Select **Inbound Security Rules**
* Select **Add** to create a new Rule with **Any** as the protocol (you could also select between TCP or UDP, depending on your game) and **10000-12000** as the Destination Port Ranges. Pick a proper name for the rule and leave everything else at their default values

Alternatively, you can use the following command, after setting the `$RESOURCE_GROUP_WITH_AKS_RESOURCES` and `$NSG_NAME` variables with proper values:

{% include code-block-start.md %}
az network nsg rule create \
  --resource-group $RESOURCE_GROUP_WITH_AKS_RESOURCES \
  --nsg-name $NSG_NAME \
  --name AKSThundernetesGameServerRule \
  --access Allow \
  --protocol "*" \
  --direction Inbound \
  --priority 1000 \
  --source-port-range "*" \
  --destination-port-range 10000-12000
{% include code-block-end.md %}

Once you make sure cluster has been installed and operates smoothly, you can proceed to the [installing Thundernetes](./installing-thundernetes.md) section.

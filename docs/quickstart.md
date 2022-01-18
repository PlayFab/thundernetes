# Quickstart

We've tested thundernetes on Azure Kubernetes Service (AKS) version 1.20.7 and 1.20.9 and [kind](https://kind.sigs.k8s.io/) but it can theoretically be installed on any Kubernetes cluster, optionally supporting Public IP per Node (which is something you want if you want to expose your game servers outside the cluster). Read the relevant section depending on where you want to install thundernetes.

> If you are using Windows, we recommend using [Windows Subsystem for Linux](https://docs.microsoft.com/windows/wsl/install-win10) to run the CLI commands listed below.

## Create an Azure Kubernetes Service cluster with a Public IP per Node

We can use the following [Azure CLI](https://docs.microsoft.com/cli/azure/) commands to create an Azure Kubernetes Service (AKS) cluster with a Public IP per Node.

```bash
az login # you don't need to do this if you're using Azure Cloud shell
# you should modify these values with your preferred ones
AKS_RESOURCE_GROUP=thundernetes # name of the resource group AKS will be installed
AKS_NAME=thundernetes # AKS cluster name
AKS_LOCATION=westus2 # AKS datacenter location

# create a resource group
az group create --name $AKS_RESOURCE_GROUP --location $AKS_LOCATION
# create a new AKS cluster enabling the feature of Public IP per Node
az aks create --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --ssh-key-value ~/.ssh/id_rsa.pub --kubernetes-version 1.20.9 --enable-node-public-ip
# get credentials for this cluster
az aks get-credentials --resource-group $AKS_RESOURCE_GROUP --name $AKS_NAME --file ~/.kube/config-thundernetes
# check that cluster is up and running
export KUBECONFIG=~/.kube/config-thundernetes
kubectl cluster-info
# open ports 10000-50000 to the internet, by modifying the Network Security Group (NSG)
# check the instructions on the next section
```

### Expose ports 10000-50000 to the Internet

Thundernetes requires VMs to have Public IPs (so game servers can be accessible) and be able to accept network traffic at port range 10000-50000 from the Internet. To allow that you need to perform the following steps *after your AKS cluster gets created*:

* Login to the Azure Portal
* Find the resource group where the AKS resources are kept, it should have a name like `MC_resourceGroupName_AKSName_location`. Alternative, you can type `az resource show --namespace Microsoft.ContainerService --resource-type managedClusters -g $AKS_RESOURCE_GROUP -n $AKS_NAME -o json | jq .properties.nodeResourceGroup` on your shell to find it.
* Find the Network Security Group object, which should have a name like `aks-agentpool-********-nsg`
* Select **Inbound Security Rules**
* Select **Add** to create a new Rule with **Any** as the protocol (you could also select between TCP or UDP, depending on your game) and **10000-50000** as the Destination Port Ranges. Pick a proper name for the rule and leave everything else at their default values

Alternatively, you can use the following command, after setting the `$RESOURCE_GROUP_WITH_AKS_RESOURCES` and `$NSG_NAME` variables with proper values:

```bash
az network nsg rule create \
  --resource-group $RESOURCE_GROUP_WITH_AKS_RESOURCES \
  --nsg-name $NSG_NAME \
  --name AKSThundernetesGameServerRule \
  --access Allow \
  --protocol "*" \
  --direction Inbound \
  --priority 1000 \
  --source-port-range "*" \
  --destination-port-range 10000-50000
```

Last but not least, don't forget to install kubectl ([instructions](https://kubernetes.io/docs/tasks/tools/#kubectl)) to manage your Kubernetes cluster.

## Installing Kubernetes locally with kind

You can use a variety of options to run Kubernetes locally, either [kind](https://kind.sigs.k8s.io/) or [k3d](https://k3d.io/) or [minikube](https://kubernetes.io/docs/getting-started-guides/minikube/). In this guide, we will use [kind](https://kind.sigs.k8s.io/).

* Kind requires Docker, so make sure you have it up and running. You can find [Docker for Windows here](https://docs.docker.com/desktop/windows/install/)
* Install kind using the instructions [here](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* Create a "kind-config.yaml" file to configure the cluster, using the contents listed below. 

Special attention is needed on the ports you will forward (the "containerPort" listed below). First of all, you need to expose port 5000 since this is the port used by the thundernetes GameServer allocation API service. You will use this port to do game server allocations.
After that, you can optionally specify ports to test your game server by sending traffic to it. Thundernetes dynamically allocates ports for your game server, ranging from 10000 to 50000. Port assignment from this range is sequential. For example, if you use two game servers with each one having a single port, the first game server port will be mapped to port 10000 and the second will be mapped to port 10001. Be aware that if you scale down your GameServerBuild and scale it up again, you probably will not get the same port. Consequently, pay special attention to the ports that you will use in your kind configuration.

Save this content to a file called `kind-config.yaml`.

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
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
```

* Run `kind create cluster --config /path/to/kind-config.yaml`
* Install kubectl ([instructions](https://kubernetes.io/docs/tasks/tools/#kubectl)) to manage your Kubernetes cluster
* Once it succeeds, run `kubectl cluster-info` to verify that the cluster is running. You should get something like this:

```bash
Kubernetes control plane is running at https://127.0.0.1:34253
CoreDNS is running at https://127.0.0.1:34253/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
```

## Install thundernetes with the installation script

Once you have a Kubernetes cluster up and running, you can run the following command to install thundernetes. This will install thundernetes *without* TLS authentication for the allocation API service, which should only be used on test environments.

```bash
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/master/installfiles/operator.yaml
```

Read the following section if you want to have TLS based authentication for the thundernetes API service.

### Install thundernetes with TLS authentication

You need to create/configure the certificate that will be used to protect the allocation API service.

For testing purposes, you can generate a self-signed certificate and use it to secure the allocation API service. You can use OpenSSL to create a self-signed certificate and key (of course, this scenario is not recommended for production).

```bash
openssl genrsa 2048 > private.pem
openssl req -x509 -days 1000 -new -key private.pem -out public.pem
```

Once you have the certificate, you need to register it as a [Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/). It *must* be in the same namespace as the controller and called `tls-secret`. We are going to install it in the default namespace `thundernetes-system`.

```bash
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=/path/to/public.pem --key=/path/to/private.pem
```

Then, you can run the following script to install thundernetes with TLS security for the allocation API service.

```bash
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/master/installfiles/operator_with_security.yaml
```

The two installation files (operator.yaml and operator_with_security.yaml) are identical except for the API_SERVICE_SECURITY environment variable that is passed into the controller container.

At this point, you are ready to run your game server on thundernetes. If you want to run one of our sample game servers, please read on. Otherwise, if you want to run your own game server, please go to [this document](developertool.md).

## Run sample game servers

Thundernetes comes with two sample game server projects that are integrated with [GSDK](https://github.com/PlayFab/gsdk). You can use either one of them to validate your thundernetes installation.

### .NET Core Fake game server

This sample, located [here](../samples/netcore), is a simple .NET Core Web API app that implements GSDK. You can install it on your Kubernetes cluster by runnning the following command:

```bash
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/master/samples/netcore/sample.yaml
```

> To read about the fields that you need to specify for a GameServerBuild, you can check [this document](gameserverbuild.md).

Try using `kubectl get gs` to see the running game servers:

```bash
dgkanatsios@desktopdigkanat:thundernetes$ kubectl get gs
NAME                                   HEALTH    STATE        PUBLICIP      PORTS      SESSIONID
gameserverbuild-sample-netcore-ayxzh   Healthy   StandingBy   52.183.89.4   80:10001
gameserverbuild-sample-netcore-mveex   Healthy   StandingBy   52.183.89.4   80:10000
```

and `kubectl get gsb` to see the status of the GameServerBuild:

```bash
dgkanatsios@desktopdigkanat:thundernetes$ kubectl get gsb
NAME                             STANDBY   ACTIVE   CRASHES   HEALTH
gameserverbuild-sample-netcore   2/2       0        0         Healthy
```

> `gs` and `gsb` are just short names for GameServer and GameServerBuild, respectively. You could just type `kubectl get gameserver` or `kubectl get gameserverbuild` instead.

To scale your GameServerBuild, you can do `kubectl edit gsb gameserverbuild-sample-netcore` and edit the max/standingBy numbers.

#### Allocate a game server

Allocating a GameServer will transition its state from "StandingBy" to "Active" and will unblock the "ReadyForPlayers" GSDK call.

If you are running on Azure Kubernetes Service, you can use the following command to allocate a game server:

```bash
# grab the IP of the external load balancer that is used to route traffic to the allocation API service
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
# do the allocation call. Make sure the buildID is the same as the one that you created your Build with
curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"ac1b7082-d811-47a7-89ae-fe1a9c48a6da"}' http://${IP}:5000/api/v1/allocate
```

As you can see, the arguments to the allocation call are two:

* buildID: this must be the same as the buildID configured in the GameServerBuild
* sessionID: a GUID that you can use to identify the game server session. Must be unique for each game server you allocate. If you try to allocate using a sessionID that is in use, the call will return the details of the existing game server. This call is equivalent to calling [RequestMultiplayerServer](https://docs.microsoft.com/rest/api/playfab/multiplayer/multiplayer-server/request-multiplayer-server) in PlayFab Multiplayer Servers.

Result of the allocate call is the IP/Port of the server in JSON format.

```bash
{"IPV4Address":"52.183.89.4","Ports":"80:10000","SessionID":"ac1b7082-d811-47a7-89ae-fe1a9c48a6da"}
```

You can now use the IP/Port to connect to the allocated game server. The fake game server exposes a /hello endpoint that returns the hostname of the container.

```bash
dgkanatsios@desktopdigkanat:thundernetes$ curl 52.183.89.4:10000/Hello
Hello from fake GameServer with hostname gameserverbuild-sample-netcore-mveex
```

At the same time, you can check your game servers again. Since the original request was for 2 standingBy and 4 maximum servers, we can now see that we have 2 standingBy and 1 active.

```bash
dgkanatsios@desktopdigkanat:thundernetes$ kubectl get gs
NAME                                   HEALTH    STATE        PUBLICIP      PORTS      SESSIONID
gameserverbuild-sample-netcore-ayxzh   Healthy   StandingBy   52.183.89.4   80:10001
gameserverbuild-sample-netcore-mveex   Healthy   Active       52.183.89.4   80:10000   ac1b7082-d811-47a7-89ae-fe1a9c48a6da
gameserverbuild-sample-netcore-pxrqx   Healthy   StandingBy   52.183.89.4   80:10002
```

#### Lifecycle of a game server

The game server will remain in Active state as long as the game server process is running. Once the game server process exits, the game server pod will be deleted and a new one will be created in its place. For more information on the GameServer lifecycle, please check [here](gameserverlifecycle.md).

### Openarena

This sample, located [here](../samples/openarena), is based on the popular open source FPS game [OpenArena](http://www.openarena.ws/smfnews.php). You can install it using this script

```bash
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/master/samples/openarena/sample.yaml
```

You can allocate a game server by using the same command as the fake game server.  To connect to an active server, you need to download the OpenArena client from [here](http://openarena.ws/download.php?view.4).

## Modifying standingBy and max number of servers

You can use `kubectl edit gsb <name-of-your-gameserverbuild>` to modify the max/standingBy numbers. Bear in mind that the count of active+standingBy will never be larger than the max.

## Uninstalling thundernetes

You should first remove all your GameServerBuilds. Since each GameServer has a finalizer, removing the controller before removing GameServer instances will make the GameServer instances get stuck if you try to delete them.

```bash
kubectl delete gsb --all -A # this will delete all GameServerBuilds from all namespaces, which in turn will delete all GameServers
kubectl get gs -A # verify that there are no GameServers in all namespaces
kubectl delete ns thundernetes-system # delete the namespace with all thundernetes resources
# delete RBAC resources. You might need to add namespaces for the service account and the role binding
kubectl delete clusterrole thundernetes-proxy-role thundernetes-metrics-reader thundernetes-manager-role thundernetes-gameserver-editor-role
kubectl delete serviceaccount thundernetes-gameserver-editor
kubectl delete clusterrolebinding thundernetes-manager-rolebinding thundernetes-proxy-rolebinding
kubectl delete rolebinding thundernetes-gameserver-editor-rolebinding
```
---
layout: default
title: Installing Thundernetes
parent: Quickstart
nav_order: 3
---

# Installing Thundernetes

Follow the steps below to install Thundernetes on your Kubernetes cluster.

## Install cert-manager

Once you have a Kubernetes cluster up and running, you need to install [cert-manager](https://cert-manager.io). Cert-manager is a certificate controller for Kubernetes and it is needed for the webhooks used to validate your GameServerBuilds.

{% include code-block-start.md %}
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
{% include code-block-end.md %}

To verify that cert-manager is installed, you can run the following command:

{% include code-block-start.md %}
kubectl get pods -n cert-manager
{% include code-block-end.md %}

## Install Thundernetes with the installation script

You can run the following command to install Thundernetes. 

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/installfiles/operator.yaml
{% include code-block-end.md %}

**Note:** installing Thundernetes will automatically deploy two DaemonSets: one for Linux nodes and for Windows nodes. If you only plan to use one OS for the nodes, you can safely delete the DaemonSet for the other. These DaemonSets live under the `thundernetes-system` namespace, you can optionally delete them with the following commands (even though there is no harm in keeping them around):

- Windows: `kubectl delete -n thundernetes-system daemonset thundernetes-nodeagent-win` (if you plan to only use Linux game servers)
- Linux: `kubectl delete -n thundernetes-system daemonset thundernetes-nodeagent` (if you plan to only use Windows game servers)

To verify that Thundernetes is up and running, you can run the following command:

{% include code-block-start.md %}
kubectl get pods -n thundernetes-system
{% include code-block-end.md %}

You should see something like that, for a 3-node cluster:

{% include code-block-start.md %}
NAME                                               READY   STATUS    RESTARTS   AGE
thundernetes-controller-manager-5fc55b9db7-rcns9   1/1     Running   0          10s
thundernetes-nodeagent-6wljt                       1/1     Running   0          15s
thundernetes-nodeagent-6x8c4                       1/1     Running   0          20s
thundernetes-nodeagent-eabgh                       1/1     Running   0          17s
{% include code-block-end.md %}

At this point, you are ready to run a test game server on Thundernetes to verify that the system is working as expected. If you want to run one of our sample game servers, check our [samples](samples.md). Otherwise, if you want to run your own game server, go to [this document](../gsdk/README.md).

The aforementioned scripts install Thundernetes with unauthenticated access to the allocation API service. This is fine for development scenarios, but for production environments you would need to secure the service. There are a couple of options you can use. Thundernetes offers a way to configure mTLS authentication to the allocation API service, you can read the next section. Alternatively, you can use a [Kubernetes Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) service, like [nginx-ingress](https://github.com/kubernetes/ingress-nginx). To lean how to secure your service, read our ["Protect your Services using an Ingress"](../howtos/serviceingress.md) document.

### Installing Thundernetes with mTLS authentication

You need to create/configure the certificate that will be used to protect the allocation API service. A properly configured certificate (signed by a well-known CA) is recommended for production environments.

For testing purposes, you can generate a self-signed certificate and use it to secure the allocation API service. You can use OpenSSL to create a self-signed certificate and key (of course, this scenario is not recommended for production).

{% include code-block-start.md %}
openssl genrsa 2048 > private.pem
openssl req -x509 -days 1000 -new -key private.pem -out public.pem
{% include code-block-end.md %}

Once you have the certificate, you need to register it as a [Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/). It *must* be in the same namespace as the controller and called `tls-secret`. We are going to install it in the default namespace `thundernetes-system`.

{% include code-block-start.md %}
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=/path/to/public.pem --key=/path/to/private.pem
{% include code-block-end.md %}

Then, you can run the following script to install Thundernetes with TLS security for the allocation API service.

{% include code-block-start.md %}
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/installfiles/operator_with_security.yaml
{% include code-block-end.md %}

**Note:** The two installation files (operator.yaml and operator_with_security.yaml) are identical except for the API_SERVICE_SECURITY environment variable that is passed into the controller container.
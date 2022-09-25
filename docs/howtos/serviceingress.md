---
layout: default
title: Protect your Services using an Ingress
parent: How to's
nav_order: 8
---

# Protect your Services using an Ingress

It is possible to secure any service in the cluster using a Kubernetes [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/), this way it can terminate any requests that don't have the right credentials. Kubernetes doesn't have an Ingress implementation out of the box, so you have to choose an [Ingress Controller](https://kubernetes.io/docs/concepts/services-networking/ingress-controllers/) to install. An Ingress can provide multiple ways to authenticate requests, depending on the implementation, for example, the [Nginx Ingress Controller](https://kubernetes.github.io/ingress-nginx/deploy/) allows you to use: basic authentication, mutual TLS (mTLS) or use external authentication services. In the following section we show how to use an Ingress to enable mTLS for the Game Server API.

## Enabling mTLS for the Game Server API

When you deploy an Ingress you can use annotations to enable the authentication of both the server and the client, all of this will happen at the Ingress level, so you don't have to change any code in the service. Note that this needs you to have a domain you can use for your service. For this, you have to create a Kubernetes Secret containing the server's private and public key, and the public key from the Certificate Authority (CA). For testing purposes, or for private use, you can create your own CA and use it to sign all your certificates. To do all of this you can follow the next steps:

### Step 1: Install Thundernetes and the Nginx Ingress Controller on your cluster

{% include code-block-start.md %}
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml
kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/installfiles/operator.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.2.0/deploy/static/provider/cloud/deploy.yaml
{% include code-block-end.md %}

### Step 2: Create a key pair to act as your Certificate Authority (CA)

{% include code-block-start.md %}
openssl req -x509 -sha256 -newkey rsa:4096 -keyout ca.key -out ca.crt -days 1000 -nodes -subj '/CN=My Cert Authority'
{% include code-block-end.md %}

### Step 3: Create key pairs for the server and for the client and sign them with the CA

{% include code-block-start.md %}
# create and sign the server keys
openssl req -new -newkey rsa:4096 -keyout server.key -out server.csr -nodes -subj '/CN={the host of your server}' -addext "subjectAltName=DNS:{the host of your server}"
openssl x509 -req -sha256 -days 1000 -in server.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out server.crt
# create and sign the client keys
openssl req -new -newkey rsa:4096 -keyout client.key -out client.csr -nodes -subj '/CN=Client'
openssl x509 -req -sha256 -days 1000 -in client.csr -CA ca.crt -CAkey ca.key -set_serial 02 -out client.crt
{% include code-block-end.md %}

### Step 4: Create a Kubernetes Secret

{% include code-block-start.md %}
kubectl create secret generic -n thundernetes-system tls-certs --from-file=tls.crt=server.crt --from-file=tls.key=server.key --from-file=ca.crt=ca.crt
{% include code-block-end.md %}

### Step 5: Deploy the Service and the Ingress

We have bundled the definitions to deploy the GameServer API in the [deployment/secured/deploy_mtls.yaml](https://github.com/PlayFab/thundernetes/blob/main/cmd/gameserverapi/deployment/secured/deploy_mtls.yaml) file, it includes the Deployment, the Service, and the Ingress needed for the mTLS to work. You have to check the name of the Secret referenced in the Ingress matches the one you created, replace the `${IMAGE_TAG}` for the current release, and also replace the `${HOST}` values for your domain. Then you just run:

{% include code-block-start.md %}
kubectl apply -f {path to deploy_mtls.yaml}
{% include code-block-end.md %}

### Step 6: Connect to the Game Server API

Now the Game Server API is exposed through the Ingress, to connect to it you have to get the Ingress' external IP, you can do this with this command:

{% include code-block-start.md %}
kubectl get ingress thundernetes-gameserverapi-ingress -n thundernetes-system
{% include code-block-end.md %}

The Ingress may take a minute before getting an IP, if you're running this locally it won't ever get one, but you can use [port forwarding](https://kubernetes.io/docs/tasks/access-application-cluster/port-forward-access-application-cluster/) instead. Finally you can try a simple GET request providing the client keys to test that the API is working, note that you will have to add the public certificate of your CA to your trusted root certificates:

{% include code-block-start.md %}
curl https://{ingress_IP}/api/v1/gameserverbuilds --cert client.crt --key client.key
{% include code-block-end.md %}

Or, you can also skip the validation on the client side and only check that the server is verifying the client certificates, adding the `-k` or `--insecure` option:

{% include code-block-start.md %}
curl https://{ingress_IP}/api/v1/gameserverbuilds --cert client.crt --key client.key -k
{% include code-block-end.md %}

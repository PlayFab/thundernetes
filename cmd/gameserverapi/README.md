# GameServer API service (work in progress)

GameServer API service is a RESTful API service that facilitates access to GameServerBuild/GameServer/GameServerDetail Custom Resources on your Kubernetes clusters.

## Deploying the GameServer API with mutual TLS
It is possible to secure the GameServer API with mutual TLS using a Kubernetes Ingress, this way the Ingress will terminate any requests without the correct certificate. For this you have to create a Kubernetes Secret containing the server's private and public key, and the public key from the Certificate Authority (CA). For testing purposes, or for private use, you can be your own CA and self sign all your certificates. To do all of this you can follow the next steps:

### Step 1: Install Thundernetes and the Nginx Ingress Controller on your cluster
```
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml

kubectl apply -f https://raw.githubusercontent.com/PlayFab/thundernetes/main/installfiles/operator.yaml

kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.2.0/deploy/static/provider/cloud/deploy.yaml
```
### Step 2: Create a key pair to act as your Certificate Authority (CA)
```
openssl req -x509 -sha256 -newkey rsa:4096 -keyout ca.key -out ca.crt -days 1000 -nodes -subj '/CN=My Cert Authority'
```

### Step 3: Create key pairs for the server and for the client and sign them with the CA
```
# create and sign the server keys
openssl req -new -newkey rsa:4096 -keyout server.key -out server.csr -nodes -subj '/CN=Server'

openssl x509 -req -sha256 -days 1000 -in server.csr -CA ca.crt -CAkey ca.key -set_serial 01 -out server.crt

# create and sign the client keys
openssl req -new -newkey rsa:4096 -keyout client.key -out client.csr -nodes -subj '/CN=Client'

openssl x509 -req -sha256 -days 1000 -in client.csr -CA ca.crt -CAkey ca.key -set_serial 02 -out client.crt
```

### Step 4: Create a Kubernetes Secret
```
kubectl create secret generic -n thundernetes-system tls-certs --from-file=tls.crt=server.crt --from-file=tls.key=server.key --from-file=ca.crt=ca.crt
```

### Step 5: Deploy the Game Server API and an Ingress with your keys
We have bundled the definitions for the GameServer Deployment, Service, and Ingress in the ```deployment/secured/deploy_mtls.yaml``` file. You have to change the image inside the deployment and check the name of the Secret referenced in the Ingress matches the one you created. Then you just run:
```
kubectl apply -f [path to deploy_mtls.yaml]
```

### Connect to the Game Server API
Now the Game Server API is exposed through the Ingress, to connect to it you have to get the Ingress' external IP, you can do this with this command:
```
kubectl get ingress thundernetes-gameserverapi-ingress -n thundernetes-system
```
The Ingress may take a minute before getting an IP, if you're running this locally it won't ever get one, but you can use port forwarding instead. Finally try a simple GET request providing the client keys to test that the API is working:
```
curl https://{ingress_IP}/api/v1/gameserverbuilds --cert client.crt --key client.key -k
```

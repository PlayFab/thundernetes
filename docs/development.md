# Development

## Release new thundernetes version

This will require 2 PRs.

- Make sure you update `.version` file on the root of this repository with the new version
- Run `make clean` to ensure any cached artifacts of old builds are deleted.
- Push and merge
- Run the GitHub Actions workflow [here](https://github.com/PlayFab/thundernetes/actions/workflows/publish.yml) to create the new images
- Run `make create-install-files` to generate the operator install files
- Replace the image on the [netcore-sample YAML files](../samples/netcore)
- Push and merge

## Metrics

- If you are using Prometheus and Prometheus operator, uncomment all sections with `# [PROMETHEUS]` on `config/default/kustomization.yaml` file. More details [here](https://book.kubebuilder.io/reference/metrics.html)
- To enable authentication for the metrics server, remove the comment from this line on the file ``config/default/kustomization.yaml`: `- manager_auth_proxy_patch.yaml`

## Running end to end tests on macOS

First of all, end to end tests require `envsubst` utility, assuming that you have Homebrew installed you can get it via `brew install gettext &&brew link --force gettext`.
We assume that you have installed Go, then you should install kind with `go install sigs.k8s.io/kind@latest`. Kind will be installed in `$(go env GOPATH)/bin` directory. Then, you should move kind to the `<projectRoot>/operator/testbin/bin/` folder with a command like `cp $(go env GOPATH)/bin/kind ./operator/testbin/bin/kind`. You can run end to end tests with `make builddockerlocal createkindcluster e2elocal`.

## Various scripts

### Generate cert for testing

```
openssl genrsa 2048 > private.pem
openssl req -x509 -days 1000 -new -key private.pem -out public.pem
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=/home/dgkanatsios/public.pem --key=/home/dgkanatsios/private.pem
```

### Allocate a game server

#### With TLS auth

```bash
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' http://${IP}:5000/api/v1/allocate
```

#### Without TLS auth

```bash
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' http://${IP}:5000/api/v1/allocate
```

### Do 50 allocations

#### Without TLS auth

```bash
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
for i in {1..50}; do SESSION_ID=$(uuidgen); curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"'${SESSION_ID}'"}' http://${IP}:5000/api/v1/allocate; done
```

#### With TLS auth

```bash
IP=$(kubectl get svc -n thundernetes-system thundernetes-controller-manager -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
for i in {1..50}; do SESSION_ID=$(uuidgen); curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f6","sessionID":"'${SESSION_ID}'"}' https://${IP}:5000/api/v1/allocate; done
```

## Run end to end tests locally

```bash
make clean deletekindcluster builddockerlocal createkindcluster e2elocal
```

## Run controller locally

```bash
cd operator
THUNDERNETES_INIT_CONTAINER_IMAGE=ghcr.io/playfab/thundernetes-initcontainer:0.2.0 go run main.go
```

## [ADVANCED] Install thundernetes via cloning this repository

You should `git clone` this repository to your local machine. As soon as this is done, you can run the following command to install Thundernetes.

```bash
export TAG=0.0.2.0
IMG=ghcr.io/playfab/thundernetes-operator:${TAG} \
  IMAGE_NAME_INIT_CONTAINER=ghcr.io/playfab/thundernetes-initcontainer \
  IMAGE_NAME_SIDECAR=ghcr.io/playfab/thundernetes-sidecar-netcore \
  API_SERVICE_SECURITY=none \
   make -C pkg/operator install deploy
```

Note that this will install thundernetes without any security for the allocation API service. If you want to enable security for the allocation API service, you can should provide a certificate and key for the allocation API service.

You can use OpenSSL to create a self-signed certificate and key (not recommended for production).

```bash
openssl genrsa 2048 > private.pem
openssl req -x509 -days 1000 -new -key private.pem -out public.pem
```

Install the cert and key as a Kubernetes Secret in the same namespace as the operator.

```bash
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=/home/dgkanatsios/public.pem --key=/home/dgkanatsios/private.pem
```

Then, you need to install the operator enabling TLS authentication for the allocation API service.

```bash
export TAG=0.0.1.2
IMG=ghcr.io/playfab/thundernetes-operator:${TAG} \
  IMAGE_NAME_INIT_CONTAINER=docker.io/dgkanatsios/thundernetes-initcontainer \
  IMAGE_NAME_SIDECAR=docker.io/dgkanatsios/thundernetes-sidecar-netcore \
  API_SERVICE_SECURITY=usetls \
   make -C pkg/operator install deploy
```

As soon as this is done, you can run `kubectl -n thundernetes-system get pods` to verify that the operator pod is running. To run a demo gameserver, you can use the command:

```bash
kubectl apply -f pkg/operator/config/samples/netcore.yaml
```

This will create a GameServerBuild with 2 standingBy and 4 maximum gameservers.
After a while, you will see your game servers.

```bash
kubectl get gameservers # or kubectl get gs
```

```bash
NAME                           HEALTH    STATE        PUBLICIP        PORTS      SESSIONID
gameserverbuild-sample-apbjz   Healthy   StandingBy   52.183.89.4     80:24558
gameserverbuild-sample-gqhrm   Healthy   StandingBy   52.183.88.255   80:10319
```

and your GameServerBuild:

```bash
kubectl get gameserverbuild # or kubectl get gsb
```

```bash
NAME                     ACTIVE   STANDBY   CRASHES   HEALTH
gameserverbuild-sample   0        2         0         Healthy
```

You can edit the number of standingBy and max by changing the values in the GameServerBuild.

```bash
kubectl edit gsb gameserverbuild-sample
```

You can also use the allocation API. To get the Public IP, you can use the command:

```bash
kubectl get svc -n thundernetes-system thundernetes-controller-manager
```

```bash
NAME                              TYPE           CLUSTER-IP    EXTERNAL-IP    PORT(S)          AGE
thundernetes-controller-manager   LoadBalancer   10.0.62.144   20.83.72.255   5000:32371/TCP   39m
```

The External-Ip field is the Public IP of the LoadBalancer that we can use to call the allocation API.

If you have configured your allocation API service with no security:

```bash
IP=...
curl -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' http://${IP}:5000/api/v1/allocate
```

If you're using TLS authentication:

```bash
IP=...
curl --key ~/private.pem --cert ~/public.pem --insecure -H 'Content-Type: application/json' -d '{"buildID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5","sessionID":"85ffe8da-c82f-4035-86c5-9d2b5f42d6f5"}' https://${IP}:5000/api/v1/allocate
```

Then, you can see that the game server has successfully been allocated. 

```bash
kubectl get gameservers
```

```bash
NAME                           HEALTH    STATE        PUBLICIP        PORTS      SESSIONID
gameserverbuild-sample-apbjz   Healthy   StandingBy   52.183.89.4     80:24558
gameserverbuild-sample-bmich   Healthy   Active       20.94.219.110   80:38208   85ffe8da-c82f-4035-86c5-9d2b5f42d6f5
gameserverbuild-sample-gqhrm   Healthy   StandingBy   52.183.88.255   80:10319
```

On the allocated server, you can call the demo game server HTTP endpoint.

```bash
curl 20.94.219.110:38208/hello
```

You'll get a response like `Hello from <containerName>`.

Try calling the endpoint that will gracefully terminate the game server.

```bash
curl 20.94.219.110:38208/hello/terminate
```

We can see that the game server has been terminated. Since our GameServerBuild is configured with 2 standingBys, no other servers are created.

```bash
kubectl get gameservers
```

```bash
NAME                           HEALTH    STATE        PUBLICIP        PORTS      SESSIONID
gameserverbuild-sample-apbjz   Healthy   StandingBy   52.183.89.4     80:24558
gameserverbuild-sample-gqhrm   Healthy   StandingBy   52.183.88.255   80:10319
```

## kubebuilder notes

Project was bootstrapped using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) using the following commands:

```bash
kubebuilder init --domain playfab.com --repo github.com/playfab/thundernetes/pkg/operator
kubebuilder create api --group mps --version v1alpha1 --kind GameServer
kubebuilder create api --group mps --version v1alpha1 --plural gameserverbuilds --kind GameServerBuild 
kubebuilder create api --group mps --version v1alpha1 --plural gameserverdetails --kind GameServerDetail 
```

## env variables sample

```bash
PUBLIC_IPV4_ADDRESS=20.184.250.154
PF_REGION=WestUs
PF_VM_ID=xcloudwus4u4yz5dlozul:WestUs:6b5973a5-a3a5-431a-8378-eff819dc0c25:tvmps_efa402aacd4f682230cfd91bd3dc0ddfae68c312f2b6905577cb7d9424681930_d
PF_SHARED_CONTENT_FOLDER=/gsdkdata/GameSharedContent
PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
PF_SERVER_INSTANCE_NUMBER=2
PWD=/app
PF_BUILD_ID=88a958b9-14fb-4ad9-85ca-5cc13207232e
GSDK_CONFIG_FILE=/gsdkata/Config/gsdkConfig.json
SHLVL=1
HOME=/root
CERTIFICATE_FOLDER=/gsdkdata/GameCertificates
PF_SERVER_LOG_DIRECTORY=/gsdkdata/GameLogs/
PF_TITLE_ID=1E03
_=/usr/bin/env
```

## Docker compose

The docker-compose.yml file on the root of this repo was created to facilitate sidecar development.

## Test your changes to a cluster

To test your changes to thundernetes to a Kubernetes cluster, you can use the following steps:

- The Makefile on the root of the project contains a variable `NS` that points to the container registry that you use during development. So you'd need to either set the variable in your environment (`export NS=<your-container-registry>`) or set it before calling `make` (like `NS=<your-container-registry> make build push`).
- Login to your container registry (`docker login`)
- Run `make clean build push` to build the container images and push them to your container registry
- Run `create-install-files-dev` to create the install files for the cluster
- Checkout the `installfilesdev` folder for the generated install files. This file is included in .gitignore so it will never be committed.
- Test your changes as required.
 
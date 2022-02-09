#!/bin/bash

# custom script for e2e testing

# kudos to https://elder.dev/posts/safer-bash/
set -o errexit # script exits when a command fails == set -e
set -o nounset # script exits when tries to use undeclared variables == set -u
#set -o xtrace # trace what's executed == set -x (useful for debugging)
set -o pipefail # causes pipelines to retain / set the last non-zero status

#https://stackoverflow.com/questions/59895/getting-the-source-directory-of-a-bash-script-from-within
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"

KUBECONFIG_FILE=$1
BUILD=${2:-remote} # setting a default value for $BUILD

if [ "$BUILD" = "local" ]; then
  	./pkg/operator/testbin/bin/kind load docker-image ${IMAGE_NAME_OPERATOR}:${IMAGE_TAG} --name kind
	./pkg/operator/testbin/bin/kind load docker-image ${IMAGE_NAME_INIT_CONTAINER}:${IMAGE_TAG} --name kind
	./pkg/operator/testbin/bin/kind load docker-image ${IMAGE_NAME_NETCORE_SAMPLE}:${IMAGE_TAG} --name kind
	./pkg/operator/testbin/bin/kind load docker-image ${IMAGE_NAME_NODE_AGENT}:${IMAGE_TAG} --name kind
	./pkg/operator/testbin/bin/kind load docker-image ${IMAGE_NAME_GAMESERVER_API}:${IMAGE_TAG} --name kind
fi

# certificate generation for the TLS security on the allocation API server
echo "-----Creating temp certificates for TLS security on the operator's allocation API service-----"
export TLS_PRIVATE=/tmp/${RANDOM}.pem
export TLS_PUBLIC=/tmp/${RANDOM}.pem
openssl req -x509 -newkey rsa:4096 -nodes -keyout ${TLS_PRIVATE} -out ${TLS_PUBLIC} -days 365 -subj '/CN=localhost'
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=${TLS_PUBLIC} --key=${TLS_PRIVATE}

echo "-----Compiling, building and deploying the operator to local Kubernetes cluster-----"
IMG=${IMAGE_NAME_OPERATOR}:${IMAGE_TAG} API_SERVICE_SECURITY=usetls make -C "${DIR}"/../pkg/operator deploy

echo "-----Deploying GameServer API-----"
IMAGE_TAG=${IMAGE_TAG} envsubst < cmd/gameserverapi/deploy.yaml | kubectl apply -f -

echo "-----Waiting for Controller deployment-----"
kubectl wait --for=condition=available --timeout=300s deployment/thundernetes-controller-manager -n thundernetes-system

echo "-----Waiting for GameServer API deployment-----"
kubectl wait --for=condition=ready --timeout=300s pod -n thundernetes-system -l app=thundernetes-gameserverapi

echo "-----Running end to end tests-----"
cd cmd/e2e
# create the test namespaces
kubectl create namespace gameserverapi
kubectl create namespace mynamespace
# https://onsi.github.io/ginkgo/#recommended-continuous-integration-configuration
IMG=${IMAGE_NAME_NETCORE_SAMPLE}:${IMAGE_TAG} go run github.com/onsi/ginkgo/v2/ginkgo -r --procs=4 --compilers=4 --randomize-all --randomize-suites --fail-on-pending --keep-going --race --trace
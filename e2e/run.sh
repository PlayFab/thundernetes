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

# install cert manager for webhook certificates CRDs
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.8.0/cert-manager.yaml

echo "-----Waiting for cert-manager deployments-----"
kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager
kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-cainjector
kubectl wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-webhook

# certificate generation for the TLS security on the allocation API server
echo "-----Creating temp certificates for TLS security on the operator's allocation API service-----"
export TLS_PRIVATE=/tmp/${RANDOM}.pem
export TLS_PUBLIC=/tmp/${RANDOM}.pem
openssl req -x509 -newkey rsa:4096 -nodes -keyout ${TLS_PRIVATE} -out ${TLS_PUBLIC} -days 365 -subj '/CN=localhost'
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=${TLS_PUBLIC} --key=${TLS_PRIVATE}

# fake certificate for testing the TLS security on the allocation API server
export FAKE_TLS_PRIVATE=/tmp/${RANDOM}.pem
export FAKE_TLS_PUBLIC=/tmp/${RANDOM}.pem
openssl req -x509 -newkey rsa:4096 -nodes -keyout ${FAKE_TLS_PRIVATE} -out ${FAKE_TLS_PUBLIC} -days 365 -subj '/CN=localhost'

echo "-----Compiling, building and deploying the operator to local Kubernetes cluster-----"
IMG=${IMAGE_NAME_OPERATOR}:${IMAGE_TAG} API_SERVICE_SECURITY=usetls INT_LISTENING_PORT=${LISTENING_PORT} STR_LISTENING_PORT=\"${LISTENING_PORT}\" make -C "${DIR}"/../pkg/operator deploye2e

echo "-----Deploying GameServer API-----"
cd cmd/gameserverapi/deployment/default
"${DIR}"/../pkg/operator/bin/kustomize build ../e2e | IMAGE_TAG=${IMAGE_TAG} envsubst | kubectl apply -f -

echo "-----Waiting for Controller deployment-----"
kubectl wait --for=condition=available --timeout=300s deployment/thundernetes-controller-manager -n thundernetes-system

echo "-----Waiting for GameServer API deployment-----"
kubectl wait --for=condition=ready --timeout=300s pod -n thundernetes-system -l app=thundernetes-gameserverapi

echo "-----Running end to end tests-----"
cd "${DIR}"/../cmd/e2e
# create the test namespaces
kubectl create namespace gameserverapi
kubectl create namespace e2e

GINKGO=github.com/onsi/ginkgo/v2/ginkgo
# we are using description based filtering to first run only the test that modifies the number of Nodes in the cluster
# Reason is that if we ran it together with the other tests, it would have an impact on the number of Actives
# https://onsi.github.io/ginkgo/#description-based-filtering
IMG=${IMAGE_NAME_NETCORE_SAMPLE}:${IMAGE_TAG} go run ${GINKGO} --focus "Cluster with variable number of Nodes" --fail-on-pending --keep-going --race --trace
# check here for flags explanation: https://onsi.github.io/ginkgo/#recommended-continuous-integration-configuration
IMG=${IMAGE_NAME_NETCORE_SAMPLE}:${IMAGE_TAG} go run ${GINKGO} --skip "Cluster with variable number of Nodes" -r --procs=4 --compilers=4 --randomize-all --randomize-suites --fail-on-pending --keep-going --race --trace
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
fi

# function finish {
#   echo "-----Cleaning up-----"
#   if [ "$BUILD" = "local" ]; then
#     make -C "${DIR}"/.. cleank8slocal
#   fi
#   rm ${TLS_PRIVATE} ${TLS_PUBLIC}
# }

# trap finish EXIT

# certificate generation for the TLS security on the allocation API server
echo "-----Creating temp certificates for TLS security on the operator's allocation API service-----"
export TLS_PRIVATE=/tmp/${RANDOM}.pem
export TLS_PUBLIC=/tmp/${RANDOM}.pem
openssl req -x509 -newkey rsa:4096 -nodes -keyout ${TLS_PRIVATE} -out ${TLS_PUBLIC} -days 365 -subj '/CN=localhost'
kubectl create namespace thundernetes-system
kubectl create secret tls tls-secret -n thundernetes-system --cert=${TLS_PUBLIC} --key=${TLS_PRIVATE}

echo "-----Compiling, building and deploying to local Kubernetes cluster-----"
IMG=${IMAGE_NAME_OPERATOR}:${IMAGE_TAG} API_SERVICE_SECURITY=usetls make -C "${DIR}"/../pkg/operator deploy

echo "-----Waiting for Controller deployment-----"
kubectl wait --for=condition=available --timeout=300s deployment/thundernetes-controller-manager -n thundernetes-system

echo "-----Running Go tests-----"
cd cmd/e2e && GOPRIVATE=github.com/playfab/thundernetes go mod tidy && go run *.go ${IMAGE_NAME_NETCORE_SAMPLE}:${IMAGE_TAG}
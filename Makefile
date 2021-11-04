NS=ghcr.io/playfab

export IMAGE_NAME_OPERATOR=thundernetes-operator
export IMAGE_NAME_INIT_CONTAINER=thundernetes-initcontainer
export IMAGE_NAME_SIDECAR=thundernetes-sidecar-go
export IMAGE_NAME_NETCORE_SAMPLE=thundernetes-netcore-sample
export IMAGE_NAME_OPENARENA_SAMPLE=thundernetes-openarena-sample

export OPERATOR_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)
export INIT_CONTAINER_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)
export SIDECAR_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)
export NETCORE_SAMPLE_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)
export OPENARENA_SAMPLE_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)

# local e2e with kind
export KIND_CLUSTER_NAME=kind

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

docker-compose:
	rm -rf samples/data/GameLogs/*
	docker-compose up --build

build:
	docker build -f ./operator/Dockerfile -t $(NS)/$(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG) ./operator
	docker build -f ./initcontainer/Dockerfile -t $(NS)/$(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG) ./initcontainer
	docker build -f ./sidecar-go/Dockerfile -t $(NS)/$(IMAGE_NAME_SIDECAR):$(SIDECAR_TAG) ./sidecar-go
	docker build -f ./samples/netcore/Dockerfile -t $(NS)/$(IMAGE_NAME_NETCORE_SAMPLE):$(NETCORE_SAMPLE_TAG) ./samples/netcore
	docker build -f ./samples/openarena/Dockerfile -t $(NS)/$(IMAGE_NAME_OPENARENA_SAMPLE):$(OPENARENA_SAMPLE_TAG) ./samples/openarena

push:
	docker push $(NS)/$(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG)
	docker push $(NS)/$(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG)
	docker push $(NS)/$(IMAGE_NAME_SIDECAR):$(SIDECAR_TAG)
	docker push $(NS)/$(IMAGE_NAME_NETCORE_SAMPLE):$(NETCORE_SAMPLE_TAG)
	docker push $(NS)/$(IMAGE_NAME_OPENARENA_SAMPLE):$(OPENARENA_SAMPLE_TAG)

builddockerlocal:
	docker build -f operator/Dockerfile -t $(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG) ./operator 
	docker build -f initcontainer/Dockerfile -t $(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG) ./initcontainer	
	docker build -f sidecar-go/Dockerfile -t $(IMAGE_NAME_SIDECAR):$(SIDECAR_TAG) ./sidecar-go 
	docker build -f samples/netcore/Dockerfile -t $(IMAGE_NAME_NETCORE_SAMPLE):$(NETCORE_SAMPLE_TAG) ./samples/netcore	
	docker build -f samples/openarena/Dockerfile -t $(IMAGE_NAME_OPENARENA_SAMPLE):$(OPENARENA_SAMPLE_TAG) ./samples/openarena	

installkind:
	curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.11.1/kind-linux-amd64
	chmod +x ./kind
	mkdir -p ./operator/testbin/bin
	mv ./kind ./operator/testbin/bin/kind

createkindcluster: 
	./operator/testbin/bin/kind create cluster --config ./e2e/kind-config.yaml

deletekindcluster:
	./operator/testbin/bin/kind delete cluster 

e2elocal: 
	kubectl config use-context kind-$(KIND_CLUSTER_NAME)
	chmod +x ./e2e/run.sh
	./e2e/run.sh config-$(KIND_CLUSTER_NAME) local

createcrds:
	make -C operator install
cleancrds:
	make -C operator uninstall

cleanall:
	kubectl delete gsb --all && make -C operator undeploy

create-install-files:
	. .versions && \
	IMG=$(NS)/$(IMAGE_NAME_OPERATOR):$${OPERATOR_TAG} \
	IMAGE_NAME_INIT_CONTAINER=$(NS)/$(IMAGE_NAME_INIT_CONTAINER) \
	IMAGE_NAME_SIDECAR=$(NS)/$(IMAGE_NAME_SIDECAR) \
	SIDECAR_TAG=$${SIDECAR_TAG} \
	INIT_CONTAINER_TAG=$${INIT_CONTAINER_TAG} \
	make -C operator create-install-files

create-install-files-with-monitoring: create-install-files
	make -C operator create-install-files-with-monitoring
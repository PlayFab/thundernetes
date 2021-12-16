
NS ?= ghcr.io/playfab/

export IMAGE_NAME_OPERATOR=thundernetes-operator
export IMAGE_NAME_NODE_AGENT=thundernetes-nodeagent
export IMAGE_NAME_INIT_CONTAINER=thundernetes-initcontainer
export IMAGE_NAME_NETCORE_SAMPLE=thundernetes-netcore-sample
export IMAGE_NAME_OPENARENA_SAMPLE=thundernetes-openarena-sample

export IMAGE_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)

# local e2e with kind
export KIND_CLUSTER_NAME=kind

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

GIT_REVISION := $(shell git rev-parse --short HEAD)
UPTODATE := .uptodate
# Automated DockerFile building
# By convention, every directory with a Dockerfile in it will build an image called ghcr.io/playfab/<directory name>
# An .uptodate file will be created in the directory to indicate that the Dockerfile has been built.
%/$(UPTODATE): %/Dockerfile
	@echo
	$(SUDO) docker build --build-arg=revision=$(GIT_REVISION) -t $(NS)$(shell basename $(@D)) -t $(NS)$(shell basename $(@D)):$(IMAGE_TAG) -f $(@D)/Dockerfile .
	@echo
	touch $@

# We don't want find to scan inside a bunch of directories, to speed up Dockerfile dectection.
DONT_FIND := -name vendor -prune -o -name .git -prune -o -name .cache -prune -o -name .pkg -prune -o -name packaging -prune -o

# Get a list of directories containing Dockerfiles
DOCKERFILES := $(shell find . $(DONT_FIND) -type f -name 'Dockerfile' -print)
UPTODATE_FILES := $(patsubst %/Dockerfile,%/$(UPTODATE),$(DOCKERFILES))
DOCKER_IMAGE_DIRS := $(patsubst %/Dockerfile,%,$(DOCKERFILES))
IMAGE_NAMES := $(foreach dir,$(DOCKER_IMAGE_DIRS),$(patsubst %,$(IMAGE_PREFIX)%,$(shell basename $(dir))))
images:
	$(info $(IMAGE_NAMES))
	@echo > /dev/null

buildimage:
	docker build -t thundernetes-src:$(GIT_REVISION) -f build-env/Dockerfile .

build: buildimage $(UPTODATE_FILES)
	
push:
	docker push $(NS)/$(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG)
	docker push $(NS)/$(IMAGE_NAME_NODE_AGENT):$(NODE_AGENT_TAG)
	docker push $(NS)/$(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG)
	docker push $(NS)/$(IMAGE_NAME_NETCORE_SAMPLE):$(NETCORE_SAMPLE_TAG)
	docker push $(NS)/$(IMAGE_NAME_OPENARENA_SAMPLE):$(OPENARENA_SAMPLE_TAG)

builddockerlocal:
	docker build -f operator/Dockerfile -t $(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG) ./operator
	docker build -f nodeagent/Dockerfile -t $(IMAGE_NAME_NODE_AGENT):$(NODE_AGENT_TAG) ./nodeagent
	docker build -f initcontainer/Dockerfile -t $(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG) ./initcontainer	
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
	IMAGE_NAME_NODE_AGENT=$(NS)/$(IMAGE_NAME_NODE_AGENT) \
	INIT_CONTAINER_TAG=$${INIT_CONTAINER_TAG} \
	NODE_AGENT_TAG=$${NODE_AGENT_TAG} \
	make -C operator create-install-files

create-install-files-dev:
	mkdir -p ./installfilesdev && \
	INSTALL_FILES_FOLDER=installfilesdev \
	IMG=$(NS)/$(IMAGE_NAME_OPERATOR):$${OPERATOR_TAG} \
	IMAGE_NAME_INIT_CONTAINER=$(NS)/$(IMAGE_NAME_INIT_CONTAINER) \
	IMAGE_NAME_NODE_AGENT=$(NS)/$(IMAGE_NAME_NODE_AGENT) \
	INIT_CONTAINER_TAG=$${INIT_CONTAINER_TAG} \
	NODE_AGENT_TAG=$${NODE_AGENT_TAG} \
	make -C operator create-install-files
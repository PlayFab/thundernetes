
NS ?= ghcr.io/playfab/

export IMAGE_NAME_OPERATOR=thundernetes-operator
export IMAGE_NAME_NODE_AGENT=thundernetes-nodeagent
export IMAGE_NAME_INIT_CONTAINER=thundernetes-initcontainer
export IMAGE_NAME_NETCORE_SAMPLE=thundernetes-netcore
export IMAGE_NAME_OPENARENA_SAMPLE=thundernetes-openarena

export IMAGE_TAG?=$(shell git rev-list HEAD --max-count=1 --abbrev-commit)

# local e2e with kind
export KIND_CLUSTER_NAME=kind

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Support gsed on OSX (installed via brew), falling back to sed. On Linux
# systems gsed won't be installed, so will use sed as expected.
SED ?= $(shell which gsed 2>/dev/null || which sed)

GIT_REVISION := $(shell git rev-parse --short HEAD)
UPTODATE := .uptodate
# Automated DockerFile building
# By convention, every directory with a Dockerfile in it will build an image called ghcr.io/playfab/<directory name>
# An .uptodate file will be created in the directory to indicate that the Dockerfile has been built.
%/$(UPTODATE): %/Dockerfile
	@echo
	$(SUDO) docker build --build-arg=revision=$(GIT_REVISION) -t $(NS)thundernetes-$(shell basename $(@D)) -t $(NS)thundernetes-$(shell basename $(@D)):$(IMAGE_TAG) -f $(@D)/Dockerfile .
	@echo
	touch $@

# We don't want find to scan inside a bunch of directories, to speed up Dockerfile dectection.
DONT_FIND := -name vendor -prune -o -name .git -prune -o -name .cache -prune -o -name .pkg -prune -o -name packaging -prune -o -name build-env -prune -o

# Get a list of directories containing Dockerfiles
DOCKERFILES := $(shell find . $(DONT_FIND) -type f -name 'Dockerfile' -print)
UPTODATE_FILES := $(patsubst %/Dockerfile,%/$(UPTODATE),$(DOCKERFILES))
DOCKER_IMAGE_DIRS := $(patsubst %/Dockerfile,%,$(DOCKERFILES))
IMAGE_NAMES := $(foreach dir,$(DOCKER_IMAGE_DIRS),$(patsubst %,$(NS)thundernetes-%,$(shell basename $(dir))))
images:
	$(info $(IMAGE_NAMES))
	@echo > /dev/null

buildimage: #creates a docker image as a build environment for thundernetes
	docker build -t thundernetes-src:$(GIT_REVISION) -f build-env/Dockerfile .

build: buildimage $(UPTODATE_FILES)
	
push:
	docker push $(NS)/$(IMAGE_NAME_OPERATOR):$(OPERATOR_TAG)
	docker push $(NS)/$(IMAGE_NAME_NODE_AGENT):$(NODE_AGENT_TAG)
	docker push $(NS)/$(IMAGE_NAME_INIT_CONTAINER):$(INIT_CONTAINER_TAG)
	docker push $(NS)/$(IMAGE_NAME_NETCORE_SAMPLE):$(NETCORE_SAMPLE_TAG)
	docker push $(NS)/$(IMAGE_NAME_OPENARENA_SAMPLE):$(OPENARENA_SAMPLE_TAG)

builddockerlocal: build 
	for image in $(IMAGE_NAMES); do \
		localname=`echo $$image| $(SED) -e 's:$(NS)::g'`; \
		docker tag $$image:$(IMAGE_TAG) $$localname:$(IMAGE_TAG); \
	done

installkind:
	curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.11.1/kind-linux-amd64
	chmod +x ./kind
	mkdir -p ./pkg/operator/testbin/bin
	mv ./kind ./pkg/operator/testbin/bin/kind

createkindcluster: 
	./pkg/operator/testbin/bin/kind create cluster --config ./e2e/kind-config.yaml

deletekindcluster:
	./pkg/operator/testbin/bin/kind delete cluster 

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

clean:
	$(SUDO) docker rmi $(IMAGE_NAMES) >/dev/null 2>&1 || true
	rm -rf -- $(UPTODATE_FILES) $(EXES) .cache dist
	go clean ./...
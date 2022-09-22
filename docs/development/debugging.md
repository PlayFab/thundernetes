---
layout: default
title: Debugging
parent: Development
nav_order: 1
---

# Debugging

To test your local code you have 2 options: you can run the code in a local kind cluster, or you can build local container images, upload them to a container registry, and then deploy them to a cluster.

## Run end to end tests locally

First of all, you need to install `kustomize`. You can do it by running `make -C pkg/operator kustomize`.

This command will run the e2e tests locally, and it won't delete the cluster after it's done, so you can either deploy more GameServerBuilds or check the ones used for the tests under the `e2e` namespace.

{% include code-block-start.md %}
make clean deletekindcluster builddockerlocal createkindcluster e2elocal
{% include code-block-end.md %}

## Run the controller unit tests locally

To tun the controller unit tests locally, you should go to the `pkg/operator` directory and run the following command:

{% include code-block-start.md %}
make test
{% include code-block-end.md %}

Make sure to not run them while kind cluster is up, since there will be port collisions and the tests will fail.

### Running end to end tests on macOS

First of all, end to end tests require `envsubst` utility, assuming that you have Homebrew installed you can get it via `brew install gettext && brew link --force gettext`.
We assume that you have installed Go, then you should install kind with `go install sigs.k8s.io/kind@latest`. Kind will be installed in `$(go env GOPATH)/bin` directory. Then, you should move kind to the `<projectRoot>/operator/testbin/bin/` folder with a command like `cp $(go env GOPATH)/bin/kind ./operator/testbin/bin/kind`. You can run end to end tests with `make clean builddockerlocal createkindcluster e2elocal`.

## Test your changes on a cluster

To test your changes to Thundernetes on a Kubernetes cluster, you can use the following steps:

- The Makefile on the root of the project contains a variable `NS` that points to the container registry that you use during development. So you'd need to either set the variable in your environment (`export NS=<your-container-registry>`) or set it before calling `make` (like `NS=<your-container-registry> make build push`).
- Login to your container registry (`docker login`)
- Run `make clean build push` to build the container images and push them to your container registry
- Run `create-install-files-dev` to create the install files for the cluster
- Checkout the `installfilesdev` folder for the generated install files. This file is included in .gitignore so it will never be committed.
- Test your changes as required. For example, to install Thundernetes controller, you can do `kubectl apply -f installfilesdev/operator_with_monitoring.yaml` and then you can install any of the samples on the `samples` folder.
- single command: `NS=docker.io/<repo>/ make clean build push create-install-files-dev`

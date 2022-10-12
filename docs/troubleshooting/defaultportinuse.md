---
layout: default
title: Controller port 5000
parent: Troubleshooting
nav_order: 4
---

# How can I change the port that Thundernetes uses for the Allocation API service?

By default, Thundernetes's Allocation API service listens on port 5000. Locally, it opens with the kind config set-up [here](../quickstart/installing-kind.md). This port can already be in use by another service thus causing Thundernetes to fail.

## Kind Changes

To use an alternate port, the first step is changing the `kind-config.yaml` to use the desired port. For example:

{% include code-block-start.md %}
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
  extraPortMappings:
  - containerPort: 5000
    hostPort: 5000
    listenAddress: "0.0.0.0"
    protocol: tcp
  - containerPort: 10000
    hostPort: 10000
    listenAddress: "0.0.0.0"
    protocol: tcp
  - containerPort: 10001
    hostPort: 10001
    listenAddress: "0.0.0.0"
    protocol: tcp
{% include code-block-end.md %}

## YAML Changes

The necessary YAML changes are found within the `manager.yaml` file. A find and replace of `5000` with `{DESIRED_PORT}` will change where the Allocation API listens. 

Once this file is modified, you can generate new installfiles with `make create-install-files` and verify your changes in `operator.yaml`

### Development - End to end tests

End to end tests also run and listen on port 5000. Once you complete the above yaml change, you also need to modify `e2e/kind-config.yaml` to listen on your desired port. The other needed change is modifying allocationApiSvcPort in `pkg/operator/controllers/suite_test.go`

## Verify changes

Once these changes are made and Thundernetes is running, you can verify the port within the logs using the following:
`kubectl -n thundernetes-system logs {thundernetes-controller-manager} | grep addr`

Resulting in the following output:

`2022-10-07T17:01:07Z    INFO    allocation-api  serving allocation API service  {"addr": ":5005", "port": 5005}`

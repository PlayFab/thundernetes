---
layout: default
title: game server logging
parent: How to's
nav_order: 4
---

# Get GameServer logs

Thundernetes does not do anything special to obtain the logs for your GameServer Pods, since there already are a lot of solutions in the Kubernetes ecosystem for this purpose. One of easiest ways to do this is to use [fluentbit](https://fluentbit.io/) to capture logs and send them to [Azure Blob Storage](https://docs.microsoft.com/azure/storage/blobs/storage-blobs-overview) or on a Storage provide of your choice (you can see output providers for fluentbit [here](https://docs.fluentbit.io/manual/pipeline/outputs)).

You can use the following steps to setup fluentbit to capture logs from your GameServer Pods and send them to Azure Storage:

- Set up an Azure Storage Account. Check [here](https://docs.microsoft.com/azure/storage/common/storage-account-create?tabs=azure-portal) on how to do it using the Azure Portal.
- Install fluentbit on your Kubernetes cluster. Check [here](https://docs.fluentbit.io/manual/installation/kubernetes) on how to do it using the Azure Portal.
- As soon as you create the namespace and roles/role bindings, you should create the fluentbit ConfigMap containing the fluentbit configuration file. You can see a sample [here](../samples/fluentbit/fluent-bit-configmap.yaml). Remember to replace the values with your Azure Storage Account name and key.
- Finally, you should create the fluentbit DaemonSet, so a fluentbit Pod runs on every Node in your cluster and grabs the logs. You can find a sample [here](../samples/fluentbit/fluent-bit-ds.yaml).
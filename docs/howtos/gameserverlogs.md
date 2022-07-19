---
layout: default
title: Game server logging
parent: How to's
nav_order: 4
---

# Game server logging

Thundernetes does not do anything special to obtain the logs for your GameServer Pods, since there already are a lot of existing solutions in the Kubernetes ecosystem. One easy way to accomplish this is to use [Fluent Bit](https://fluentbit.io/) to capture logs and send them to [Azure Blob Storage](https://docs.microsoft.com/azure/storage/blobs/storage-blobs-overview) or on a Storage provide of your choice. Fluent Bit supports multiple output providers, you can can see them [here](https://docs.fluentbit.io/manual/pipeline/outputs).

You can use the following steps to setup Fluent Bit to capture logs from your GameServer Pods and send them to Azure Storage:

- Set up an Azure Storage Account. Check [here](https://docs.microsoft.com/azure/storage/common/storage-account-create?tabs=azure-portal) on how to do it using the Azure Portal.
- As soon as you create the namespace and roles/role bindings, you should create the Fluent Bit ConfigMap containing the Fluent Bit configuration file. You can see a sample [here](https://github.com/PlayFab/thundernetes/blob/main/samples/fluentbit/fluent-bit-configmap.yaml). Remember to replace the values with your Azure Storage Account name and key.
- Finally, you should create the Fluent Bit DaemonSet, so a Fluent Bit Pod runs on every Node in your cluster and grabs the logs. You can find a sample [here](https://github.com/PlayFab/thundernetes/blob/main/samples/fluentbit/fluent-bit-ds.yaml). You can also follow their [official installation guide](https://docs.fluentbit.io/manual/installation/kubernetes), but you will have to edit their Helm Chart values to the ones in our [config map](https://github.com/PlayFab/thundernetes/blob/main/samples/fluentbit/fluent-bit-configmap.yaml).

From now on, the logs from all your pods, including your game servers, will start being uploaded to you Azure Storage Account. Try creating a Game Server Build and allocating a Game Server.

---
layout: default
title: Stuck terminating namespace
parent: Troubleshooting
nav_order: 1
---

# Deleting namespace thundernetes-system stuck in terminating state

Thundernetes creates finalizers for the GameServer custom resource. So, if you delete the thundernetes controller and you try to remove the GameServer Pods and/or the namespace they are in, the namespace might be stuck in terminating state. To fix this, you can run the following commands:

```bash
 kubectl get namespace thundernetes-system -o json > tmp.json
```

Open tmp.json file and find this section:

```json
    "spec": {
        "finalizers": [
            "kubernetes"
        ]
    },
    "status": {
        "phase": "Active"
    }
```

Remove the finalizer section:

```json
 "spec": {

   },
   "status": {
     "phase": "Terminating"
   }
```

Upload the json file:

```bash
kubectl proxy # this command is blocking, so you can either run it on background or use a different shell for the next commands
curl -k -H "Content-Type: application/json" -X PUT --data-binary @tmp.json http://127.0.0.1:8001/api/v1/namespaces/thundernetes-system/finalize
kubectl get ns # verify that the namespace is gone
```

For more information about deleting namespaces stuck in terminating state check the [link](https://www.ibm.com/docs/en/cloud-private/3.2.0?topic=console-namespace-is-stuck-in-terminating-state).
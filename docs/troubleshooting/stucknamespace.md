---
layout: default
title: GameServer objects are not deleted
parent: Troubleshooting
nav_order: 1
---

# GameServer objects are not deleted

Thundernetes creates [finalizers](https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/) for every GameServer custom resource. If you delete the Thundernetes controller and you try to remove the GameServer Pods and/or the namespace they are in, the namespace might be stuck in terminating state since there is no controller to handle the finalizer notification. To fix this and have the resources cleaned up, you can run the following commands:

{% include code-block-start.md %}
 kubectl get namespace thundernetes-system -o json > tmp.json
{% include code-block-end.md %}

Open tmp.json file and find this section:

{% include code-block-start.md %}
    "spec": {
        "finalizers": [
            "kubernetes"
        ]
    },
    "status": {
        "phase": "Active"
    }
{% include code-block-end.md %}

Remove the finalizer section:

{% include code-block-start.md %}
 "spec": {

   },
   "status": {
     "phase": "Terminating"
   }
{% include code-block-end.md %}

Upload the json file:

{% include code-block-start.md %}
kubectl proxy # this command is blocking, so you can either run it on background or use a different shell for the next commands
curl -k -H "Content-Type: application/json" -X PUT --data-binary @tmp.json http://127.0.0.1:8001/api/v1/namespaces/thundernetes-system/finalize
kubectl get ns # verify that the namespace is gone
{% include code-block-end.md %}

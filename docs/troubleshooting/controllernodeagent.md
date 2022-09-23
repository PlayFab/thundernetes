---
layout: default
title: Controller/NodeAgent logs
parent: Troubleshooting
nav_order: 4
---

# How can I get access to the controller and the NodeAgent logs and status? 

Thundernetes controller Pod and NodeAgent Pods are installed in the `thundernetes-system` namespace by default. 

- You can see the status of the controller Deployment with `kubectl get deploy -n thundernetes-system thundernetes-controller-manager`. This will give you a result like: 
{% include code-block-start.md %}
NAME                              READY   UP-TO-DATE   AVAILABLE   AGE
thundernetes-controller-manager   1/1     1            1           2h
{% include code-block-end.md %}
- To get the controller Pod logs, you need to find the Pod name. Run `kubectl get pods -n thundernetes-system` and look for the name of the controller Pod. It should be something like `thundernetes-controller-manager-774c99cd4-6tq8w`. Then you can do `kubectl logs -n thundernetes-system thundernetes-controller-manager-774c99cd4-6tq8w` to see the controller logs.
- To see the NodeAgent status you can run `kubectl get ds -n thundernetes-system`, since NodeAgent is installed as a Kubernetes DaemonSet. You will see an output similar to this:
{% include code-block-start.md %}
NAME                     DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
thundernetes-nodeagent   3         3         3       3            3           &lt;none&gt;          2h
{% include code-block-end.md %}
- To see the logs for a NodeAgent Pod, you need to find the Pod name. Run `kubectl get pods -n thundernetes-system` and look for the name of the NodeAgent Pod. It should be something like `thundernetes-nodeagent-4fdb9`. Then you can do `kubectl logs -n thundernetes-system thundernetes-nodeagent-4fdb9` to see the NodeAgent logs.
- If you want to see the logs for a NodeAgent in a particular Node (e.g. if you need to debug communication between the NodeAgent and a GameServer Pod), you need to first find out the Node name.
  - Run `kubectl get pods -owide` so you can see the Node name that the Pod you want to debug is running on.
  - Run `kubectl get pods -n thundernetes-system -owide` so you can see the Node name along with all the NodeAgent Pods.
  - As soon as you find out the NodeAgent Pod you want to get its logs, you can run `kubectl logs -n thundernetes-system thundernetes-nodeagent-XXXXX` to get this NodeAgent Pod logs.
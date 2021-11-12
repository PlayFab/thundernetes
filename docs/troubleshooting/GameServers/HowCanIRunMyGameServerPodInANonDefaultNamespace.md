## How can I run my game server pods in a non-default namespace?

By default, thundernetes monitors the ```default``` namespace. If you want to run your game servers in a different namespace, you should first install the necessary ServiceAccount/RoleBinding RBAC roles on this namespace. This is because the sidecar running on the GameServer Pod needs access to talk to the Kubernetes API server. For information on Kubernetes RBAC, check here.

You can save the following configuration on a yaml file and then run ```kubectl apply -f /path/to/file.yaml``` to create the namespace and RBAC objects.

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mynamespace
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gameserver-editor
  namespace: mynamespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: gameserver-editor-rolebinding
  namespace: mynamespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: gameserver-editor-role
subjects:
- kind: ServiceAccount
  name: gameserver-editor
  namespace: mynamespace  
```
---
layout: default
title: Uninstalling
parent: How to's
nav_order: 15
---

## Uninstalling Thundernetes

You should first remove all your GameServerBuilds. Since each GameServer has a finalizer, removing the controller before removing GameServer instances will make the GameServer instances get stuck if you try to delete them.

{% include code-block-start.md %}
kubectl delete gsb --all -A # this will delete all GameServerBuilds from all namespaces, which in turn will delete all GameServers
kubectl get gs -A # verify that there are no GameServers in all namespaces
kubectl delete ns thundernetes-system # delete the namespace with all thundernetes resources
# delete RBAC resources. You might need to add namespaces for the service account and the role binding
kubectl delete clusterrole thundernetes-proxy-role thundernetes-metrics-reader thundernetes-manager-role thundernetes-gameserver-editor-role
kubectl delete serviceaccount thundernetes-gameserver-editor
kubectl delete clusterrolebinding thundernetes-manager-rolebinding thundernetes-proxy-rolebinding
kubectl delete rolebinding thundernetes-gameserver-editor-rolebinding
{% include code-block-end.md %}

If you don't need `cert-manager` any more, you can [remove](https://cert-manager.io/docs/installation/kubectl/#uninstalling) it as well.
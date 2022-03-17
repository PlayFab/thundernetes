---
layout: default
title: Configure Port range
parent: How to's
nav_order: 7
---

# Configure Thundernetes port range

By default, Thundernetes will allocate ports in the range 10000-12000 to your GameServers. These ports are allocated to the entire set of VMs in the cluster and are open for each and every VM. If you need more or just a different port range, you can configure it via changing the `MIN_PORT` and the `MAX_PORT` environment variables in the controller deployment YAML file. However, do not modify the port range when there game servers running on the cluster, since this will probably corrupt the port registry, especially if the new and the old range are different.
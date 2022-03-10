---
layout: default
title: get public IP address
parent: How to's
nav_order: 1
---

# How can I find the Public IP address from inside a GameServer?

External code can easily get the Public IP for each game server by querying the Kubernetes API, e.g. you can easily do `kubectl get gs` and you will get IP:port for all your game servers. However, what if you want to find the Public IP from inside a GameServer process?

You can easily get the Public IP address by using one of the following web sites from your game server:

```
curl http://canhazip.com
curl http://whatismyip.akamai.com/
curl https://4.ifcfg.me/
curl http://checkip.amazonaws.com
curl -s http://whatismijnip.nl | awk '{print $5}'
curl -s icanhazip.com
curl ident.me
curl ipecho.net/plain
curl wgetip.com
curl ip.tyk.nu
curl bot.whatismyipaddress.com
wget -q -O - checkip.dyndns.org | sed -e 's/[^[:digit:]\|.]//g'
```

The above methods work since the Node hosting your Pod has a Public IP.
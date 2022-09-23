---
layout: default
title: Discover IP address
parent: How to's
nav_order: 14
---

# How can I find the Public IP address from inside a GameServer?

External code (e.g. your matchmaker or lobby service) can easily get the Public IP for each game server by querying the Kubernetes API, e.g. you can easily do `kubectl get gs` and you will get IP:port for all your game servers. However, what if you want to find the Public IP from the code in your GameServer process?

You can easily get the Public IP address by calling one of the following web sites from inside your game server:

{% include code-block-start.md %}
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
{% include code-block-end.md %}

The above methods work since the Node hosting your Pod has a Public IP, which is returned by all of the above web services.
## How can I find the Public IP address from inside a GameServer?

The GSDK call to get the Public IP is not supported at this time, it returns "N/A". However, you can easily get the Public IP address by using one of the following web sites from your game server:

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
### Testing with LocalMultiplayerAgent

You can use [LocalMultiplayerAgent](runlocalmultiplayeragent.html) to test your GSDK integration of your game server before uploading to Thundernetes.

### Other GSDK methods and callbacks

There are some other GSDK methods you can use from your game server process:

- `GetGameServerConnectionInfo`: Returns the connection information for the GameServer. Usually, it should be the port that you have already defined in your Pod specification. It is **required** use this method if you want to use the [hostNetwork](../howtos/hostnetworking.html) option.
- `GetInitialPlayers`: Returns the IDs of the players that are expected to connect to the GameServer when it starts. It is set during the call to [the allocation service API](../quickstart/allocation-scaling.html).
- `UpdateConnectedPlayers`: It updates the currently connected players to the GameServer. On the backend, Thundernetes updates the `GameServerDetail` Custom Resource with the new number and IDs of connected players.
- `GetConfigSettings`: Returns the current configuration settings for the GameServer. You can retrieve the [associated GameServerBuild metadata](../gameserverbuild.html) with this method.
- `GetLogsDirectory`: Returns the path to the directory for the GameServer logs. It is recommended to just send the logs to standard output/standard error streams, where you can use [a Kubernetes-native logging solution](../howtos/gameserverlogs.html) to grab them.
- `LogMessage`: Writes an entry to the log file. As mentioned, it is recommended to send your logs to standard output/standard error streams
- `GetSharedContentDirectory`: Not used in Thundernetes
- `RegisterMaintenanceCallback` (name might be slightly different depending on your environment): Used to determine the `Health` status of the GameServer
- `RegisterMaintenanceCallback` (name might be slightly different depending on your environment): Not used in Thundernetes
- `RegisterShutdownCallback` (name might be slightly different depending on your environment): Not used in Thundernetes
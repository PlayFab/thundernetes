# Game Server lifecycle

This document contains information about GSDK and how it affects the game server lifecycle on thundernetes.

## Game Server SDK (GSDK)

Game server lifecycle is dependent on the [GSDK](https://github.com/PlayFab/gsdk) calls as well as the state of the game server process, running in the thundernetes Pod. For more information on integrating your game server with GSDK, check the docs [here](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/integrating-game-servers-with-gsdk).

Your game server can integrate GSDK and run great on thundernetes just by calling the **ReadyForPlayers** method. However, we recommend calling **Start** as well when your game server process starts so thundernetes is aware that the game server is initializing.

## Verifying GSDK integration with your game server

MPS has a developer tool to test GSDK integration for game servers. Tool is called **LocalMultiplayerAgent**, you can download it [here](https://github.com/PlayFab/MpsAgent) and you can check the docs [here](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/locally-debugging-game-servers-and-integration-with-playfab#using-localmultiplayeragent-with-linux-containers) for more information.

## GSDK samples

To check GSDK samples demonstrating integration with popular game engines, you can check the MpsSamples repository [here](https://github.com/PlayFab/MpsSamples).

## States of the game server

You can view the states of your GameServers by typing `kubectl get gs`. The states are:

- Empty: when the GameServer is starting, the status is empty
- **Initializing**: GameServer transitions to this state when it calls the **Start()** GSDK method. In this state, the game server is starting to load the necessary assets.
- **StandingBy**: GameServer transitions to this state when it calls the **ReadyForPlayers()** GSDK method. This state implies that the GameServer has loaded all the necessary assets and its ready for allocation.
- **Active**: GameServer transitions to this state when it is [allocated](quickstart.md#allocate-a-game-server). This state implies that players can connect to the game server.

GameServer process can terminate either gracefully or via a crash. This can happen at any state. Thundernetes will remove the Pod running this GameServer and will create a new one in its place. User is responsible for collecting logs while the Pod is running. Check [here](FAQ.md#grab-gameserver-logs) on how to do this.

> GameServer can never transition to StandingBy state from Active state. The only way to get a new game server in StandingBy state is if the GameServer process exits.
> If the game server process crashes for more than `crashesToMarkUnhealthy` times (specified in the GameServerBuild spec, default value 5), then no more operations will be performed on the GameServerBuild. 
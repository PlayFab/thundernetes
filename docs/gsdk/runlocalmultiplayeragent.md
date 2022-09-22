---
layout: default
title: Using LocalMultiplayerAgent
parent: GSDK integration
nav_order: 8
---

# Using LocalMultiplayerAgent

LocalMultiplayerAgent is an open source utility from the Azure PlayFab MPS team that can be used to test game server integration with GSDK and potentially local debugging of a game server. Since MPS and Thundernetes game server container images are identical, the same tool can be used as you develop your game server for Thundernetes. LocalMultiplayerAgent works on Windows operating system.

## Setting up LocalMultiplayerAgent

To run LocalMultiplayerAgent to test your game servers for Thundernetes, you'll need to perform the following steps:

- Download latest version of LocalMultiplayerAgent from the [Releases](https://github.com/PlayFab/MpsAgent/releases) page on GitHub
- [Install Docker Desktop on Windows](https://docs.docker.com/docker-for-windows/install/)
- Make sure it's running on the appropriate container for the operating system your server needs (either Linux containers or Windows containers). Check the [Docker Desktop documentation on how to switch between the two operating systems](https://docs.microsoft.com/en-us/virtualization/windowscontainers/quick-start/quick-start-windows-10-linux#run-your-first-linux-container)
- Your game server image can be can be locally built or published on a container registry.
- You should run `SetupLinuxContainersOnWindows.ps1` Powershell file which will create a Docker network called "PlayFab". This is necessary so that your containers running on a separate network namespace can communicate with LocalMultiplayerAgent, running on the host's network namespace.
- You should properly configure your *LocalMultiplayerSettings.json* file. Below you can see a sample, included in `MultiplayerSettingsLinuxContainersOnWindowsSample.json`:

{% include code-block-start.md %}
{
    "RunContainer": true,
    "OutputFolder": "C:\\output\\UnityServerLinux",
    "NumHeartBeatsForActivateResponse": 10,
    "NumHeartBeatsForTerminateResponse": 60,
    "TitleId": "",
    "BuildId": "00000000-0000-0000-0000-000000000000",
    "Region": "WestUs",
    "AgentListeningPort": 56001,
    "ContainerStartParameters": {
        "ImageDetails": {
            "Registry": "mydockerregistry.io",
            "ImageName": "mygame",
            "ImageTag": "0.1",
            "Username": "",
            "Password": ""
        }
    },
    "PortMappingsList": [
        [
            {
                "NodePort": 56100,
                "GamePort": {
                    "Name": "game_port",
                    "Number": 7777,
                    "Protocol": "TCP"
                }
            }
        ]
    ],
    "SessionConfig": {
        "SessionId": "ba67d671-512a-4e7d-a38c-2329ce181946",
        "SessionCookie": null,
        "InitialPlayers": [ "Player1", "Player2" ]
    }
}
{% include code-block-end.md %}

> _**NOTE**_: Some notes:
> 1. You must set `RunContainer` to true.
> 2. Modify `imageDetails` with your game server Docker image details. Image may be built locally (using [docker build](https://docs.docker.com/engine/reference/commandline/build/) command) or be hosted in a remote container registry. If you host it on a remote container registry, you must provide the username and password for the registry.
> 3. `StartGameCommand` and `AssetDetails` are optional. You don't normally use them when you use a Docker container since all game assets + start game server command can be packaged in the corresponding [Dockerfile](https://docs.docker.com/engine/reference/builder/).
> 4. Last, but definitely not least, pay attention to the casing on your `OutputFolder` variable, since Linux containers are case sensitive. If casing is wrong, you might see a Docker exception similar to *error while creating mount source path '/host_mnt/c/output/UnityServerLinux/PlayFabVmAgentOutput/2020-01-30T12-47-09/GameLogs/a94cfbb5-95a4-480f-a4af-749c2d9cf04b': mkdir /host_mnt/c/output: file exists*

## Verifying GSDK integration

After you perform all the previous steps, you can then run the LocalMultiPlayerAgent with the command `LocalMultiplayerAgent.exe -lcow` (lcow stands for *Linux Containers On Windows*).

If the GSDK is integrated correctly, **LocalMultiplayerAgent** prints the following outputs:  
 - `CurrentGameState - Initializing` (this is optional and may not show up if your game server directly calls `GSDK::ReadyForPlayers` and does not call `GSDK::Start`)
 - `CurrentGameState - StandingBy`
 - `CurrentGameState - Active`
 - `CurrentGameState - Terminating`

At this point, LocalMultiplayerAgent will try to execute the GSDK shutdown callback which **is not** required for Thundernetes. You can either implement it (by exiting the game server process manually) or simply cancel LocalMultiplayerAgent execution and then manually delete the game server container (by executing `docker rm -f <containerName>`).

### Testing connection to your game

When your game server executable is running and **LocalMultiplayerAgent** prints `CurrentGameState - Active`, you can connect to your game server using IP address **127.0.0.1** and port `NodePort` on which your game server is listening.

After `NumHeartBeatsForActivateResponse` heartbeats, **LocalMultiplayerAgent** requests the game server to move from standby to active.

> _**NOTE**_: You can also check the (very similar) MPS instructions [here](https://docs.microsoft.com/en-us/gaming/playfab/features/multiplayer/servers/locally-debugging-game-servers-and-integration-with-playfab#using-localmultiplayeragent-with-linux-containers).
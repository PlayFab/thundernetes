# OpenArena GSDK sample for Azure PlayFab Multiplayer Servers (MPS)

[![OpenArena](https://vignette.wikia.nocookie.net/openarena/images/9/9e/OpenArena_Collage.jpg/revision/latest?cb=20080625093517)](https://openarena.fandom.com/wiki/Main_Page)

This sample demonstrates a way to wrap any existing game with the PlayFab Multiplayer Game Server SDK. For this to work, game server should output relevant information about its lifecycle into its output and error streams.

In this sample, we are using the Linux build of an open source game [OpenArena](https://openarena.fandom.com/wiki/Main_Page) and a simple .NET Core app that starts the game and processes its output. When the game starts, the .NET Core app calls the proper GSDK methods. Since PlayFab Multiplayer Servers needs Docker Containers for Linux Builds, we're using one of the available OpenArena Docker images on Docker Hub as a base image, available [here](https://hub.docker.com/r/fgracia/openarena).

Specifically, OpenArena game server outputs messages (like "Opening IP socket", "ClientBegin", "ClientDisconnect") that allows the .NET Core app/wrapper to call the specific GSDK methods. For example, on "Opening IP socket" the app will call `GameserverSDK.Start()`.

## Usage

1. Make sure you have an account on [playfab.com](https://www.playfab.com) and have enabled Multiplayer Servers
2. Git clone this repo. You should have installed Docker
3. Create a new Linux Build and get the Azure Container Registry information, it will be appear in the Multiplayer page like `docker login --username customervz4l34rmt7rnk --password XXXXXXX customervz4l34rmt7rnk.azurecr.io`. You can also use [the GetContainerRegistryCredentials API call](https://docs.microsoft.com/en-gb/rest/api/playfab/multiplayer/multiplayerserver/getcontainerregistrycredentials?view=playfab-rest) to get the ACR credentials
4. Replace the *TAG* and the *ACR* variables with your values
```bash
TAG="0.4"
ACR="customervz4l34rmt7rnk.azurecr.io"
docker login --username XXXXXX --password XXXXXXX ${ACR}
docker build -t ${ACR}/openarena:${TAG} .
docker push ${ACR}/openarena:${TAG}
```
You can run the above script on [Windows Subsystem for Linux](https://docs.microsoft.com/en-us/windows/wsl/wsl2-index).

5. Create a new MPS Build either via playfab.com or via [the CreateBuildWithCustomerContainer API call](https://docs.microsoft.com/en-gb/rest/api/playfab/multiplayer/multiplayerserver/createbuildwithcustomcontainer?view=playfab-rest). On this Build, select Linux VMs, the image:tag container image you uploaded and a single port for the game, 27960/UDP. 
6. Wait for the Build to be deployed
7. To allocate a server and get IP/port, you can use the [MpsAllocator sample](../MpsAllocatorSample/README.md) or use the [RequestMultiplayerServer](https://docs.microsoft.com/en-gb/rest/api/playfab/multiplayer/multiplayerserver/requestmultiplayerserver?view=playfab-rest) API call. For more information you can check the [documentation](https://docs.microsoft.com/en-us/gaming/playfab/features/multiplayer/servers)
8. Download [OpenArena client](https://openarena.fandom.com/wiki/Main_Page), open the game executable for your platform and connect to your server. Enjoy!

### Is this the recommended way I should use GSDK?

Definitely not. Standard output/error stream processing is hacky at its best. For best results, you should use the [proper GSDK for your language of choice](https://github.com/PlayFab/gsdk).

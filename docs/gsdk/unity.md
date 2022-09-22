---
layout: default
title: Unity
parent: GSDK integration
nav_order: 2
---

# Integrating GSDK with Unity

You can find the Unity GSDK [here](https://github.com/PlayFab/gsdk/tree/main/UnityGsdk). This folder contains the Game Server SDK for the [Unity game engine](https://unity.com/). The GSDK files are in the `Assets/PlayFabSdk/MultiplayerAgent` folder. The entire GSDK is usable by class `PlayFabMultiplayerAgentAPI`. 

### What do I need to do in order to use the Unity GSDK in a new project? 

You can drag the `PlayFabSDK` folder to your Unity project or you can use the provided `unitypackage` file. After that, you need to enable the scripting directive `ENABLE_PLAYFABSERVER_API` on your Build settings, like in [this screenshot](https://user-images.githubusercontent.com/8256138/81462605-a6d7ac80-9168-11ea-9748-110ed01095c2.png). This is useful if your game server and client share the same project files.

### Usage

The minimum work you need to is call the `PlayFabMultiplayerAgentAPI.Start()` when your server is initializing and then call the `PlayFabMultiplayerAgentAPI.ReadyForPlayers();` when your server is ready for players to connect.

{% include code-block-start.md %}
StartCoroutine(ReadyForPlayers());
...
}

IEnumerator ReadyForPlayers()
{
    yield return new WaitForSeconds(.5f);
    PlayFabMultiplayerAgentAPI.ReadyForPlayers();
}
{% include code-block-end.md %}

To create your container image to use on Kubernetes, you should build a "Linux Dedicated Server" and then use a Dockerfile similar to the following:

{% include code-block-start.md %}
FROM ubuntu:18.04
WORKDIR /game
ADD . .
CMD ["/game/UnityServer.x86_64", "-nographics", "-batchmode", "-logfile"]
{% include code-block-end.md %}

### Samples

For a more robust sample integrating Unity with the popular [Mirror](https://mirror-networking.com/) networking library, check the `MpsSamples` repository [here](https://github.com/PlayFab/MpsSamples/tree/main/UnityMirror).

{% include gsdkfooter.md %}
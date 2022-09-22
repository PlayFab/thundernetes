---
layout: default
title: Unreal
parent: GSDK integration
nav_order: 3
---

# Integrating GSDK with Unreal

You can find the Unreal GSDK [here](https://github.com/PlayFab/gsdk/tree/main/UnrealPlugin). This folder contains the Game Server SDK for the [Unreal game engine](https://unrealengine.com/).  

### What do I need to do in order to use the Unreal GSDK in a new project? 

The Unreal server project needs to be a network-enabled multiplayer Unreal project with a dedicated-server mode. If you don't have a project that meets these prerequisites, follow our [Unreal prerequisite](https://github.com/PlayFab/gsdk/blob/main/UnrealPlugin/ThirdPersonMPSetup.md) set up guide to set one up. Once you have a network-enabled, multiplayer game, with a dedicated server, return to this step and continue.

When ready, open your network-enabled multiplayer Unreal server project, and continue to the [next step](https://github.com/PlayFab/gsdk/tree/main/UnrealPlugin), specifically to the part about [building for the cloud](https://github.com/PlayFab/gsdk/blob/main/UnrealPlugin/ThirdPersonMPCloudDeploy.md). 

At a minimum, you would need to call the `SetDefaultServerHostPort()` to get the port your server should listen to and `ReadyForPlayers()` method to transition your server to the StandingBy state.

{% include code-block-start.md %}
#if UE_SERVER
    UGSDKUtils::SetDefaultServerHostPort();
#endif

// ...

void U[YourGameInstanceClassName]::OnStart()
{
    UE_LOG(LogPlayFabGSDKGameInstance, Warning, TEXT("Reached onStart!"));
    UGSDKUtils::ReadyForPlayers();
}
{% include code-block-end.md %}

To create your container image to use on Kubernetes, you should build a "Linux Dedicated Server" and then use a Dockerfile similar to the following:

{% include code-block-start.md %}
FROM ubuntu:18.04

# Unreal refuses to run as root user, so we must create a user to run as
# Docker uses root by default
RUN useradd --system ue
USER ue

EXPOSE 7777/udp

WORKDIR /server

COPY --chown=ue:ue . /server
USER root
CMD su ue -c ./ShooterServer.sh
{% include code-block-end.md %}

### Samples

Check [this guide](https://github.com/PlayFab/MpsSamples/tree/main/UnrealThirdPersonMP) on how to integrate the [Unreal Third Person template](https://docs.unrealengine.com/4.27/en-US/Resources/Templates/ThirdPerson/) with the Unreal GSDK.

{% include gsdkfooter.md %}
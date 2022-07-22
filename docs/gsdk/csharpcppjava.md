---
layout: default
title: C++/C#/Java/Go
parent: GSDK integration
nav_order: 4
---

# Integrating GSDK with C++/C#/Java/Go

You can find the GSDK libraries for each language here:
- [C# GSDK](https://github.com/PlayFab/gsdk/tree/main/csharp). You can find it on [C# GSDK Nuget package page](https://www.nuget.org/packages/com.playfab.csharpgsdk)
- [C++ GSDK](https://github.com/PlayFab/gsdk/tree/main/cpp). You can find it on [C++ GSDK Nuget package page](https://www.nuget.org/packages/com.playfab.cppgsdk.v140)
- [Java GSDK](https://github.com/PlayFab/gsdk/tree/main/java). You can find it on [Java GSDK Maven package page](https://mvnrepository.com/artifact/com.playfab/gameserverSDK)
- [Go GSDK](https://github.com/PlayFab/gsdk/tree/main/experimental/go)

## Usage

In all these programming languages, you need to include the GSDK libraries in your project and call the `Start()` and `ReadyForPlayers()` methods. `Start` will signal to Thundernetes that the game server is initializing whereas `ReadyForPlayers` will signal that the game server is ready for players to connect.

{% include_relative gsdkfooter.md %}
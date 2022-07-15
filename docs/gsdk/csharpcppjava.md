---
layout: default
title: C++/C#/Java
parent: GSDK integration
nav_order: 4
---

# Integrating GSDK with C++/C#/Java

You can find the GSDK libraries for each language here:
- [C# GSDK](https://github.com/PlayFab/gsdk/tree/main/csharp)
- [C++ GSDK](https://github.com/PlayFab/gsdk/tree/main/cpp)
- [Java GSDK](https://github.com/PlayFab/gsdk/tree/main/java)

## Usage

In all these programming languages, you need to include the GSDK libraries in your project and call the `Start()` and `ReadyForPlayers()` methods. `Start` will signal to Thundernetes that the game server is initializing whereas `ReadyForPlayers` will signal that the game server is ready for players to connect.

#### Testing with LocalMultiplayerAgent

You can use [LocalMultiplayerAgent](runlocalmultiplayeragent.md) to test your GSDK integration of your game server before uploading to Thundernetes.
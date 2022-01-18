# Running Unity servers on thundernetes

Unity game servers are supported on thundernetes. The only thing you need to is add the Unity GSDK to your project. You can download the Unity GSDK from [here](https://github.com/PlayFab/gsdk/tree/master/UnityGsdk), check the documentation [here](https://docs.microsoft.com/gaming/playfab/features/multiplayer/servers/mps-unity). You can find a Unity sample integrated with the [Mirror Network SDK](https://mirror-networking.com/) in the [PlayFab GSDK Samples](https://github.com/PlayFab/MpsSamples/tree/master/UnityMirror).

As soon as you built the Unity server container image, you can create a GameServerBuild on thundernetes using a file like [this](./sample.yaml).
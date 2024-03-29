---
layout: default
title: Thundernetes UI
nav_order: 10
has_children: true
---


# Thundernetes UI

Currently we are developing [Thundernetes UI](https://github.com/PlayFab/thundernetes-ui), a front end application to manage game servers running in one or more Thundernetes clusters. To be able to connect to them, be sure to deploy the [GameServer API](https://github.com/PlayFab/thundernetes/tree/main/cmd/gameserverapi) on each cluster.

## Check all of your clusters in the same place

You can see a summary of what's going on in all your clusters.

[![Home page](../assets/images/thundernetes_ui_home.png "Home page")](../assets/images/thundernetes_ui_home.png)

## Manage the builds on each cluster

Check the builds you have on each cluster and their current status. You can also create a new one, either from scratch, or by cloning one from any cluster!

[![Cluster view](../assets/images/thundernetes_ui_cluster.png "Cluster view")](../assets/images/thundernetes_ui_cluster.png)

[![Create build view](../assets/images/thundernetes_ui_cluster_create_build.png "Create build view")](../assets/images/thundernetes_ui_cluster_create_build.png)

## Manage each build

You can check each build separately and see it's status and specs, you can modify the standingBy and max values, allocate individual game servers for testing, and see a list of all the game servers running.

[![Build view - Specs](../assets/images/thundernetes_ui_build_specs.png "Build view - Specs")](../assets/images/thundernetes_ui_build_specs.png)

[![Build view - GameServers](../assets/images/thundernetes_ui_build_gameservers.png "Build view - GameServers")](../assets/images/thundernetes_ui_build_gameservers.png)

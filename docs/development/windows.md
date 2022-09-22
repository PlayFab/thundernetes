---
layout: default
title: Developing with Windows containers
parent: Development
nav_order: 2
---

# Developing with Windows containers

Thundernetes now supports game servers running on Windows containers, you can read more about it [here](../howtos/windowscontainers.md). If you want to try this you need both a Windows machine, with Windows 2019 or higher, to build the necessary Windows Docker images, and a Kubernetes cluster with Windows nodes. You can't build or run Windows containers on a Linux machine. If you have all of this, you can follow these next steps:

- Login to your container registry (`docker login <registry>`) on your Linux machine or WSL, where `<registry>` is the registry where you want to upload your images.
- Run `NS=<registry> make clean build push create-install-files-dev`.
- Login to your container registry (`docker login`) on your Windows machine.
- Run `.\windows\Build-DockerWin.ps1 -registry <registry>`.
- Now you can install Thundernetes on your cluster using any of the files on the `installfilesdev` directory.
- If you want to deploy a Windows game server on Thundernetes make sure to include the following on the game server build YAML file, we use this to know how to deploy the game servers correctly:

{% include code-block-start.md %}
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample
spec:
  ...
  template:
      spec:
        nodeSelector:
          kubernetes.io/os: windows
    ...
{% include code-block-end.md %}

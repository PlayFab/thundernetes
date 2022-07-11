---
layout: default
title: Quickstart
nav_order: 2
has_children: true
---

# Quickstart

This guide will help you get started with Thundernetes. That is, you will see how to create a Kubernetes cluster, install Thundernetes on it, make sure it's up and running and, last but not least, run a sample game server! 

First of all, you need a Kubernetes cluster. Let's see how we can create one.

If you want to install Thundernetes on Azure Kubernetes Service (AKS) you can follow [this guide](quickstart/installing-aks.md). If you want to test locally with kind, you can read [here](quickstart/installing-kind.md).

> _**NOTE**_: We've tested Thundernetes on the latest versions of Azure Kubernetes Service (AKS) and [kind](https://kind.sigs.k8s.io/) but it can be installed on any Kubernetes cluster supporting Public IP per Node. This is something you definitely want if you want to expose your game servers outside the cluster. 

> _**NOTE**_: If you are using Windows, we highly recommend using [Windows Subsystem for Linux](https://docs.microsoft.com/windows/wsl/install) to run the CLI commands listed in the various sections of the quickstart.
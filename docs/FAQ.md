---
layout: default
title: Frequently Asked Questions
nav_order: 15
---

# Frequently Asked Questions

## Can I run a Unity or Unreal game server on Thundernetes?

You can run any game server that supports the [PlayFab GameServer SDK](https://github.com/PlayFab/gsdk). Check a Unity sample [here](https://github.com/PlayFab/thundernetes/tree/main/samples/unity/README.md) and documentation for the Unreal plugin [here](https://github.com/PlayFab/gsdk/tree/main/UnrealPlugin). On [this](https://github.com/PlayFab/MpsSamples) repository you can find samples for all programming languages GSDK supports, like C#/Java/C++/Go/Unity/Unreal.

## How can I add custom Annotations and/or Labels to my GameServer Pods?

The GameServerBuild template allows you to set custom Annotations and/or Labels along with the Pod specification. This is possible since GameServerBuild includes the entire PodTemplateSpec. Labels and Annotations are copied to the GameServers and the Pods in the GameServerBuild. Check the following YAML for an example:

{% include code-block-start.md %}
apiVersion: mps.playfab.com/v1alpha1
kind: GameServerBuild
metadata:
  name: gameserverbuild-sample-netcore
spec:
  titleID: "1E03" # required
  buildID: "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6" # must be a GUID
  standingBy: 2 # required
  max: 4 # required
  portsToExpose:
    - 80 # one of the ports mentioned below
  template:
    metadata:
        annotations:
          annotation1: annotationvalue1
        labels:
          label1: labelvalue1
    spec:
      containers:
        - image: ghcr.io/playfab/thundernetes-netcore:0.2.0
          name: thundernetes-sample-netcore
          ports:
          - containerPort: 80 # your game server port
            protocol: TCP # your game server port protocol
            name: gameport # required field    
{% include code-block-end.md %}

## Can I run my game server pods in a non-default namespace?

Yes. You don't need to do anything special to run your game server Pods in a namespace different than "default".

## How do I make GameServer Pods start before DaemonSet Pods?

When a new Node is added to the Kubernetes cluster, a NodeAgent Pod (part of DaemonSet) will be created there. However, if there were pending GameServer Pods before the Node's addition to the cluster, they will also be scheduled on the new Node. Consequently, GameServer Pods might start at the same time as the NodeAgent Pod. GameServer Pods are heartbeating to the NodeAgent process so there is a chance that some heartbeats will be lost and, potentially, a state change from "" to "Initializing" will not be tracked (however, the GameServer Pod should have no trouble transitioning to StandingBy when the NodeAgent Pod is up and can process heartbeats).

There will be no impact from these lost heartbeats. However, you can ask Kubernetes to schedule NodeAgent Pods before the GameServer Pods by assigning Pod Priorities to the NodeAgent Pods. You can read more about Pod priority [here](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption) and specifically about the impact of Pod priority on scheduling order [here](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#effect-of-pod-priority-on-scheduling-order).

## How can I add resource constraints to my GameServer Pods?

Kubernetes supports resource constraints when you are creating a Pod ([reference](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)). Essentially, you can specify the amount of CPU and memory that your Pod can request when it starts (requests) as well as the maximum amount of CPU and memory that your Pod can use (limits). To configure resource constraints for your Pod, you can modify the GameServerBuild definition. Since the entire PodSpec is defined in the GameServerBuild definition, you can add these resource constraints to the PodSpec. Take a look at the following example to see how you can modify your GameServerBuild YAML for this purpose:

{% include code-block-start.md %}
template:
    spec:
      containers:
        - image: your-image:tag
          name: thundernetes-sample
          ports:
          - containerPort: 80 # your game server port
            protocol: TCP # your game server port protocol
            name: gameport # required field
          resources:
            requests:
              cpu: 100m
              memory: 500Mi
            limits:
              cpu: 100m
              memory: 500Mi
{% include code-block-end.md %}

For a full sample, you can check [here](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore/sample-requestslimits.yaml).

## How can I run custom code on my container to respond to a terminated event from Kubernetes?

You can use the [PreStop container hook](https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks). Effectively, this hook is executed when the container is terminated. You can use this hook to perform custom cleanup operations. Read [here](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination) for more information about Pod termination.

## Where does the name 'Thundernetes' come from?

It's a combination of the words 'thunderhead' and 'kubernetes'. 'Thunderhead' is the internal code name for the Azure PlayFab Multiplayer Servers service. Credits to [Andreas Pohl](https://github.com/Annonator) for the naming idea!

## Is there any validation when creating or editing GameServerBuilds?

Currently we are using validation webhooks when for GameServerBuild and GameServers. For GameServerBuild we make the following validations:

- Checks that there is not another GameServerBuild with different name but with the same buildID.
- Prevents changing the buildID.
- Validates that the port configuration is correct.
- Validates that standingBy < Max

For GameServers we make these ones:

- Validates that every GameServer has a GameServerBuild as an owner.
- Validates that the port configuration is correct.

## Can I use `kubectl scale` to scale GameServers?

Currently we enabled the scale command for changing the number of standingBy GameServers, but it has the side effect of bypassing the validation webhooks. This means you can have a standingBy value thats higher than the max of GameServers allowed. In practice, the controller won't create more GameServers than the max, but it's an inconsistency. We recommend changing the standingBy value using `kubectl edit` instead.


---
layout: default
title: Development
nav_order: 14
has_children: true
---

# Development

This section contains development notes and tips for working with Thundernetes source code.

## Kubebuilder notes

Project was bootstrapped using [kubebuilder](https://github.com/kubernetes-sigs/kubebuilder) using the following commands:

{% include code-block-start.md %}
kubebuilder init --domain playfab.com --repo github.com/playfab/thundernetes/pkg/operator
kubebuilder create api --group mps --version v1alpha1 --kind GameServer
kubebuilder create api --group mps --version v1alpha1 --plural gameserverbuilds --kind GameServerBuild 
kubebuilder create api --group mps --version v1alpha1 --plural gameserverdetails --kind GameServerDetail 
{% include code-block-end.md %}

---
layout: default
title: Release new Thundernetes version
parent: Development
nav_order: 4
---

# Release new Thundernetes version

This will require 2 PRs.

- Make sure you update `.versions` file on the root of this repository with the new version
- Run `make clean` to ensure any cached artifacts of old builds are deleted.
- Push and merge. The GitHub Action step that checks if "create-install-files" are modified will fail, that is expected on a new release.
- Manually run the GitHub Actions workflows to create new [linux images](https://github.com/PlayFab/thundernetes/actions/workflows/publish.yml) and [windows images](https://github.com/PlayFab/thundernetes/actions/workflows/publish-windows.yml)
- Git pull the latest changes from the main branch
- Run `make create-install-files` to generate the operator install files
- Replace the image tag on the [samples folder](https://github.com/PlayFab/thundernetes/tree/main/samples) and especially the [netcore-sample YAML files](https://github.com/PlayFab/thundernetes/tree/main/samples/netcore) since these are referenced by our quickstart doc.
- Push and merge. Good luck!
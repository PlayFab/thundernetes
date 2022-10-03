---
layout: default
title: Contributing
nav_order: 13
---

# Contributing Guide

Welcome! We are glad that you want to contribute to our project! 💖

As you get started, you are in the best position to give us feedback on areas of
our project that we need help with including:

* Problems found during setting up a new developer environment
* Gaps in our [Quickstart Guide](quickstart.md) or documentation
* Bugs in our automation scripts

If anything doesn't make sense, or doesn't work when you run it, please open a
bug report and let us know!

* [Contributing Guide](#contributing-guide)
  * [Ways to Contribute](#ways-to-contribute)
  * [Find an Issue](#find-an-issue)
  * [Ask for Help](#ask-for-help)
  * [Development Environment Setup](#development-environment-setup)
    * [Code](#code)
    * [Tools](#tools)
  * [Pull Request Workflow](#pull-request-workflow)
  * [Pull Request Checklist](#pull-request-checklist)
  * [Sign Your Commits](#sign-your-commits)
    * [CLA](#cla)

## Ways to Contribute

We welcome many different types of contributions including:

* New features
* Builds, CI/CD
* Bug fixes
* Documentation
* Issue Triage
* PR Comments & Feedback
* Answering questions on Github Discussions
* Communications / Social Media / Blog Posts

## Find an Issue

We have good first issues for new contributors and help wanted issues suitable
for any contributor.
[good first issue](https://github.com/PlayFab/thundernetes/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22) has extra information to help you make your first contribution.
[help wanted](https://github.com/PlayFab/thundernetes/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22) are issues suitable for someone who isn't a core maintainer and is
good to move onto after your first pull request.

Sometimes there won’t be any issues with these labels.
That’s ok! There is likely still something for you to work on.
If you want to contribute but you don’t know where to start or can't find a suitable issue,
you can reach out to the team with
[Github Discussions](https://github.com/PlayFab/thundernetes/discussions)
and we can help find something for you to work on.

Once you see an issue that you'd like to work on, please post a comment saying
that you want to work on it.
Something like "I want to work on this" is fine.

## Ask for Help

The best way to reach us with a question when contributing is to ask is via the
[Github Discussions](https://github.com/PlayFab/thundernetes/discussions)
section of the Thundernetes repository.

## Development Environment Setup

### Code

To get the latest code for Thundernetes run the following in a terminal:

{% include code-block-start.md %}
git clone https://github.com/PlayFab/thundernetes.git
{% include code-block-end.md %}

### Tools

See the [Prerequisites](prerequisites.md) page for tools & systems to run Thundernetes.
The follow list is only the tools related to building & developing the code for Thundernetes.

* [VS Code](https://code.visualstudio.com)
* [Golang](https://go.dev)
* [.NET](https://dotnet.microsoft.com/download/dotnet)
* [Docker](https://docs.docker.com/get-docker/)

## External Contributors

If you want to contribute but are not part of the GitHub Playfab org, we recommend working on a fork of the project.

## Pull Request Workflow

Thundernetes follows a standard GitHub pull request workflow.
If you're unfamiliar with this workflow, read the very helpful
[Understanding the GitHub flow](https://docs.github.com/en/get-started/quickstart/github-flow) guide from GitHub.

Anyone is welcome to create draft PRs at any stage of development/readiness.
This can be very helpful, when asking for assistance, to refer to an active PR.
However, draft PRs will not be actively reviewed by maintainers.

To signal that a PR is ready for open review by maintainers & other community members,
please convert the draft PR into a "ready for review" PR.

## Pull Request Checklist

When you submit your pull request, or you push new commits to it, our automated
systems will run some checks on your new code. We require that your pull request
passes these checks, but we also have more criteria than just that before we can
accept and merge it. We recommend that you check the following things locally
before you submit your code:

* Any new or changed code include test additions or changes, as appropriate.
* Documentation is updated as required by the scope of the change.
* CHANGELOG.md is updated to reflect this change.
* Our CLA has been signed

## Sign Your Commits

### CLA

We require that contributors have signed our Contributor License Agreement (CLA).

Our current CLA can be found & signed at
[https://cla.opensource.microsoft.com/microsoft/.github](https://cla.opensource.microsoft.com/microsoft/.github).

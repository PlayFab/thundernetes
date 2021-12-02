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
    * [DCO](#dco)
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
[good first issue](TODO) has extra information to help you make your first contribution.
[help wanted](TODO) are issues suitable for someone who isn't a core maintainer and is
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

To get the latest code for thundernetes run the following in a terminal:

```bash
git clone https://github.com/PlayFab/thundernetes.git
```

### Tools

See the [Prerequisites](prerequisites.md) page for tools & systems to run thundernetes.
The follow list is only the tools related to building & developing the code for thundernetes.

* [VS Code](https://code.visualstudio.com)
* [Golang](https://go.dev)
* [.NET](https://dotnet.microsoft.com/download/dotnet)
* [Docker](https://docs.docker.com/get-docker/)

## Pull Request Workflow

Thundernetes follows a standard GitHub pull request workflow.
If you're unfamiliar with this workflow, read the very helpful
[Understanding the GitHub flow](https://guides.github.com/introduction/flow/) guide from GitHub.

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
* Commits are signed with DCO
* Our CLA has been signed

## Sign Your Commits

### DCO

Licensing is important to open source projects. It provides some assurances that
the software will continue to be available based under the terms that the
author(s) desired. We require that contributors sign off on commits submitted to
our project's repositories. The [Developer Certificate of Origin
(DCO)](https://developercertificate.org/) is a way to certify that you wrote and
have the right to contribute the code you are submitting to the project.

You sign-off by adding the following to your commit messages. Your sign-off must
match the git user and email associated with the commit.

```text
    This is my commit message

    Signed-off-by: Your Name <your.name@example.com>
```

Git has a `-s` command line option to do this automatically:

```text
    git commit -s -m 'This is my commit message'
```

If you forgot to do this and have not yet pushed your changes to the remote
repository, you can amend your commit with the sign-off by running

```text
    git commit --amend -s 
```

### CLA

We require that contributors have signed our Contributor License Agreement (CLA).

Our current CLA can be found & signed at
[https://cla.opensource.microsoft.com/microsoft/.github](https://cla.opensource.microsoft.com/microsoft/.github).

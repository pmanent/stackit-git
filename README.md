
<div align="center">
    <img src="./assets/logo.svg" alt="" width="192" align="center" />
    <h1 align="center">Welcome to STACKIT Git</h1>
</div>

## Table of Contents
- [About the Project](#about-the-project)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
- [Project structure guide](#project-structure-guide)
  - [CMD](#cmd)
  - [Custom folder](#custom)
  - [Models](#models)
  - [Services](#services)
  - [Templates](#templates)
  - [Test](#tests)
  - [Deployment Pipelines](#toolsazure)
- [App.ini file](#appini)
- [Old Readme](#old-readmemd)
  - [What does forgejo offer?](#what-does-forgejo-offer)
  - [Learn more](#learn-more)
  - [Get involved](#get-involved)


# About The Project

This project provides the core for the STACKIT Git Project. It contains the base code to have a STACKIT Git instance.

STACKIT Git is a git solution, forked from Forgejo, that allows developers store their code, project management, actions,... among other
features.

## Build With

- Go 1.25
- Docker for building the docker images, version `27.1.1`

# Getting Started
These are the first steps to be able to have your repository in your local machine

## Prerequisites

Download Microsoft [Visual Studio Code](https://code.visualstudio.com/)

Follow instructions at https://code.visualstudio.com/docs/languages/go  to setup Go extension for it.

Install Go:
```shell
brew install go
echo "Check the installed version is 1.25"
go version
```
Install poetry
```shell
brew install poetry
poetry version
```

Install npm
```shell
brew install npm
```

Install nvm
```shell
brew install nvm
nvm install v22.15.1
```

## Installation

1. Clone the repository:
```shell
git clone git@github.com:pmanent/stackit-git.git
```

2. Download Dependencies
```shell
go mod tidy
```

And then type the following commands (be aware that it can take a long time to finish. If you wish, you can run the first command followed by the parameter " --debug" to see the progress)

```shell
make deps
make build
```

## Project structure guide

### /cmd
Contains all the logic and commands that will be received via CMD.

### /custom
Custom folder contains the custom implementation for the STACKIT Git project, all the files that are not there
will use the default files provided from the fork.

For UI changes, please use the custom folder to avoid merge conflicts in the future.

### /models
Package that includes all the struct and logic related to each struct.

### /services
Logic for each of the services.

### /templates
Templates that are used for the UI with dynamic variables that can be replaced.

### /tests
Contain all the tests for the application


## App.ini
This concept has a different section due to its relevance.
The code uses this app.ini file to initialize multiple varibles and configurations depending on this file.
that provides a general overview of the possible variables and configuration.

We have created an extra Section called stackitgitsettings, to add custom variables to this app.ini.

The injection of these variables is done via Secrets following the format: `FORGEJO__{{section}}__{{VARIABLE_NAME_IN_CAPITAL}}`


## What does STACKIT Git offer?


If you like any of the following, STACKIT Git is literally meant for you:

- Lightweight: STACKIT Git can easily be hosted on nearly **every machine**.
  Running on a Raspberry? Small cloud instance? No problem!
- Project management: Besides Git hosting, STACKIT Git offers issues,
  pull requests, wikis, kanban boards and much more to **coordinate with your team**.
- Publishing: Have something to share? Use **releases** to host your software for download,
  or use the **package registry** to publish it for docker, npm and many other package managers.
- Customizable: Want to change your look? Change some settings?
  There are many **config switches** to make STACKIT Git work exactly like you want.
- Powerful: Organizations & team permissions, CI integration, Code Search, LDAP, OAuth and much more.
  If you have **advanced needs**, STACKIT Git has you covered.
- Privacy: From update checker to default settings: STACKIT Git is built to be **privacy first** for you and your crew.
- Federation: (WIP) We are actively working to connect software forges with each other through **ActivityPub**,
  and create a collaborative network of personal instances.

## License

STACKIT Git is distributed under the terms of the [GPL version 3.0](LICENSE) or any later version.


## Get involved

If you are interested in making STACKIT Git better, either by reporting a bug or by changing the governance, please [take a look at the contribution guide](CONTRIBUTING.md).

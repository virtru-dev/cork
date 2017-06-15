# Quick Start Guide

This quick start guide will walk you through creating your first cork server.
You don't need to know the nitty gritty details yet, but the cork server is
what executes your build plan. It's a docker container that contains all the
scripts, dependencies, and different stages of your cork build. For this
quick start example we will create a basic cork server to handle a very
simple node.js project. This guide assumes you have some knowledge of bash
and docker, but you shouldn't need all that much knowledge.

## Step 1: Create a new directory for your cork server

Feel free to name this directory anything you'd like, but for this tutorial
we're going to call this `practice-project`.

```
$ mkdir practice-project
```

## Step 2: Create the scaffolding for the project

There is work to making this easier but at the moment do all of the following
to get the proper scaffolding:

    $ cd practice-project # cd into your project
    $ mkdir -p cork/commands
    $ mkdir -p cork/hooks

## Step 3: Create the Dockerfile for the cork server

In the root of your `practice-project` make the following `Dockerfile`:

```
FROM virtru/base-cork-server:xenial

RUN curl -sL https://deb.nodesource.com/setup_6.x | bash - && \
    apt-get update && \
    apt-get install -y nodejs && \
    npm install -g npm
```

You will notice we are basing this docker container off of the
`virtru/base-cork-server:xenial` container. This container will automatically
copy the `cork` directory into the correct place for the cork server. 

## Step 4: Create a build script

We're going to create a simple build script for now just to get things going.
The build script will do basically nothing except say "Hello, world". Don't
worry this is just to get a feel for things :)

So create a file `cork/commands/build` with this data:

```
#!/bin/bash

echo "Hello, world"
```

Then set this file to be executable with:

```
$ chmod +x cork/commands/build
```

## Step 5: Create a definition.yml to define your build workflow

Cork servers can host a set of workflows that we call stages. Each stage is
composed of steps that can do things like execute commands or export
variables from a build, or call other stages. For now, we only need one
stage, the `build` stage. 

Do that by creating a file `cork/definition.yml` with this data:

```
# The version is required
version: 1

# Define the stages
stages:
  # The build stage runs all the steps for the build
  build:
    - name: build
      type: command
      args:
        command: build

  # The default stage is run if you simply run `cork run`
  default:
    - type: stage
      args:
        stage: build
```

## Step 6: Build your first cork server

To build the cork server. You simply run:

```
$ cork run
```

And if you've followed along your cork server will build properly.

## Step 7: Run your hello world cork server

To run your hello world cork server:

```
$ cork ext-run practice-project build
```

If all was good you should see output look something like this:

```
Cork - The most reliable build tool ever conceived! (... probably)

Cork Is Running
-------------------
Project: practice-project
Project Type: virtru/cork-server-project:latest
Executing Stage: default
-------------------

>>> Executing command step "build"
/cork/commands/build: line 5: [: missing `]'
Sending build context to Docker daemon 6.144 kB
Step 1/2 : FROM virtru/base-cork-server:xenial
# Executing 1 build trigger...
Step 1/1 : COPY cork /cork
 ---> Using cache
 ---> 444444444444
Successfully built 444444444444

Hello, world!

Cork is done!
Find your outputs: /Users/raven/development/virtru/practice-project/outputs.json
```

Obviously, you want cork to do much more than this, so let's move on to the
next step.

## Step 8: Making the build stage useful

Ok let's do something interesting with the build stage and use it to install
a node project and package it into a docker container.

Change your `cork/commands/build` script like so:

```
#!/bin/bash

npm install
```

## Step 9: Making an example node project to test with

Before we can try to use this cork server let's setup a simple node project
to use this.

First setup the example project scaffolding (from the root of the `practice-project`):

```
$ mkdir -p example
```

Next create an `example/package.json` file:

```
{
  "name": "example",
  "version": "1.0.0",
  "description": "An example project",
  "main": "index.js",
  "author": "",
  "license": "MIT",
  "dependencies": {
    "express": "^4.15.3"
  }
}
```

Finally, create a script for the project at `example/index.js`:

```
const express = require('express')
const app = express()

app.get('/', function (req, res) {
  res.send('Hello World!')
})

app.listen(process.env.PORT, function () {
  console.log('Example app listening on port ' + process.env.PORT)
})
```

## Step 10: 
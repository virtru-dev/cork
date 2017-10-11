# Quick Start Guide

This guide will walk you through creating your first cork server and cover
most of the key features of cork. You don't need to know the nitty gritty
details yet, but the cork server is what executes your build plan. It's a
docker container that contains all the scripts, dependencies, and different
stages of your cork build. For this quick start example we will create a
basic cork server to handle a very simple node.js project. This guide assumes
you have some knowledge of bash and docker, but you shouldn't need all that
much knowledge.

## Before we begin

Before using cork on your local system it's best to setup your ssh agent with
your private key. On some kind of automated server you may have a different
way to handle this, but on a local system the ssh-agent is by far the most
useful.

## Walkthrough

### Step 1: Create a new directory for your cork server

Feel free to name this directory anything you'd like, but for this tutorial
we're going to call this `practice-project`.

```
$ mkdir practice-project
```

### Step 2: Create the scaffolding for the project

There is work to making this easier but at the moment do all of the following
to get the proper scaffolding:

```
$ cd practice-project # cd into your project
$ mkdir -p cork/commands
$ mkdir -p cork/hooks
$ echo "#!/bin/bash" > cork-server-setup
$ chmod +x cork-server-setup
```

The last piece of scaffolding is a `cork.yml` file that will tell cork that
you'd like to use the `virtru/cork-server-project` as the cork server for
this project. A `cork.yml` file is required for `cork run` to execute. Think
of it like a `package.json` for node packages or `setup.py` file for python
packages.

So, create the `cork.yml` file to contain:

```yaml
type: virtru/cork-server-project:latest
```

Once you're done with this step your directory structure should look like
this:

```
practice-project/
 |-cork.yml
 |-cork-server-setup
 |-cork/
 |  |-commands/
 |  |  |-build
 |  |-hooks/
```

### Step 3: Create the Dockerfile for the cork server

In the root of your `practice-project` make the following `Dockerfile`:

```
FROM virtru/common-cork-server:xenial
```

You will notice we are basing this docker container off of the
`virtru/common-cork-server:xenial` container. This container will automatically
copy the `cork` directory into the correct place for the cork server. 

### Step 4: Create a build script

We're going to create a simple build script for now just to get things going.
The build script will do basically nothing except say "Hello, world". Don't
worry this is just to get a feel for things :)

So create a file `cork/commands/build` with this data:

```bash
#!/bin/bash

echo "Hello, world"
```

Then set this file to be executable with:

```
$ chmod +x cork/commands/build
```

### Step 5: Create a definition.yml to define your build workflow

Cork servers can host a set of workflows that we call stages. Each stage is
composed of steps that can do things like execute commands or export
variables from a build, or call other stages. For now, we only need one
stage, the `build` stage. 

Do that by creating a file `cork/definition.yml` with this data:

```yaml
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

### Step 6: Build your first cork server

To build the cork server. You simply run:

```
$ cork run
```

And if you've followed along your cork server will build properly and be
available to you via docker. To check, run:

```
$ docker images | grep practice-project
```

It might not be obvious, but once a cork server is built, it is simply a
docker image. In the next step we will use that docker container.

### Step 7: Run your hello world cork server

To run your hello world cork server, we will use the `ext-run` subcommand of
cork. This subcommand allows you to run any cork server that is available to
you on docker without having to have a `cork.yml` in the local directory.

The syntax for this call is:

```
cork ext-run [docker-image] [stage]
```

For our purposes, we'd like to use our project which is available at the
local docker image `practice-project` and our stage `build`. To do this, run
the following command:

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
Find your outputs: /Users/foo/development/virtru/practice-project/outputs.json
```

Obviously, you want cork to do much more than give you a silly "Hello,
World!", so let's move on to the next step.

### Step 8: Making the build stage useful

Ok, let's do something interesting with the build stage and use it to install
a node project and package it into a docker container. We're going to put a
test node project into the `example/` sub-directory of the `practice-project`
root directory.

Change your `cork/commands/build` script like so:

```bash
#!/bin/bash

docker build -t ${CORK_PROJECT_NAME}-container .

echo "Created a container ${CORK_PROJECT_NAME}-container
```

Notice the use of the environment variable `CORK_PROJECT_NAME`. That variable
is injected by cork and represents either the directory name within which
cork is running, or the project name that can be optionally defined in the
`cork.yml` file. To see the list of injected variables see here.

### Step 9: Start your example project

To start the example project. Create the `example` sub-directory

```
$ mkdir -p example
```

Add a dockerfile like so at `example/Dockerfile`:

```
FROM ubuntu:xenial

ENV PORT=9000
EXPOSE 9000

RUN curl -sL https://deb.nodesource.com/setup_6.x | bash - && \
    apt-get update && \
    apt-get install -y nodejs && \
    npm install -g npm && \
    mkdir /app

WORKDIR /app

COPY package.json /app/package.json
RUN npm install
COPY index.js /app/index.js
```

### Step 9: Create a simple express app to test with

Create an `example/package.json` file:

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

Next, create a script for the project at `example/index.js`:

```javascript
const express = require('express')
const app = express()

app.get('/', function (req, res) {
  res.send('Hello World!')
})

app.listen(process.env.PORT, function () {
  console.log('Example app listening on port ' + process.env.PORT)
})
```

Finally, to tie it all together we use a `cork.yml` file that will be used to
tell cork that you'd like to use your `practice-project` cork server with
this example project. 

This file, at `example/cork.yml`, should look like this:

```yaml
type: practice-project:latest
```

### Step 10: Rebuild your cork server

From within the root directory run the following:

```
$ cork run
```

### Step 11: Test your cork server on your express app

Now let's build the docker container for our express app. So, from within the
root directory run the following commands:

```
$ cd example
$ cork run
```

Once you run this you should have a container `example-container` in docker.

### Step 12: Run the express app

To run the docker container simply do this: 

```
$ docker run -it --rm -p 9000:9000 example-container
```

You can now visit [http://127.0.0.1:9000](http://127.0.0.1:9000) to see the
result.

### Finished! So now what?

Congrats! You've finished the quick start guide. You've now seen the most
basic uses of cork. To get the full benefits of using cork, continue on to
the [advanced section](advanced.md).
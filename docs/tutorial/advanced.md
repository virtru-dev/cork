# Advanced Guide

This guide builds upon the [Quick Start](quick-start.md) to show more
advanced options. If you haven't gone through the 
[Quick Start](quick-start.md), do so before beginning this guide.

## Using built-in cork variables

cork provides some built-in variables that can be used when you're building 

## Accepting input parameters for the build

Often, you need to accept input for a build. These input values can be used
to do anything you'd like, but are probably useful to guide parts of your
build workflow.

### Step 1: Setup the definition.yml to accept user input

In the practice project from the [Quick Start](quick-start.md) we will setup
it's `definition.yml` to accept user input. Update the `cork/definition.yml`
of the `practice-project` to look like this:

```yaml
# The version is required
version: 1

# Parameters need to be defined up front. 
params:
  image_tag:
    type: string
    description: The tag for the docker image that this build will create

# Define the stages
stages:
  # The build stage runs all the steps for the build
  build:
    - name: build
      type: command
      args:
        command: build
        params:
          image_tag: '{{ param "image_tag" }}'

  # The default stage is run if you simply run `cork run`
  default:
    - type: stage
      args:
        stage: build
```

### Step 2: Change the build script to use `image_tag`

To update the build script to use the newly defined input parameter, you'll
need to update the build script to use the variable. Cork loads parameters
into the running docker container's environment variables using the pattern
`CORK_PARAM_PARAM_NAME`. So `image_tag` would be available to the `build`
script as `CORK_PARAM_IMAGE_TAG`

To use this in the `cork/commands/build` script update that file like so:

```bash
#!/bin/bash

docker build -t ${CORK_PROJECT_NAME}-container:${CORK_PARAM_IMAGE_TAG} .

echo "Created a container ${CORK_PROJECT_NAME}-container:${CORK_PARAM_IMAGE_TAG}
```

### Step 3: Rebuild the cork server

As always when you change the cork server's internal scripts you need to rebuild the cork server. So from the root of the `practice-project` directory run:

```
$ cork run
```

### Step 4: Rebuild the example project

Now you need to rebuild the example project.

```
$ cd example # cd into the example project
$ cork run
```

If you run this command, you will be asked to input the `image_tag` that you
wish to use. Input any value you'd like and the example project should build.
Do a `docker images | grep example-container` to verify your updates.

### Step 5: Rebuild the example project, but specify the input from the command line

Now, being asked for the input every time won't always be the desired request.

To accept input from the command line you can run `cork run` or `cork
ext-run` with the following syntax:

```
$ cork run --param param_name=value
```

The `--param` flag can be repeated any number of times.
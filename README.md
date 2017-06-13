# cork - The most reliable build tool ever conceived (_... probably_)

cork is a build tool for projects that executes build steps in docker
containers to achieve highly reliable and highly portable build workflows. cork
is the tool you always wanted but didn't know how to describe.

Have you ever had to spin up a build agent for your CI tool and needed to build
node-v4 projects and node-v6? If so, you've run in to issues with tools
conflicting. It's possible to make it work, but why not save yourself the
headache of having to deal with build agents. Imagine if you could make it so
the exact same tool used in CI to build/test was the tool you used on your
local development machine.

Dream no more. Cork is here.

Cork is powered by docker and runs anywhere docker runs. If it works on Linux,
it will work on Windows and OS X. 

## Quick start

For the quick start, we will be creating a brand new node-v4 project.

### Initialize a project

```
$ cork init virtru/node-v4-project:latest
```

### Build the project

```
$ cork run
```

## Usage

### Initialize a project

```
$ cork init [project-type]
```

### Running all the default stage defined in the project type server

```
$ cork run
```

### Run just the test stage

```
$ cork run test
```

## Open Source By Virtru

This tool was created by Virtru for greater the software development community.
[We're Hiring!](https://www.virtru.com/careers/)
